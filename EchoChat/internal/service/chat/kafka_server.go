package chat

// 本文件实现 kafka server 相关的实时消息链路与在线连接管理逻辑。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/dto/respond"
	"echo_chat_server/internal/model"
	"echo_chat_server/internal/observability"
	"echo_chat_server/internal/pressure"
	"echo_chat_server/internal/service/kafka"
	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/zlog"
	"github.com/IBM/sarama"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type KafkaServer struct {
	// Clients 保存当前 Kafka 模式下的在线连接。
	Clients map[string]*Client
	// mutex 保护在线连接表。
	mutex *sync.Mutex
	// Login 承接新登录客户端。
	Login chan *Client // 登录通道
	// Logout 承接待下线客户端。
	Logout       chan *Client // 退出登录通道
	shutdownOnce sync.Once

	done            chan struct{}
	stopped         chan struct{}
	readerStopped   chan struct{}
	dispatchStopped chan struct{}
	instanceID      string

	shuttingDown  atomic.Bool
	consumerReady atomic.Bool
}

// KafkaChatServer 是基于 Kafka 消费循环驱动的聊天服务单例。
var KafkaChatServer *KafkaServer

// kafkaQuit 预留给 Kafka 模式优雅退出使用。
var kafkaQuit = make(chan os.Signal, 1)

// init 在包加载时完成当前模块的默认实例或运行期资源初始化。
func init() {
	if KafkaChatServer == nil {
		KafkaChatServer = &KafkaServer{
			Clients: make(map[string]*Client),
			mutex:   &sync.Mutex{},
			Login:   make(chan *Client),
			Logout:  make(chan *Client),

			done:    make(chan struct{}),
			stopped: make(chan struct{}),

			readerStopped:   make(chan struct{}),
			dispatchStopped: make(chan struct{}),
			instanceID:      kafkaInstanceID(),
		}
	}
	//signal.Notify(kafkaQuit, syscall.SIGINT, syscall.SIGTERM)
}

// Start 启动 Kafka 模式的主循环，同时处理 Kafka 消费与在线事件。
func (k *KafkaServer) Start() {
	defer func() {
		if r := recover(); r != nil {
			zlog.Error(fmt.Sprintf("kafka server panic: %v", r))
		}
		close(k.stopped)
	}()

	// 单独的 goroutine 以 ConsumerGroup 方式持续消费 chat topic，
	// 这样 Kafka 的消费、重平衡和 offset 提交都交给 Sarama 的标准模型处理。
	go k.startRemoteDispatchSubscriber()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				zlog.Error(fmt.Sprintf("kafka server panic: %v", r))
			}
			close(k.readerStopped)
		}()

		consumeCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			<-k.done
			cancel()
		}()

		handler := &kafkaChatConsumerHandler{server: k}
		for {
			if k.IsShuttingDown() {
				return
			}
			err := kafka.KafkaService.ConsumeChatMessages(consumeCtx, handler)
			if err != nil {
				if k.IsShuttingDown() || errors.Is(err, context.Canceled) || errors.Is(err, sarama.ErrClosedConsumerGroup) {
					return
				}
				zlog.Error(err.Error())
				time.Sleep(200 * time.Millisecond)
				continue
			}
		}
	}()

	// login, logout message
	for {
		select {
		case <-k.done:
			return
		case client := <-k.Login:
			{
				k.mutex.Lock()
				k.Clients[client.Uuid] = client
				k.mutex.Unlock()
				k.registerClientRoute(client)
				k.renewClientRoute(client, time.Now())
				if pressure.ShouldLogHotPath(client.Benchmark) {
					zlog.Info("ws.connection.login", client.wsFields(zap.String("event", "ws.connection.login"), zap.String("module", "chat.kafka"))...)
				}
				if !client.Benchmark {
					err := client.writeText([]byte("欢迎来到EchoChat聊天服务器"))
					if err != nil {
						zlog.Error(err.Error())
					}
				}
			}

		case client := <-k.Logout:
			{
				k.mutex.Lock()
				delete(k.Clients, client.Uuid)
				k.mutex.Unlock()
				k.unregisterClientRoute(client)
				zlog.Info("ws.connection.logout", client.wsFields(zap.String("event", "ws.connection.logout"), zap.String("module", "chat.kafka"))...)
				if err := client.writeText([]byte("已退出登录")); err != nil {
					zlog.Error(err.Error())
				}
			}
		}
	}
}

type kafkaChatConsumerHandler struct {
	server *KafkaServer
}

func (h *kafkaChatConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	h.server.consumerReady.Store(true)
	observability.ObserveKafkaConsumerSessionReady(config.GetConfig().KafkaConfig.ResolveConsumerGroup(), true)
	return nil
}

