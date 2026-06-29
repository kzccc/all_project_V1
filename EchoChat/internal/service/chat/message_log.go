package chat

import (
	"echo_chat_server/internal/model"
	"echo_chat_server/pkg/zlog"

	"go.uber.org/zap"
)

func messageCoreFields(mode string, message *model.Message) []zap.Field {
	return []zap.Field{
		zap.String("module", "chat."+mode),
		zap.String("message_uuid", message.Uuid),
		zap.String("session_id", message.SessionId),
		zap.Int64("session_seq", message.SessionSeq),
		zap.Int8("message_type", message.Type),
		zap.String("send_id", message.SendId),
		zap.String("receive_id", message.ReceiveId),
	}
}

func logMessagePersist(mode string, message *model.Message) {
	fields := append([]zap.Field{zap.String("event", "message.persist")}, messageCoreFields(mode, message)...)
	zlog.Info("message.persist", fields...)
}

func logMessageDispatch(mode, target string, message *model.Message, payloadSize int) {
	fields := append(
		[]zap.Field{
			zap.String("event", "message.dispatch"),
			zap.String("target", target),
			zap.Int("payload_size", payloadSize),
		},
		messageCoreFields(mode, message)...,
	)
	zlog.Info("message.dispatch", fields...)
}

func logMessageCacheUpdate(mode, cacheKey string, count int) {
	zlog.Info(
		"message.cache_update",
		zap.String("event", "message.cache_update"),
		zap.String("module", "chat."+mode),
		zap.String("cache_key", cacheKey),
		zap.Int("message_count", count),
	)
}

func logMessageCacheDuplicateSkip(mode, cacheKey, messageID string, sessionSeq int64) {
	zlog.Info(
		"message.cache_duplicate_skipped",
		zap.String("event", "message.cache_duplicate_skipped"),
		zap.String("module", "chat."+mode),
		zap.String("cache_key", cacheKey),
		zap.String("message_id", messageID),
		zap.Int64("session_seq", sessionSeq),
	)
}
