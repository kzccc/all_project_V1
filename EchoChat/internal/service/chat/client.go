package chat

// 本文件实现 client 相关的实时消息链路与在线连接管理逻辑。

import (
	"context"
	"echo_chat_server/internal/config"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/observability"
	"echo_chat_server/internal/pressure"
	myKafka "echo_chat_server/internal/service/kafka"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/util/random"
	"echo_chat_server/pkg/zlog"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type MessageBack struct {
	// Message 是准备写回前端的序列化消息体。
	Message []byte
	// Uuid 是对应消息记录的主键，用于发送成功后回写状态。
	Uuid string
	// EnqueuedAt 记录消息进入 websocket SendBack 队列的时刻，用于压测阶段统计 receiver queue wait。
	EnqueuedAt time.Time
	// statusUpdated 保证同一条消息在多接收者场景下最多只做一次状态更新。
	statusUpdated atomic.Bool
}

type Client struct {
	// Conn 是该用户持有的 WebSocket 连接。
	Conn *websocket.Conn
	// Uuid 是当前在线用户 ID。
	Uuid string
	// ConnectionID 是当前 websocket 连接标识，用于日志追踪。
	ConnectionID string
	// RequestID 是 websocket 握手请求对应的 request_id。
	RequestID string
	// RoutePath 是当前 websocket 握手路径。
	RoutePath string
	// Benchmark 标记当前连接是否来自压测专用路由。
	Benchmark bool
	// SendTo 用于在 channel 模式下暂存待转发给服务端的消息。
	SendTo chan []byte // 给server端
	// SendBack 用于承接服务端准备推送给前端的消息。
	SendBack chan *MessageBack // 给前端
	// CriticalBack 用于承接关键失败提示，避免和普通消息共用同一条可挤爆的回写通道。
	CriticalBack chan *MessageBack // 给前端

	// connectedAt 是本次 websocket 连接建立时间。
	connectedAt time.Time
	// lastActiveAt 记录最近一次有效客户端活动时间。
	lastActiveAt atomic.Int64
	// closeReason 记录连接关闭原因，便于排障和观测。
	closeReason atomic.Pointer[string]
	// writeMutex 保护 websocket conn 的并发写入。
	writeMutex sync.Mutex

	closeOnce   sync.Once
	observeOnce sync.Once
	cleanupOnce sync.Once
	closed      chan struct{}
	isClosed    atomic.Bool
}

// upgrader 是一个全局的 WebSocket 升级器实例，用于将 HTTP 请求协议升级为 WebSocket 协议。
// 配置说明：
// - ReadBufferSize 和 WriteBufferSize 分别设置读写缓冲区大小为 2048 字节，用于优化网络 I/O 性能；
// - CheckOrigin 回调函数用于校验请求的 Origin 头，默认返回 true 表示允许所有来源的跨域连接（适用于开发或信任前端的场景）。
var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
	// 检查连接的Origin头
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ctx 复用到 Kafka 写消息场景中。
var ctx = context.Background()

// messageMode 决定当前客户端把消息写入本地 channel 还是 Kafka。
var messageMode = config.GetConfig().KafkaConfig.MessageMode

func newConnectionID() string {
	return "WS" + random.GetNowAndLenRandomString(6)
}

func (c *Client) markActive(now time.Time) {
	if c == nil {
		return
	}
	c.lastActiveAt.Store(now.UnixNano())
	if config.GetConfig().KafkaConfig.MessageMode == "kafka" {
		KafkaChatServer.renewClientRoute(c, now)
	}
}

func (c *Client) lastActiveTime() time.Time {
	nano := c.lastActiveAt.Load()
	if nano == 0 {
		return c.connectedAt
	}
	return time.Unix(0, nano)
}

func (c *Client) recordCloseReason(reason string) {
	if c == nil || reason == "" {
		return
	}
	reasonCopy := reason
	c.closeReason.CompareAndSwap(nil, &reasonCopy)
}

func (c *Client) startHeartbeat() {
	conf := config.GetConfig().KafkaConfig
	interval := conf.ResolveWSHeartbeatInterval()
	timeout := conf.ResolveWSSilenceTimeout()
	if interval <= 0 || timeout <= 0 {
		return
	}
	c.markActive(time.Now())
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-c.closed:
				return
			case <-ticker.C:
				lastActive := c.lastActiveTime()
				if !lastActive.IsZero() && time.Since(lastActive) > timeout {
					c.recordCloseReason("heartbeat_timeout")
					c.observeClose("heartbeat", "heartbeat_timeout")
					c.cleanupDisconnected()
					return
				}
				if err := c.writeControl(websocket.PingMessage, nil, time.Now().Add(2*time.Second)); err != nil {
					c.recordCloseReason("heartbeat_ping_failed")
					c.observeClose("heartbeat", classifyWSError(err))
					c.cleanupDisconnected()
					return
				}
			}
		}
	}()
}

