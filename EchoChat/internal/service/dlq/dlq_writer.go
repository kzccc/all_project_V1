package dlq

import (
	"time"

	"echo_chat_server/internal/model"
)

type CreateRecordInput struct {
	MessageID       string
	ConversationKey string
	SessionSeq      int64
	RawPayload      string
	PayloadSnapshot string
	ContextSnapshot string
	Topic           string
	PartitionID     int32
	OffsetID        int64
	Stage           string
	ErrorCode       string
	LastError       string
	FailureType     string
	HandleType      string
	Status          string
	MaxAttemptCount int
	NextRetryAt     *time.Time
}

func CreateRecord(input CreateRecordInput) error {
	record := &model.DLQMessage{
		MessageID:       input.MessageID,
		ConversationKey: input.ConversationKey,
		SessionSeq:      input.SessionSeq,
		RawPayload:      input.RawPayload,
		PayloadSnapshot: input.PayloadSnapshot,
		ContextSnapshot: input.ContextSnapshot,
		Topic:           input.Topic,
		PartitionID:     input.PartitionID,
		OffsetID:        input.OffsetID,
		Stage:           input.Stage,
		ErrorCode:       input.ErrorCode,
		LastError:       input.LastError,
		FailureType:     input.FailureType,
		HandleType:      input.HandleType,
		Status:          input.Status,
		MaxAttemptCount: input.MaxAttemptCount,
		NextRetryAt:     input.NextRetryAt,
	}
	return Service.Create(record)
}
