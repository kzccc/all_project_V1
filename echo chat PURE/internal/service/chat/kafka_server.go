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

	"github.com/IBM/sarama"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/dto/respond"
	"echo_chat_server/internal/model"
	"echo_chat_server/internal/service/kafka"
	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/enum/message/message_status_enum"
	"echo_chat_server/pkg/enum/message/message_type_enum"
	"echo_chat_server/pkg/zlog"
)

type KafkaServer struct {
	Clients map[string]*Client
	mutex   *sync.Mutex
	Login   chan *Client
	Logout  chan *Client

	shutdownOnce    sync.Once
	done            chan struct{}
	stopped         chan struct{}
	readerStopped   chan struct{}
	dispatchStopped chan struct{}
	instanceID      string

	shuttingDown  atomic.Bool
	consumerReady atomic.Bool
}

var KafkaChatServer *KafkaServer

var kafkaQuit = make(chan os.Signal, 1)

func init() {
	if KafkaChatServer == nil {
		KafkaChatServer = &KafkaServer{
			Clients: make(map[string]*Client),
			mutex:   &sync.Mutex{},
			Login:   make(chan *Client),
			Logout:  make(chan *Client),

			done:            make(chan struct{}),
			stopped:         make(chan struct{}),
			readerStopped:   make(chan struct{}),
			dispatchStopped: make(chan struct{}),
			instanceID:      kafkaInstanceID(),
		}
	}
}

func (k *KafkaServer) Start() {
	defer func() {
		if r := recover(); r != nil {
			zlog.Error(fmt.Sprintf("kafka server panic: %v", r))
		}
		close(k.stopped)
	}()

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

	for {
		select {
		case <-k.done:
			return
		case client := <-k.Login:
			k.mutex.Lock()
			k.Clients[client.Uuid] = client
			k.mutex.Unlock()
			k.registerClientRoute(client)
			zlog.Info("ws.connection.login", client.wsFields(zap.String("event", "ws.connection.login"), zap.String("module", "chat.kafka"))...)
			if err := client.Conn.WriteMessage(websocket.TextMessage, []byte("欢迎来到EchoChat聊天服务器")); err != nil {
				zlog.Error(err.Error())
			}
		case client := <-k.Logout:
			k.mutex.Lock()
			delete(k.Clients, client.Uuid)
			k.mutex.Unlock()
			k.unregisterClientRoute(client)
			zlog.Info("ws.connection.logout", client.wsFields(zap.String("event", "ws.connection.logout"), zap.String("module", "chat.kafka"))...)
			if err := client.Conn.WriteMessage(websocket.TextMessage, []byte("已退出登录")); err != nil {
				zlog.Error(err.Error())
			}
		}
	}
}

type kafkaChatConsumerHandler struct {
	server *KafkaServer
}

func (h *kafkaChatConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	h.server.consumerReady.Store(true)
	return nil
}

func (h *kafkaChatConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.server.consumerReady.Store(false)
	return nil
}

func (h *kafkaChatConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	if config.GetConfig().KafkaConfig.UseConversationBucketExperimental() {
		return h.server.consumeClaimConversationBucket(session, claim)
	}
	if config.GetConfig().KafkaConfig.UsePartitionAsyncExperimental() {
		return h.server.consumeClaimPartitionAsync(session, claim)
	}
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
		session.Commit()
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
			if err := h.server.handleConsumedMessage(kafkaMessage); err != nil {
				stage, retryable := consumerErrorMeta(err)
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
			}
			session.MarkMessage(kafkaMessage, "")
			pendingCommitCount++
			if pendingCommitCount >= commitBatchSize {
				flushPending()
				continue
			}
			armCommitTimer()
		}
	}
}