func (c *Client) writeControl(messageType int, data []byte, deadline time.Time) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return c.Conn.WriteControl(messageType, data, deadline)
}

func (c *Client) writeText(message []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return c.Conn.WriteMessage(websocket.TextMessage, message)
}

func (c *Client) wsFields(fields ...zap.Field) []zap.Field {
	base := []zap.Field{
		zap.String("module", "chat.ws"),
		zap.String("connection_id", c.ConnectionID),
		zap.String("request_id", c.RequestID),
		zap.String("client_id", c.Uuid),
	}
	return append(base, fields...)
}

func (c *Client) notifyClient(message string) {
	select {
	case <-c.closed:
		return
	case c.SendBack <- &MessageBack{Message: []byte(message)}:
	default:
		zlog.Error("ws.client.sendback_full", c.wsFields(zap.String("event", "ws.client.sendback_full"))...)
	}
}

func (c *Client) notifyClientCritical(message string) {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-c.closed:
		return
	case c.CriticalBack <- &MessageBack{Message: []byte(message)}:
	case <-timer.C:
		zlog.Error("ws.client.critical_timeout", c.wsFields(zap.String("event", "ws.client.critical_timeout"))...)
	}
}

func (c *Client) cleanupDisconnected() {
	c.cleanupOnce.Do(func() {
		kafkaConfig := config.GetConfig().KafkaConfig
		if kafkaConfig.MessageMode == "channel" {
			ChatServer.RemoveClient(c)
		} else {
			KafkaChatServer.RemoveClient(c)
		}
		c.Close()
	})
}

// enqueueLocal 尝试将一条待发送的消息放入客户端本地的 SendTo 通道中（用于本地 channel 模式下的消息暂存）。
// 该方法是非阻塞的：如果通道已满，则立即返回 false，避免阻塞调用方。
// 主要用于在全局消息通道（ChatServer.Transmit）繁忙时，将消息暂存在当前客户端的本地队列中，
// 后续由 forwardPendingMessages 协程按序转发，保证消息顺序不乱。
//
// 参数：
//   - message: 要入队的原始消息字节（通常是 JSON 格式的聊天请求）
//
// 返回值：
//   - ok: true 表示成功入队；false 表示通道已满或通道已被关闭（无法入队）
func (c *Client) enqueueLocal(message []byte) (ok bool) {
	if c.isClosed.Load() {
		return false
	}
	// 使用 defer + recover 捕获可能的 panic（例如向已关闭的 channel 发送数据），
	// 避免整个 goroutine 崩溃，并记录日志提示通道已关闭。
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("ws.client.sendto_closed", c.wsFields(zap.String("event", "ws.client.sendto_closed"))...)
			ok = false // 显式标记为失败
		}
	}()

	// 非阻塞地尝试将消息写入 c.SendTo 通道：
	// - 如果通道有空闲缓冲区，则写入成功，返回 true；
	// - 如果通道已满（缓冲区满），则立即走 default 分支，返回 false。
	select {
	case <-c.closed:
		return false
	case c.SendTo <- message:
		return true
	default:
		return false
	}
}