func (h *kafkaChatConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	observability.ObserveKafkaConsumerSessionReady(config.GetConfig().KafkaConfig.ResolveConsumerGroup(), false)
	return nil
}

func (h *kafkaChatConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	if config.GetConfig().KafkaConfig.UseConversationBucketExperimental() {
		return h.server.consumeClaimConversationBucket(session, claim)
	}
	if config.GetConfig().KafkaConfig.UsePartitionAsyncExperimental() {
		return h.server.consumeClaimPartitionAsync(session, claim)
	}
	consumerGroup := config.GetConfig().KafkaConfig.ResolveConsumerGroup()
	commitBatchSize := config.GetConfig().KafkaConfig.ResolveConsumerCommitBatchSize()
	commitInterval := config.GetConfig().KafkaConfig.ResolveConsumerCommitInterval()
	commitTimer := time.NewTimer(commitInterval)
	if !commitTimer.Stop() {
		select {
		case <-commitTimer.C:
		default:
		}
	}
	defer commitTimer.Stop()

	pendingCommitCount := 0
	var commitTimerC <-chan time.Time
	stopCommitTimer := func() {
		if commitTimerC == nil {
			return
		}
		if !commitTimer.Stop() {
			select {
			case <-commitTimer.C:
			default:
			}
		}
		commitTimerC = nil
	}
	armCommitTimer := func() {
		if pendingCommitCount == 0 {
			stopCommitTimer()
			return
		}
		stopCommitTimer()
		commitTimer.Reset(commitInterval)
		commitTimerC = commitTimer.C
	}
	flushPending := func() {
		if pendingCommitCount == 0 {
			return
		}
		stopCommitTimer()
		commitStart := time.Now()
		session.Commit()
		observability.ObserveKafkaOffsetCommit(
			consumerGroup,
			claim.Topic(),
			claim.Partition(),
			time.Since(commitStart),
			pendingCommitCount,
		)
		pendingCommitCount = 0
	}
	defer flushPending()
	for {
		select {
		case <-h.server.done:
			flushPending()
			return nil
		case <-commitTimerC:
			flushPending()
		case kafkaMessage, ok := <-claim.Messages():
			if !ok {
				flushPending()
				return nil
			}
			if kafkaMessage == nil {
				continue
			}
			lag := claim.HighWaterMarkOffset() - kafkaMessage.Offset - 1
			if lag < 0 {
				lag = 0
			}
			observability.ObserveKafkaConsumePull(consumerGroup, kafkaMessage.Topic, kafkaMessage.Partition, lag)
			zlog.Info(
				"kafka.consume.chat",
				zap.String("event", "kafka.consume.chat"),
				zap.String("module", "chat.kafka"),
				zap.String("topic", kafkaMessage.Topic),
				zap.Int32("partition", kafkaMessage.Partition),
				zap.Int64("offset", kafkaMessage.Offset),
				zap.String("key", string(kafkaMessage.Key)),
				zap.Int("value_size", len(kafkaMessage.Value)),
			)
			totalStart := time.Now()
			err := h.server.handleConsumedMessage(kafkaMessage)
			observability.ObserveKafkaConsumerStageDuration(
				consumerGroup,
				kafkaMessage.Topic,
				kafkaMessage.Partition,
				"total",
				time.Since(totalStart),
			)
			if err != nil {
				stage, retryable := consumerErrorMeta(err)
				messageID := messageIDFromKafkaHeaders(kafkaMessage)
				if !retryable {
					var req request.ChatMessageRequest
					if decodeErr := json.Unmarshal(kafkaMessage.Value, &req); decodeErr == nil && req.SendId != "" {
						resolvedMessageID := messageID
						if resolvedMessageID == "" {
							resolvedMessageID = req.MessageId
						}
						if resolvedMessageID != "" {
							clearIngressIdempotencyPending(req.SendId, ensureMessageID(resolvedMessageID))
						}
					}
				}
				if !retryable {
					if dlqErr := writeKafkaMessageToDLQ(kafkaMessage, nil, stage, false, err); dlqErr != nil {
						zlog.Error(dlqErr.Error())
					}
				}
				observability.ObserveKafkaConsumeFailure(consumerGroup, kafkaMessage.Topic, kafkaMessage.Partition, stage)
				result := "non_retryable_failure"
				if retryable {
					result = "retryable_failure"
				}
				observability.ObserveKafkaConsumeHandled(consumerGroup, kafkaMessage.Topic, kafkaMessage.Partition, result)
				zlog.Error(
					"kafka.consume.chat.failed",
					zap.String("event", "kafka.consume.chat.failed"),
					zap.String("module", "chat.kafka"),
					zap.String("topic", kafkaMessage.Topic),
					zap.Int32("partition", kafkaMessage.Partition),
					zap.Int64("offset", kafkaMessage.Offset),
					zap.String("stage", stage),
					zap.Bool("retryable", retryable),
					zap.String("error", err.Error()),
				)
				if retryable {
					continue
				}
			} else {
				observability.ObserveKafkaConsumeHandled(consumerGroup, kafkaMessage.Topic, kafkaMessage.Partition, "success")
			}
			session.MarkMessage(kafkaMessage, "")
			observability.ObserveKafkaConsumeMarked(consumerGroup, kafkaMessage.Topic, kafkaMessage.Partition)
			pendingCommitCount++
			if pendingCommitCount >= commitBatchSize {
				flushPending()
				continue
			}
			armCommitTimer()
		}
	}
}

