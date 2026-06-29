package chat

// 本文件实现 client 相关的实时消息链路与在线连接管理逻辑。

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/dto/request"
	myKafka "echo_chat_server/internal/service/kafka"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/util/random"
	"echo_chat_server/pkg/zlog"
)

type MessageBack struct {
	// Message 是准备写回前端的序列化消息体。
	Message []byte
	// Uuid 是对应消息记录的主键，用于发送成功后回写状态。
	Uuid string
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
	// SendTo 用于在 channel 模式下暂存待转发给服务端的消息。
	SendTo chan []byte
	// SendBack 用于承接服务端准备推送给前端的消息。
	SendBack chan *MessageBack
	// CriticalBack 用于承接关键失败提示，避免和普通消息共用同一条可挤爆的回写通道。
	CriticalBack chan *MessageBack

	closeOnce   sync.Once
	observeOnce sync.Once
	cleanupOnce sync.Once
	closed      chan struct{}
	isClosed    atomic.Bool
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var ctx = context.Background()

var messageMode = config.GetConfig().KafkaConfig.MessageMode

func newConnectionID() string {
	return "WS" + random.GetNowAndLenRandomString(6)
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
	select {
	case <-c.closed:
		return
	case c.CriticalBack <- &MessageBack{Message: []byte(message)}:
	default:
		zlog.Error("ws.client.critical_full", c.wsFields(zap.String("event", "ws.client.critical_full"))...)
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

func (c *Client) enqueueLocal(message []byte) (ok bool) {
	if c.isClosed.Load() {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("ws.client.sendto_closed", c.wsFields(zap.String("event", "ws.client.sendto_closed"))...)
			ok = false
		}
	}()

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
		return false
	}
	select {
	case <-c.closed:
		return false
	case c.SendBack <- messageBack:
		return true
	}
}

func (c *Client) writeMessageBack(messageBack *MessageBack) bool {
	if err := c.Conn.WriteMessage(websocket.TextMessage, messageBack.Message); err != nil {
		c.observeClose("write", classifyWSError(err))
		zlog.Error("ws.message.write_failed", c.wsFields(zap.String("event", "ws.message.write_failed"), zap.String("error", err.Error()))...)
		return false
	}
	if messageBack.Uuid == "" {
		return true
	}
	if !messageBack.statusUpdated.CompareAndSwap(false, true) {
		return true
	}
	if config.GetConfig().KafkaConfig.UseStatusUpdateNoopExperimental() {
		return true
	}
	kafkaStatusUpdater.enqueue(messageBack.Uuid, c.RoutePath)
	return true
}

func (c *Client) forwardPendingMessages() {
	zlog.Info("ws.forward.start", c.wsFields(zap.String("event", "ws.forward.start"))...)
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

func (c *Client) Read() {
	defer c.cleanupDisconnected()
	zlog.Info("ws.read.start", c.wsFields(zap.String("event", "ws.read.start"))...)
	for {
		_, jsonMessage, err := c.Conn.ReadMessage()
		if err != nil {
			c.observeClose("read", classifyWSError(err))
			zlog.Error("ws.read.failed", c.wsFields(zap.String("event", "ws.read.failed"), zap.String("error", err.Error()))...)
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
		jsonMessage, err = json.Marshal(message)
		if err != nil {
			zlog.Error("ws.message.marshal_failed", c.wsFields(
				zap.String("event", "ws.message.marshal_failed"),
				zap.String("message_id", message.MessageId),
				zap.String("error", err.Error()),
			)...)
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
			if len(c.SendTo) > 0 {
				if !c.enqueueLocal(jsonMessage) {
					c.notifyClientCritical("由于目前同一时间过多用户发送消息，消息发送失败，请稍后重试")
				}
				continue
			}

			if !ChatServer.SendMessageToTransmit(jsonMessage) {
				if !c.enqueueLocal(jsonMessage) {
					c.notifyClientCritical("由于目前同一时间过多用户发送消息，消息发送失败，请稍后重试")
				}
			}
			continue
		}

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

func (c *Client) Write() {
	defer c.cleanupDisconnected()
	zlog.Info("ws.write.start", c.wsFields(zap.String("event", "ws.write.start"))...)
	for {
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
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zlog.Error(
			"ws.upgrade.failed",
			zap.String("event", "ws.upgrade.failed"),
			zap.String("module", "chat.ws"),
			zap.String("request_id", requestID),
			zap.String("client_id", clientId),
			zap.String("path", routePath),
			zap.String("method", c.Request.Method),
			zap.String("error", err.Error()),
		)
		return
	}
	connectionID := newConnectionID()
	client := &Client{
		Conn:         conn,
		Uuid:         clientId,
		ConnectionID: connectionID,
		RequestID:    requestID,
		RoutePath:    routePath,
		SendTo:       make(chan []byte, constants.CHANNEL_SIZE),
		SendBack:     make(chan *MessageBack, constants.CHANNEL_SIZE),
		CriticalBack: make(chan *MessageBack, 1),
		closed:       make(chan struct{}),
	}
	zlog.Info("ws.connection.open", client.wsFields(zap.String("event", "ws.connection.open"))...)
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
	zlog.Info("ws.connection.ready", client.wsFields(zap.String("event", "ws.connection.ready"))...)
}

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
		c.observeClose("local", "local_close")
		close(c.closed)
		if err := c.Conn.Close(); err != nil {
			zlog.Error("ws.connection.close_failed", c.wsFields(zap.String("event", "ws.connection.close_failed"), zap.String("error", err.Error()))...)
			return
		}
		zlog.Info("ws.connection.close", c.wsFields(zap.String("event", "ws.connection.close"))...)
	})
}

func (c *Client) observeClose(source string, reason string) {
	c.observeOnce.Do(func() {
		zlog.Info("ws.connection.closed", c.wsFields(
			zap.String("event", "ws.connection.closed"),
			zap.String("source", source),
			zap.String("reason", reason),
		)...)
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