func (c *Client) enqueueBack(messageBack *MessageBack) bool {
	if c.isClosed.Load() {
		observability.ObserveWSSendBackEnqueue(c.RoutePath, len(c.SendBack), 0, "client_closed")
		return false
	}
	queueLength := len(c.SendBack)
	start := time.Now()
	if messageBack != nil && messageBack.Uuid != "" {
		pressure.ObserveBenchmarkEventAt(messageBack.Uuid, "receiver_queue_enqueue", start, map[string]interface{}{
			"route_path":  c.RoutePath,
			"queue_depth": queueLength,
		})
		messageBack.EnqueuedAt = start
	}
	select {
	case <-c.closed:
		observability.ObserveWSSendBackEnqueue(c.RoutePath, queueLength, time.Since(start), "client_closed")
		return false
	case c.SendBack <- messageBack:
		observability.ObserveWSSendBackEnqueue(c.RoutePath, queueLength, time.Since(start), "success")
		return true
	}
}

func (c *Client) writeMessageBack(messageBack *MessageBack) bool {
	writeStart := time.Now()
	if messageBack != nil && messageBack.Uuid != "" && !messageBack.EnqueuedAt.IsZero() {
		pressure.ObserveBenchmarkEventAt(messageBack.Uuid, "receiver_queue_dequeue", writeStart, map[string]interface{}{
			"route_path": c.RoutePath,
		})
	}
	err := c.writeText(messageBack.Message)
	writeResult := "success"
	if err != nil {
		writeResult = "failure"
		observability.ObserveWSWrite(c.RoutePath, time.Since(writeStart), writeResult)
		c.observeClose("write", classifyWSError(err))
		zlog.Error("ws.message.write_failed", c.wsFields(zap.String("event", "ws.message.write_failed"), zap.String("error", err.Error()))...)
		return false
	}
	observability.ObserveWSWrite(c.RoutePath, time.Since(writeStart), writeResult)
	if messageBack.Uuid != "" {
		pressure.ObserveBenchmarkEvent(messageBack.Uuid, "ws_write_done", nil)
	}
	// 系统提示消息不对应数据库记录，不需要更新消息状态。
	if messageBack.Uuid == "" {
		return true
	}
	// 同一条消息可能被分发到发送者和多个接收者；这里用 CAS 避免重复更新同一行。
	if !messageBack.statusUpdated.CompareAndSwap(false, true) {
		return true
	}
	if config.GetConfig().KafkaConfig.UseStatusUpdateNoopExperimental() {
		return true
	}
	// 状态更新改为后台批量处理，避免把每条消息的 DB update 串在 websocket 写协程里。
	kafkaStatusUpdater.enqueue(messageBack.Uuid, c.RoutePath)
	return true
}

// forwardPendingMessages 独立转发本地积压队列，避免旧消息必须等下一条新消息到来才会补投。
func (c *Client) forwardPendingMessages() {
	if pressure.ShouldLogHotPath(c.Benchmark) {
		zlog.Info("ws.forward.start", c.wsFields(zap.String("event", "ws.forward.start"))...)
	}
	for {
		select {
		case <-c.closed:
			return
		case message := <-c.SendTo:
			if !ChatServer.SendMessageToTransmit(message) {
				return
			}
		}
	}
}

