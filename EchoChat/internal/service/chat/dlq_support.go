package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"

	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/model"
	dlqservice "echo_chat_server/internal/service/dlq"
)

func writeKafkaMessageToDLQ(kafkaMessage *sarama.ConsumerMessage, decoded *decodedConsumedMessage, stage string, retryable bool, err error) error {
	if kafkaMessage == nil || err == nil {
		return nil
	}
	input := dlqservice.CreateRecordInput{
		MessageID:       messageIDFromKafkaHeaders(kafkaMessage),
		RawPayload:      string(kafkaMessage.Value),
		Topic:           kafkaMessage.Topic,
		PartitionID:     kafkaMessage.Partition,
		OffsetID:        kafkaMessage.Offset,
		Stage:           stage,
		ErrorCode:       dlqErrorCode(stage, err),
		LastError:       err.Error(),
		FailureType:     model.DLQFailureTypePermanent,
		HandleType:      model.DLQHandleTypeManual,
		Status:          model.DLQStatusManual,
		MaxAttemptCount: 0,
	}
	if retryable {
		nextRetryAt := time.Now().Add(10 * time.Second)
		input.FailureType = model.DLQFailureTypeTemporary
		input.HandleType = model.DLQHandleTypeAuto
		input.Status = model.DLQStatusPending
		input.MaxAttemptCount = 5
		input.NextRetryAt = &nextRetryAt
	}
	if decoded != nil {
		input.MessageID = decoded.messageID
		input.ConversationKey = decoded.conversationKey
		input.PayloadSnapshot = marshalDLQSnapshot(decoded.request)
		input.ContextSnapshot = marshalDLQSnapshot(map[string]interface{}{
			"normalized_avatar": decoded.normalizedAvatar,
		})
	}
	if input.MessageID == "" {
		var req request.ChatMessageRequest
		if json.Unmarshal(kafkaMessage.Value, &req) == nil {
			req.MessageId = ensureMessageID(req.MessageId)
			input.MessageID = req.MessageId
			input.ConversationKey = model.BuildConversationKey(req.SendId, req.ReceiveId)
			input.PayloadSnapshot = marshalDLQSnapshot(req)
		}
	}
	return dlqservice.CreateRecord(input)
}

func dlqErrorCode(stage string, err error) string {
	switch stage {
	case "deserialize":
		return "invalid_payload"
	case "route":
		return "invalid_route"
	case "mysql_persist":
		if isConversationSeqDuplicateError(err) {
			return "conversation_seq_conflict"
		}
		if isMessageUUIDDuplicateError(err) {
			return "uuid_duplicate"
		}
		return "mysql_persist_failed"
	case "conversation_bucket_worker_panic":
		return "worker_panic"
	default:
		if errors.Is(err, sarama.ErrClosedConsumerGroup) {
			return "consumer_group_closed"
		}
		return fmt.Sprintf("%s_failed", stage)
	}
}

func writePersistFailureToDLQ(message model.Message, err error) error {
	isSeqConflict := isConversationSeqDuplicateError(err)
	input := dlqservice.CreateRecordInput{
		MessageID:        message.Uuid,
		ConversationKey:  message.ConversationKey,
		SessionSeq:       message.SessionSeq,
		PayloadSnapshot:  marshalDLQSnapshot(message),
		Stage:            "mysql_persist",
		ErrorCode:        dlqErrorCode("mysql_persist", err),
		LastError:        err.Error(),
		FailureType:      model.DLQFailureTypePermanent,
		HandleType:       model.DLQHandleTypeManual,
		Status:           model.DLQStatusManual,
		MaxAttemptCount:  0,
		RawPayload:       marshalDLQSnapshot(message),
	}
	if !isSeqConflict {
		nextRetryAt := time.Now().Add(10 * time.Second)
		input.FailureType = model.DLQFailureTypeTemporary
		input.HandleType = model.DLQHandleTypeAuto
		input.Status = model.DLQStatusPending
		input.MaxAttemptCount = 5
		input.NextRetryAt = &nextRetryAt
	}
	return dlqservice.CreateRecord(input)
}

func writeDispatchFailureToDLQ(stage string, message model.Message, sendAvatar string, err error) error {
	nextRetryAt := time.Now().Add(30 * time.Second)
	return dlqservice.CreateRecord(dlqservice.CreateRecordInput{
		MessageID:        message.Uuid,
		ConversationKey:  message.ConversationKey,
		SessionSeq:       message.SessionSeq,
		RawPayload:       marshalDLQSnapshot(message),
		PayloadSnapshot:  marshalDLQSnapshot(message),
		ContextSnapshot:  marshalDLQSnapshot(map[string]interface{}{"send_avatar": sendAvatar}),
		Stage:            stage,
		ErrorCode:        dlqErrorCode(stage, err),
		LastError:        err.Error(),
		FailureType:      model.DLQFailureTypeTemporary,
		HandleType:       model.DLQHandleTypeAuto,
		Status:           model.DLQStatusPending,
		MaxAttemptCount:  5,
		NextRetryAt:      &nextRetryAt,
	})
}

func marshalDLQSnapshot(payload interface{}) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}