func observeKafkaConsumerStage(message *sarama.ConsumerMessage, stage string, start time.Time) {
	observability.ObserveKafkaConsumerStageDuration(
		config.GetConfig().KafkaConfig.ResolveConsumerGroup(),
		message.Topic,
		message.Partition,
		stage,
		time.Since(start),
	)
}

func (k *KafkaServer) consumeClaimPartitionAsync(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	zlog.Info(
		"kafka.consume.partition_async_fallback",
		zap.String("event", "kafka.consume.partition_async_fallback"),
		zap.String("module", "chat.kafka"),
		zap.String("reason", "partition_async_not_restored_yet"),
		zap.String("topic", claim.Topic()),
		zap.Int32("partition", claim.Partition()),
	)
	handler := &kafkaChatConsumerHandler{server: k}
	current := config.GetConfig().KafkaConfig.PartitionAsyncEnabled
	config.GetConfig().KafkaConfig.PartitionAsyncEnabled = false
	defer func() {
		config.GetConfig().KafkaConfig.PartitionAsyncEnabled = current
	}()
	return handler.ConsumeClaim(session, claim)
}

func (k *KafkaServer) handleConsumedMessage(kafkaMessage *sarama.ConsumerMessage) error {
	decoded, err := decodeConsumedMessage(kafkaMessage)
	if err != nil {
		return err
	}
	return handleDecodedConsumedMessage(k, decoded)
}

func (k *KafkaServer) handleConsumedGroupMessage(
	kafkaMessage *sarama.ConsumerMessage,
	message *model.Message,
	sendAvatar string,
	messageBackUUID string,
) error {
	observeStage := func(stage string, start time.Time) {
		if kafkaMessage == nil {
			return
		}
		observeKafkaConsumerStage(kafkaMessage, stage, start)
	}

	groupQueryStart := time.Now()
	var group model.GroupInfo
	if res := dao.GormDB.Where("uuid = ?", message.ReceiveId).First(&group); res.Error != nil {
		observeStage("group_member_query", groupQueryStart)
		return retryableConsumerError("group_member_query", res.Error)
	}
	var members []string
	if err := json.Unmarshal(group.Members, &members); err != nil {
		observeStage("group_member_query", groupQueryStart)
		return retryableConsumerError("group_member_query", err)
	}
	observeStage("group_member_query", groupQueryStart)

	messageRsp := respond.GetGroupMessageListRespond{
		MessageId:  message.Uuid,
		SendId:     message.SendId,
		SendName:   message.SendName,
		SendAvatar: sendAvatar,
		ReceiveId:  message.ReceiveId,
		Type:       message.Type,
		Content:    message.Content,
		Url:        message.Url,
		FileSize:   message.FileSize,
		FileName:   message.FileName,
		FileType:   message.FileType,
		SessionSeq: message.SessionSeq,
		CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	jsonMessage, err := json.Marshal(messageRsp)
	if err != nil {
		return retryableConsumerError("websocket_dispatch", err)
	}
	logMessageDispatch("kafka", "group", message, len(jsonMessage))
	dispatchStart := time.Now()
	recipientIDs := make([]string, 0, len(members))
	for _, member := range members {
		if member != message.SendId {
			recipientIDs = append(recipientIDs, member)
			continue
		}
		recipientIDs = append(recipientIDs, message.SendId)
	}
	k.dispatchToKafkaRecipients(recipientIDs, jsonMessage, messageBackUUID)
	observeStage("websocket_dispatch", dispatchStart)
	completeIngressIdempotencyResult(message.SendId, message.Uuid, mustMarshalIngressReplayPayload(messageRsp))

	if config.GetConfig().KafkaConfig.UseGroupAsyncPipelineExperimental() {
		cacheInvalidateStart := time.Now()
		if err := myredis.DelKeyIfExists("group_messagelist_" + message.ReceiveId); err != nil {
			zlog.Error(err.Error())
		}
		observeStage("redis_write", cacheInvalidateStart)
		return nil
	}

	cacheKey := "group_messagelist_" + message.ReceiveId
	redisReadStart := time.Now()
	rspString, redisErr := myredis.GetKeyNilIsErr(cacheKey)
	observeStage("redis_read", redisReadStart)
	if redisErr == nil {
		var rsp []respond.GetGroupMessageListRespond
		if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
			zlog.Error(err.Error())
		} else {
			rsp, _ = appendUniqueGroupCacheMessage(cacheKey, rsp, messageRsp, "kafka")
			rspByte, err := json.Marshal(rsp)
			if err != nil {
				zlog.Error(err.Error())
			} else {
				redisWriteStart := time.Now()
				err = myredis.SetKeyEx(cacheKey, string(rspByte), time.Minute*constants.REDIS_TIMEOUT)
				observeStage("redis_write", redisWriteStart)
				if err != nil {
					if kafkaMessage != nil {
						observability.ObserveKafkaConsumeFailure(config.GetConfig().KafkaConfig.ResolveConsumerGroup(), kafkaMessage.Topic, kafkaMessage.Partition, "redis_write")
					}
					zlog.Error(err.Error())
				} else {
					logMessageCacheUpdate("kafka", cacheKey, len(rsp))
				}
			}
		}
	} else if !errors.Is(redisErr, redis.Nil) {
		if kafkaMessage != nil {
			observability.ObserveKafkaConsumeFailure(config.GetConfig().KafkaConfig.ResolveConsumerGroup(), kafkaMessage.Topic, kafkaMessage.Partition, "redis_read")
		}
		zlog.Error(redisErr.Error())
	}
	return nil
}