// 读取websocket消息并发送给send通道
func (c *Client) Read() {
	defer c.cleanupDisconnected()
	c.Conn.SetPongHandler(func(string) error {
		c.markActive(time.Now())
		return nil
	})
	if pressure.ShouldLogHotPath(c.Benchmark) {
		zlog.Info("ws.read.start", c.wsFields(zap.String("event", "ws.read.start"))...)
	}
	for {
		// ReadMessage 阻塞等待客户端下一帧消息，这是 WebSocket 读循环的正常行为。
		_, jsonMessage, err := c.Conn.ReadMessage()
		if err != nil {
			c.observeClose("read", classifyWSError(err))
			if !c.Benchmark {
				zlog.Error("ws.read.failed", c.wsFields(zap.String("event", "ws.read.failed"), zap.String("error", err.Error()))...)
			}
			return
		}

		var message request.ChatMessageRequest
		if err := json.Unmarshal(jsonMessage, &message); err != nil {
			zlog.Error("ws.message.unmarshal_failed", c.wsFields(zap.String("event", "ws.message.unmarshal_failed"), zap.String("error", err.Error()))...)
			c.notifyClient("消息格式错误")
			continue
		}
		if message.SendId == "" || message.ReceiveId == "" {
			zlog.Error("ws.message.invalid_payload", c.wsFields(zap.String("event", "ws.message.invalid_payload"))...)
			c.notifyClient("消息缺少必要字段")
			continue
		}
		if message.SendId != c.Uuid {
			zlog.Error(
				"ws.message.actor_mismatch",
				c.wsFields(
					zap.String("event", "ws.message.actor_mismatch"),
					zap.String("payload_send_id", message.SendId),
				)...,
			)
			c.notifyClientCritical("消息发送身份校验失败")
			continue
		}
		message.MessageId = ensureMessageID(message.MessageId)
		pressure.EnsureBenchmarkMessageMeta(message.MessageId, message.Content)
		pressure.ObserveBenchmarkEvent(message.MessageId, "ws_read_done", map[string]interface{}{
			"route_path":    c.RoutePath,
			"connection_id": c.ConnectionID,
			"session_id":    message.SessionId,
			"send_id":       message.SendId,
			"receive_id":    message.ReceiveId,
		})
		idemSession, existingRecord, err := beginIngressIdempotency(message.SendId, message.MessageId)
		if err != nil {
			zlog.Error("ws.message.idempotency_failed", c.wsFields(
				zap.String("event", "ws.message.idempotency_failed"),
				zap.String("message_id", message.MessageId),
				zap.String("error", err.Error()),
			)...)
			c.notifyClientCritical("消息发送失败，请稍后重试")
			continue
		}
		if !idemSession.Acquired {
			if shouldShortCircuitIngressDuplicate(c, message.MessageId, existingRecord) {
				continue
			}
			c.notifyClientCritical("消息发送失败，请稍后重试")
			continue
		}
		c.markActive(time.Now())
		jsonMessage, err = json.Marshal(message)
		if err != nil {
			zlog.Error("ws.message.marshal_failed", c.wsFields(
				zap.String("event", "ws.message.marshal_failed"),
				zap.String("message_id", message.MessageId),
				zap.String("error", err.Error()),
			)...)
			idemSession.clearPending()
			c.notifyClientCritical("消息发送失败，请稍后重试")
			continue
		}
		zlog.Info(
			"ws.message.in",
			c.wsFields(
				zap.String("event", "ws.message.in"),
				zap.String("message_id", message.MessageId),
				zap.String("session_id", message.SessionId),
				zap.Int8("message_type", message.Type),
				zap.String("send_id", message.SendId),
				zap.String("receive_id", message.ReceiveId),
			)...,
		)

		if messageMode == "channel" {
			// 本地有积压时，后续消息继续排队，避免新消息绕过旧消息插队发送。
			if len(c.SendTo) > 0 {
				if !c.enqueueLocal(jsonMessage) {
					idemSession.clearPending()
					c.notifyClientCritical("由于目前同一时间过多用户发送消息，消息发送失败，请稍后重试")
				}
				continue
			}

			// 全局通道有空间就直接投递；否则退回当前连接的本地缓冲。
			if !ChatServer.SendMessageToTransmit(jsonMessage) {
				if !c.enqueueLocal(jsonMessage) {
					idemSession.clearPending()
					c.notifyClientCritical("由于目前同一时间过多用户发送消息，消息发送失败，请稍后重试")
				}
			}
			continue
		}

		// Kafka 模式下把消息直接写入聊天主题，由后端消费循环统一处理。
		key := config.GetConfig().KafkaConfig.ResolveMessageKey(message.SessionId)
		if err := myKafka.KafkaService.PublishChatMessage(ctx, key, jsonMessage, message.MessageId); err != nil {
			zlog.Error("kafka.produce.chat.failed", c.wsFields(
				zap.String("event", "kafka.produce.chat.failed"),
				zap.String("message_id", message.MessageId),
				zap.String("session_id", message.SessionId),
				zap.String("send_id", message.SendId),
				zap.String("receive_id", message.ReceiveId),
				zap.String("key", key),
				zap.String("error", err.Error()),
			)...)
			idemSession.clearPending()
			c.notifyClientCritical("消息发送失败，请稍后重试")
			continue
		}
		zlog.Info("kafka.produce.chat", c.wsFields(
			zap.String("event", "kafka.produce.chat"),
			zap.String("message_id", message.MessageId),
			zap.String("session_id", message.SessionId),
			zap.String("send_id", message.SendId),
			zap.String("receive_id", message.ReceiveId),
			zap.String("key", key),
		)...)
	}
}

