package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/go-redis/redis/v8"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/dto/respond"
	"echo_chat_server/internal/model"
	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/enum/message/message_status_enum"
	"echo_chat_server/pkg/enum/message/message_type_enum"
	"echo_chat_server/pkg/zlog"
)

type decodedConsumedMessage struct {
	kafkaMessage     *sarama.ConsumerMessage
	request          request.ChatMessageRequest
	messageID        string
	conversationKey  string
	normalizedAvatar string
}

func messageIDFromKafkaHeaders(kafkaMessage *sarama.ConsumerMessage) string {
	if kafkaMessage == nil {
		return ""
	}
	for _, header := range kafkaMessage.Headers {
		if string(header.Key) == "message_id" {
			return string(header.Value)
		}
	}
	return ""
}

func decodeConsumedMessage(kafkaMessage *sarama.ConsumerMessage) (decodedConsumedMessage, error) {
	data := kafkaMessage.Value
	var chatMessageReq request.ChatMessageRequest
	decodeStart := time.Now()
	_ = messageIDFromKafkaHeaders(kafkaMessage)
	if err := json.Unmarshal(data, &chatMessageReq); err != nil {
		observeKafkaConsumerStage(kafkaMessage, "deserialize", decodeStart)
		return decodedConsumedMessage{}, nonRetryableConsumerError("deserialize", err)
	}
	observeKafkaConsumerStage(kafkaMessage, "deserialize", decodeStart)
	chatMessageReq.MessageId = ensureMessageID(chatMessageReq.MessageId)
	if chatMessageReq.SendId == "" || chatMessageReq.ReceiveId == "" {
		return decodedConsumedMessage{}, nonRetryableConsumerError("route", errors.New("send_id or receive_id is empty"))
	}
	return decodedConsumedMessage{
		kafkaMessage:     kafkaMessage,
		request:          chatMessageReq,
		messageID:        chatMessageReq.MessageId,
		conversationKey:  model.BuildConversationKey(chatMessageReq.SendId, chatMessageReq.ReceiveId),
		normalizedAvatar: normalizePath(chatMessageReq.SendAvatar),
	}, nil
}

func handleDecodedConsumedMessage(k *KafkaServer, decoded decodedConsumedMessage) error {
	kafkaMessage := decoded.kafkaMessage
	chatMessageReq := decoded.request

	seqStart := time.Now()
	sessionSeq, _, err := nextMessageSessionSeqDetail(chatMessageReq.SendId, chatMessageReq.ReceiveId)
	observeKafkaConsumerStage(kafkaMessage, "session_seq", seqStart)
	if err != nil {
		return retryableConsumerError("session_seq", err)
	}

	message := model.Message{
		Uuid:            decoded.messageID,
		SessionId:       chatMessageReq.SessionId,
		Type:            chatMessageReq.Type,
		Content:         chatMessageReq.Content,
		Url:             chatMessageReq.Url,
		SendId:          chatMessageReq.SendId,
		SendName:        chatMessageReq.SendName,
		SendAvatar:      decoded.normalizedAvatar,
		ReceiveId:       chatMessageReq.ReceiveId,
		ConversationKey: decoded.conversationKey,
		FileSize:        chatMessageReq.FileSize,
		FileType:        chatMessageReq.FileType,
		FileName:        chatMessageReq.FileName,
		Status:          message_status_enum.Unsent,
		SessionSeq:      sessionSeq,
		CreatedAt:       time.Now(),
		AVdata:          chatMessageReq.AVdata,
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