func (k *KafkaServer) IsShuttingDown() bool {
	return k.shuttingDown.Load()
}

func (k *KafkaServer) IsConsumerReady() bool {
	return k.consumerReady.Load()
}

func (k *KafkaServer) snapshotAndClearClients() []*Client {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	clients := make([]*Client, 0, len(k.Clients))
	for _, client := range k.Clients {
		clients = append(clients, client)
	}
	k.Clients = make(map[string]*Client)
	return clients
}

func (k *KafkaServer) snapshotClientsByIDs(clientIDs []string) []*Client {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	clients := make([]*Client, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		if client, ok := k.Clients[clientID]; ok {
			clients = append(clients, client)
		}
	}
	return clients
}

func uniqueClientIDs(clientIDs ...string) []string {
	seen := make(map[string]struct{}, len(clientIDs))
	result := make([]string, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		if clientID == "" {
			continue
		}
		if _, ok := seen[clientID]; ok {
			continue
		}
		seen[clientID] = struct{}{}
		result = append(result, clientID)
	}
	return result
}

func (k *KafkaServer) closeClientsGracefully(message string) {
	clients := k.snapshotAndClearClients()
	for _, client := range clients {
		client.notifyClientCritical(message)
	}
	if len(clients) > 0 {
		time.Sleep(100 * time.Millisecond)
	}
	for _, client := range clients {
		k.unregisterClientRoute(client)
		client.Close()
	}
}

// Shutdown 负责优雅停止 Kafka 聊天服务，包括在线连接和 Kafka 消费协程。
func (k *KafkaServer) Shutdown(ctx context.Context) error {
	k.shutdownOnce.Do(func() {
		k.shuttingDown.Store(true)
		k.consumerReady.Store(false)
		close(k.done)
		kafka.KafkaService.KafkaClose()
		k.closeClientsGracefully("服务器正在关闭，请稍后重连")
	})
	select {
	case <-k.stopped:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-k.readerStopped:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-k.dispatchStopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close 兼容旧调用方，内部退化为无超时的优雅关机。
func (k *KafkaServer) Close() {
	_ = k.Shutdown(context.Background())
}

func (k *KafkaServer) GetClient(uuid string) *Client {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	return k.Clients[uuid]
}

func (k *KafkaServer) RemoveClient(client *Client) {
	if client == nil {
		return
	}
	k.mutex.Lock()
	defer k.mutex.Unlock()
	if existing, ok := k.Clients[client.Uuid]; ok && existing == client {
		delete(k.Clients, client.Uuid)
	}
	k.unregisterClientRoute(client)
}

// SendClientToLogin 把客户端加入登录通道，由服务主循环统一注册在线状态。
func (k *KafkaServer) SendClientToLogin(client *Client) bool {
	if k.IsShuttingDown() {
		return false
	}
	select {
	case <-k.done:
		return false
	case k.Login <- client:
		return true
	}
}

// SendClientToLogout 把客户端加入登出通道，由服务主循环统一执行清理。
func (k *KafkaServer) SendClientToLogout(client *Client) bool {
	if k.IsShuttingDown() {
		return false
	}
	select {
	case <-k.done:
		return false
	case k.Logout <- client:
		return true
	}
}