// 从send通道读取消息发送给websocket
func (c *Client) Write() {
	defer c.cleanupDisconnected()
	if pressure.ShouldLogHotPath(c.Benchmark) {
		zlog.Info("ws.write.start", c.wsFields(zap.String("event", "ws.write.start"))...)
	}
	for {
		// 第一段 select 是一个“非阻塞优先检查”：
		// - 如果 criticalCh 此刻已经有关键提示（例如发送失败），立刻取出并写回前端；
		// - 如果 criticalCh 当前没有数据，则走 default，立刻进入下一段 select，
		//   不会因为等待关键通道而卡住普通消息的发送。
		//
		// 这样做的目的，是让关键失败提示在“已经到达”的情况下尽量抢先写回，
		// 避免被大量普通聊天消息长期排在后面。
		select {
		case <-c.closed:
			return
		case messageBack := <-c.CriticalBack:
			if !c.writeMessageBack(messageBack) {
				return
			}
			continue
		default:
		}

		// 第二段 select 是真正的“阻塞等待一条可发送消息”：
		// - 如果 criticalCh 有消息，仍然优先走关键提示分支；
		// - 否则才会从 normalCh 取普通聊天消息；
		//
		// 这里之所以同时监听两个通道，是因为 write 协程是整个连接唯一的写出口：
		// 所有普通消息和关键提示最终都必须从这里串行写入 websocket，
		// 这样可以避免多个 goroutine 并发写同一个连接。
		select {
		case <-c.closed:
			return
		case messageBack := <-c.CriticalBack:
			if !c.writeMessageBack(messageBack) {
				return
			}
		case messageBack := <-c.SendBack:
			if !c.writeMessageBack(messageBack) {
				return
			}
		}
	}
}

// NewClientInit 当接受到前端有登录消息时，会调用该函数
func NewClientInit(c *gin.Context, clientId string) {
	kafkaConfig := config.GetConfig().KafkaConfig
	if kafkaConfig.MessageMode == "channel" && ChatServer.IsShuttingDown() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "server is shutting down"})
		return
	}
	if kafkaConfig.MessageMode == "kafka" && KafkaChatServer.IsShuttingDown() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "server is shutting down"})
		return
	}
	requestID := c.GetString(constants.REQUEST_ID_CONTEXT_KEY)
	routePath := c.Request.URL.Path
	observability.ObserveWSHandshakeAttempt(routePath)
	// 使用 upgrader 将当前 HTTP 请求升级为 WebSocket 连接。
	// - c.Writer 和 c.Request 来自 Gin 的上下文，分别代表原始的 http.ResponseWriter 和 *http.Request；
	// - upgrader.Upgrade 会处理 WebSocket 握手协议（如检查 Upgrade 头、Sec-WebSocket-Key 等），
	//   若成功则返回一个 *websocket.Conn 连接对象，后续可通过该连接进行双向通信；
	// - 第三个参数为可选的响应头（此处为 nil，表示不额外添加响应头）。
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		observability.ObserveWSHandshakeResult(routePath, false)
		zlog.Error(
			"ws.upgrade.failed",
			zap.String("event", "ws.upgrade.failed"),
			zap.String("module", "chat.ws"),
			zap.String("request_id", requestID),
			zap.String("client_id", clientId),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.String("error", err.Error()),
		)
		return
	}
	observability.ObserveWSHandshakeResult(routePath, true)
	connectionID := newConnectionID()
	client := &Client{
		Conn:         conn,
		Uuid:         clientId,
		ConnectionID: connectionID,
		RequestID:    requestID,
		RoutePath:    routePath,
		Benchmark:    pressure.IsBenchmarkPath(routePath),
		SendTo:       make(chan []byte, constants.CHANNEL_SIZE),
		SendBack:     make(chan *MessageBack, constants.CHANNEL_SIZE),
		CriticalBack: make(chan *MessageBack, 1),
		connectedAt:  time.Now(),
		closed:       make(chan struct{}),
	}
	client.markActive(client.connectedAt)
	observability.IncWSOnline(routePath)
	if pressure.ShouldLogHotPath(client.Benchmark) {
		zlog.Info("ws.connection.open", client.wsFields(zap.String("event", "ws.connection.open"))...)
	}
	//? 登录注册交给不同的服务实现，但客户端读写协程逻辑保持一致。
	if kafkaConfig.MessageMode == "channel" {
		if !ChatServer.SendClientToLogin(client) {
			_ = conn.Close()
			return
		}
		go client.forwardPendingMessages()
	} else {
		if !KafkaChatServer.SendClientToLogin(client) {
			_ = conn.Close()
			return
		}
	}
	go client.Read()
	go client.Write()
	client.startHeartbeat()
	if pressure.ShouldLogHotPath(client.Benchmark) {
		zlog.Info("ws.connection.ready", client.wsFields(zap.String("event", "ws.connection.ready"))...)
	}
}