func observeKafkaConsumerStage(_ *sarama.ConsumerMessage, _ string, _ time.Time) {}

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
	data := kafkaMessage.Value
	var chatMessageReq request.ChatMessageRequest
	decodeStart := time.Now()
	if err := json.Unmarshal(data, &chatMessageReq); err != nil {
		observeKafkaConsumerStage(kafkaMessage, "deserialize", decodeStart)
		zlog.Error(
			"message.decode.failed",
			zap.String("event", "message.decode.failed"),
			zap.String("module", "chat.kafka"),
			zap.Int("payload_size", len(data)),
			zap.String("error", err.Error()),
		)
		return nonRetryableConsumerError("deserialize", err)
	}
	observeKafkaConsumerStage(kafkaMessage, "deserialize", decodeStart)
	chatMessageReq.MessageId = ensureMessageID(chatMessageReq.MessageId)
	if chatMessageReq.SendId == "" || chatMessageReq.ReceiveId == "" {
		return nonRetryableConsumerError("route", errors.New("send_id or receive_id is empty"))
	}

	seqStart := time.Now()
	sessionSeq, err := nextMessageSessionSeq(chatMessageReq.SendId, chatMessageReq.ReceiveId)
	observeKafkaConsumerStage(kafkaMessage, "session_seq", seqStart)
	if err != nil {
		return retryableConsumerError("session_seq", err)
	}

	message := model.Message{
		Uuid:       chatMessageReq.MessageId,
		SessionId:  chatMessageReq.SessionId,
		Type:       chatMessageReq.Type,
		Content:    chatMessageReq.Content,
		Url:        chatMessageReq.Url,
		SendId:     chatMessageReq.SendId,
		SendName:   chatMessageReq.SendName,
		SendAvatar: normalizePath(chatMessageReq.SendAvatar),
		ReceiveId:  chatMessageReq.ReceiveId,
		ConversationKey: model.BuildConversationKey(
			chatMessageReq.SendId,
			chatMessageReq.ReceiveId,
		),
		FileSize:   chatMessageReq.FileSize,
		FileType:   chatMessageReq.FileType,
		FileName:   chatMessageReq.FileName,
		Status:     message_status_enum.Unsent,
		SessionSeq: sessionSeq,
		CreatedAt:  time.Now(),
		AVdata:     chatMessageReq.AVdata,
	}

	switch chatMessageReq.Type {
	case message_type_enum.Text:
		message.Url = ""
		message.FileSize = "0B"
		message.FileType = ""
		message.FileName = ""
	case message_type_enum.File:
		message.Content = ""
	case message_type_enum.AudioOrVideo:
		message.Content = ""
		message.Url = ""
		message.FileSize = ""
		message.FileType = ""
		message.FileName = ""
	default:
		return nonRetryableConsumerError("route", fmt.Errorf("unsupported message type: %d", chatMessageReq.Type))
	}

	shouldPersist := chatMessageReq.Type != message_type_enum.AudioOrVideo
	if chatMessageReq.Type == message_type_enum.AudioOrVideo {
		var avData request.AVData
		if err := json.Unmarshal([]byte(chatMessageReq.AVdata), &avData); err != nil {
			return nonRetryableConsumerError("deserialize", err)
		}
		shouldPersist = avData.MessageId == "PROXY" && (avData.Type == "start_call" || avData.Type == "receive_call" || avData.Type == "reject_call")
	}

	messageBackUUID := ""
	useGroupAsyncPipeline := len(message.ReceiveId) > 0 &&
		message.ReceiveId[0] == 'G' &&
		config.GetConfig().KafkaConfig.UseGroupAsyncPipelineExperimental()
	if shouldPersist && !useGroupAsyncPipeline {
		mysqlStart := time.Now()
		alreadyProcessed, persisted, err := saveKafkaMessage(&message)
		observeKafkaConsumerStage(kafkaMessage, "mysql_persist", mysqlStart)
		if err != nil {
			return retryableConsumerError("mysql_persist", err)
		}
		if alreadyProcessed {
			return nil
		}
		if persisted {
			messageBackUUID = message.Uuid
		}
	}

	switch {
	case len(message.ReceiveId) == 0:
		return nonRetryableConsumerError("route", errors.New("receive_id is empty"))
	case message.ReceiveId[0] == 'U':
		switch chatMessageReq.Type {
		case message_type_enum.Text, message_type_enum.File:
			messageRsp := respond.GetMessageListRespond{
				MessageId:  message.Uuid,
				SendId:     message.SendId,
				SendName:   message.SendName,
				SendAvatar: chatMessageReq.SendAvatar,
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
			logMessageDispatch("kafka", "user", &message, len(jsonMessage))
			dispatchStart := time.Now()
			k.dispatchToKafkaRecipients([]string{message.ReceiveId, message.SendId}, jsonMessage, messageBackUUID)
			observeKafkaConsumerStage(kafkaMessage, "websocket_dispatch", dispatchStart)

			cacheKey := "message_list_" + message.SendId + "_" + message.ReceiveId
			redisReadStart := time.Now()
			rspString, redisErr := myredis.GetKeyNilIsErr(cacheKey)
			observeKafkaConsumerStage(kafkaMessage, "redis_read", redisReadStart)
			if redisErr == nil {
				var rsp []respond.GetMessageListRespond
				if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
					zlog.Error(err.Error())
				} else {
					rsp, _ = appendUniqueUserCacheMessage(cacheKey, rsp, messageRsp, "kafka")
					rspByte, err := json.Marshal(rsp)
					if err != nil {
						zlog.Error(err.Error())
					} else {
						redisWriteStart := time.Now()
						err = myredis.SetKeyEx(cacheKey, string(rspByte), time.Minute*constants.REDIS_TIMEOUT)
						observeKafkaConsumerStage(kafkaMessage, "redis_write", redisWriteStart)
						if err != nil {
							zlog.Error(err.Error())
						} else {
							logMessageCacheUpdate("kafka", cacheKey, len(rsp))
						}
					}
				}
			} else if !errors.Is(redisErr, redis.Nil) {
				zlog.Error(redisErr.Error())
			}
		case message_type_enum.AudioOrVideo:
			messageRsp := respond.AVMessageRespond{
				MessageId:  message.Uuid,
				SendId:     message.SendId,
				SendName:   message.SendName,
				SendAvatar: message.SendAvatar,
				ReceiveId:  message.ReceiveId,
				Type:       message.Type,
				Content:    message.Content,
				Url:        message.Url,
				FileSize:   message.FileSize,
				FileName:   message.FileName,
				FileType:   message.FileType,
				SessionSeq: message.SessionSeq,
				CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
				AVdata:     message.AVdata,
			}
			jsonMessage, err := json.Marshal(messageRsp)
			if err != nil {
				return retryableConsumerError("websocket_dispatch", err)
			}
			logMessageDispatch("kafka", "user", &message, len(jsonMessage))
			dispatchStart := time.Now()
			k.dispatchToKafkaRecipients([]string{message.ReceiveId}, jsonMessage, messageBackUUID)
			observeKafkaConsumerStage(kafkaMessage, "websocket_dispatch", dispatchStart)
		}
	case message.ReceiveId[0] == 'G':
		if chatMessageReq.Type == message_type_enum.AudioOrVideo {
			return nonRetryableConsumerError("route", errors.New("group audio/video is not supported"))
		}
		if useGroupAsyncPipeline {
			enqueueStart := time.Now()
			if err := k.enqueueConsumedGroupMessage(message, chatMessageReq.SendAvatar); err != nil {
				observeKafkaConsumerStage(kafkaMessage, "group_async_enqueue", enqueueStart)
				return retryableConsumerError("group_async_enqueue", err)
			}
			observeKafkaConsumerStage(kafkaMessage, "group_async_enqueue", enqueueStart)
			return nil
		}
		return k.handleConsumedGroupMessage(kafkaMessage, &message, chatMessageReq.SendAvatar, messageBackUUID)
	default:
		return nonRetryableConsumerError("route", fmt.Errorf("unsupported receive_id: %s", message.ReceiveId))
	}
	return nil
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
					zlog.Error(err.Error())
				} else {
					logMessageCacheUpdate("kafka", cacheKey, len(rsp))
				}
			}
		}
	} else if !errors.Is(redisErr, redis.Nil) {
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
		client.Close()
	}
}

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
