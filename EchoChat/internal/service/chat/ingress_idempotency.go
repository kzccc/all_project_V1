package chat

import (
	"encoding/json"
	"time"

	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/util/random"
	"echo_chat_server/pkg/zlog"

	"go.uber.org/zap"
)

const (
	ingressIdempotencyPendingTTL = 15 * time.Second
	ingressIdempotencyDoneTTL    = 10 * time.Minute
)

type ingressIdempotencySession struct {
	Key       string
	Owner     string
	MessageID string
	SendID    string
	Acquired  bool
}

func beginIngressIdempotency(sendID, messageID string) (ingressIdempotencySession, myredis.IngressIdempotencyRecord, error) {
	session := ingressIdempotencySession{
		Key:       buildIngressIdempotencyKey(sendID, messageID),
		Owner:     "ING" + random.GetNowAndLenRandomString(6),
		MessageID: messageID,
		SendID:    sendID,
	}
	acquired, record, err := myredis.TryAcquireIngressIdempotency(session.Key, session.Owner, ingressIdempotencyPendingTTL)
	if err != nil {
		return ingressIdempotencySession{}, myredis.IngressIdempotencyRecord{}, err
	}
	session.Acquired = acquired
	return session, record, nil
}

func buildIngressIdempotencyKey(sendID, messageID string) string {
	return "ingress_idempotency:" + sendID + ":" + messageID
}

func (s ingressIdempotencySession) completeWithPayload(payload []byte) {
	s.completePayloadForKey(payload)
}

func (s ingressIdempotencySession) completePayloadForKey(payload []byte) {
	if s.Key == "" {
		return
	}
	ok, err := myredis.MarkIngressIdempotencyDone(s.Key, s.Owner, string(payload), ingressIdempotencyDoneTTL)
	if err != nil {
		zlog.Error(
			"message.ingress_idempotency.done_failed",
			zap.String("event", "message.ingress_idempotency.done_failed"),
			zap.String("module", "chat.ingress"),
			zap.String("message_id", s.MessageID),
			zap.String("send_id", s.SendID),
			zap.String("error", err.Error()),
		)
		return
	}
	if !ok {
		zlog.Info(
			"message.ingress_idempotency.done_skipped",
			zap.String("event", "message.ingress_idempotency.done_skipped"),
			zap.String("module", "chat.ingress"),
			zap.String("message_id", s.MessageID),
			zap.String("send_id", s.SendID),
		)
	}
}

func (s ingressIdempotencySession) clearPending() {
	s.clearPendingForKey()
}

func (s ingressIdempotencySession) clearPendingForKey() {
	if s.Key == "" {
		return
	}
	if _, err := myredis.ClearIngressIdempotencyPending(s.Key, s.Owner); err != nil {
		zlog.Error(
			"message.ingress_idempotency.clear_failed",
			zap.String("event", "message.ingress_idempotency.clear_failed"),
			zap.String("module", "chat.ingress"),
			zap.String("message_id", s.MessageID),
			zap.String("send_id", s.SendID),
			zap.String("error", err.Error()),
		)
	}
}

func completeIngressIdempotencyResult(sendID, messageID string, payload []byte) {
	session := ingressIdempotencySession{
		Key:       buildIngressIdempotencyKey(sendID, messageID),
		MessageID: messageID,
		SendID:    sendID,
	}
	session.completePayloadForKey(payload)
}

func clearIngressIdempotencyPending(sendID, messageID string) {
	session := ingressIdempotencySession{
		Key:       buildIngressIdempotencyKey(sendID, messageID),
		MessageID: messageID,
		SendID:    sendID,
	}
	session.clearPendingForKey()
}

func replayIngressDoneRecord(client *Client, record myredis.IngressIdempotencyRecord) bool {
	if client == nil || record.Status != myredis.IngressIdempotencyStatusDone || record.Payload == "" {
		return false
	}
	if !json.Valid([]byte(record.Payload)) {
		zlog.Error(
			"message.ingress_idempotency.invalid_payload",
			zap.String("event", "message.ingress_idempotency.invalid_payload"),
			zap.String("module", "chat.ingress"),
		)
		return false
	}
	return client.enqueueBack(&MessageBack{
		Message: []byte(record.Payload),
	})
}

func shouldShortCircuitIngressDuplicate(client *Client, messageID string, record myredis.IngressIdempotencyRecord) bool {
	switch record.Status {
	case myredis.IngressIdempotencyStatusPending:
		zlog.Info(
			"message.ingress_idempotency.pending_duplicate",
			zap.String("event", "message.ingress_idempotency.pending_duplicate"),
			zap.String("module", "chat.ingress"),
			zap.String("message_id", messageID),
		)
		return true
	case myredis.IngressIdempotencyStatusDone:
		zlog.Info(
			"message.ingress_idempotency.done_duplicate",
			zap.String("event", "message.ingress_idempotency.done_duplicate"),
			zap.String("module", "chat.ingress"),
			zap.String("message_id", messageID),
		)
		if replayIngressDoneRecord(client, record) {
			return true
		}
		return true
	default:
		if record.Status != "" {
			zlog.Error(
				"message.ingress_idempotency.unknown_status",
				zap.String("event", "message.ingress_idempotency.unknown_status"),
				zap.String("module", "chat.ingress"),
				zap.String("message_id", messageID),
				zap.String("status", string(record.Status)),
			)
			return true
		}
		return false
	}
}

func mustMarshalIngressReplayPayload(message interface{}) []byte {
	payload, err := json.Marshal(message)
	if err != nil {
		zlog.Error(
			"message.ingress_idempotency.payload_marshal_failed",
			zap.String("event", "message.ingress_idempotency.payload_marshal_failed"),
			zap.String("module", "chat.ingress"),
			zap.String("error", err.Error()),
		)
		return nil
	}
	return payload
}