// ClientLogout 当接受到前端有登出消息时，会调用该函数
func ClientLogout(clientId string) (string, int) {
	kafkaConfig := config.GetConfig().KafkaConfig
	var client *Client
	if kafkaConfig.MessageMode == "channel" {
		client = ChatServer.GetClient(clientId)
	} else {
		client = KafkaChatServer.GetClient(clientId)
	}
	if client != nil {
		zlog.Info("ws.connection.logout", client.wsFields(zap.String("event", "ws.connection.logout"))...)
		// 先通知在线服务移除该连接，再主动关闭底层 websocket 和消息通道。
		if kafkaConfig.MessageMode == "channel" {
			ChatServer.SendClientToLogout(client)
		} else {
			KafkaChatServer.SendClientToLogout(client)
		}
		client.Close()
	}
	return "退出成功", 0
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.isClosed.Store(true)
		reason := "local_close"
		if ptr := c.closeReason.Load(); ptr != nil && *ptr != "" {
			reason = *ptr
		}
		c.observeClose("local", reason)
		close(c.closed)
		observability.DecWSOnline(c.RoutePath)
		if err := c.Conn.Close(); err != nil {
			zlog.Error("ws.connection.close_failed", c.wsFields(zap.String("event", "ws.connection.close_failed"), zap.String("error", err.Error()))...)
			return
		}
		if pressure.ShouldLogHotPath(c.Benchmark) {
			zlog.Info("ws.connection.close", c.wsFields(zap.String("event", "ws.connection.close"))...)
		}
	})
}

func (c *Client) observeClose(source string, reason string) {
	c.observeOnce.Do(func() {
		observability.ObserveWSClose(c.RoutePath, source, reason)
	})
}

func classifyWSError(err error) string {
	if err == nil {
		return "unknown"
	}
	if errors.Is(err, io.EOF) {
		return "eof"
	}
	switch {
	case websocket.IsCloseError(err, websocket.CloseNormalClosure):
		return "close_1000"
	case websocket.IsCloseError(err, websocket.CloseGoingAway):
		return "close_1001"
	case websocket.IsCloseError(err, websocket.CloseAbnormalClosure):
		return "close_1006"
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "unexpected eof"):
		return "unexpected_eof"
	case strings.Contains(message, "use of closed network connection"):
		return "closed_network_connection"
	case strings.Contains(message, "broken pipe"):
		return "broken_pipe"
	case strings.Contains(message, "connection reset by peer"):
		return "connection_reset"
	case strings.Contains(message, "i/o timeout"):
		return "io_timeout"
	default:
		return "other_error"
	}
}
