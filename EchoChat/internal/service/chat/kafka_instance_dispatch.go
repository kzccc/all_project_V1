package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/observability"
	"echo_chat_server/internal/pressure"
	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/zlog"
)

const (
	kafkaWSRouteKeyPrefix        = "echochat:kafka:ws_route:"
	kafkaWSDispatchChannelPrefix = "echochat:kafka:ws_dispatch:"
)

type kafkaWSDispatchEnvelope struct {
	ClientID string `json:"client_id"`
	Message  string `json:"message"`
	UUID     string `json:"uuid"`
}

type kafkaWSRouteRecord struct {
	InstanceID      string `json:"instance_id"`
	ConnectionID    string `json:"connection_id"`
	ActiveAtUnix    int64  `json:"active_at_unix"`
	ConnectedAtUnix int64  `json:"connected_at_unix"`
}

func kafkaInstanceID() string {
	conf := config.GetConfig()
	return fmt.Sprintf("chat-%d", conf.MainConfig.Port)
}

func kafkaWSRouteKey(clientID string) string {
	return kafkaWSRouteKeyPrefix + clientID
}

func kafkaWSDispatchChannel(instanceID string) string {
	return kafkaWSDispatchChannelPrefix + instanceID
}

func newKafkaWSRouteRecord(client *Client, instanceID string, now time.Time) kafkaWSRouteRecord {
	connectedAt := now.Unix()
	if client != nil && !client.connectedAt.IsZero() {
		connectedAt = client.connectedAt.Unix()
	}
	return kafkaWSRouteRecord{
		InstanceID:      instanceID,
		ConnectionID:    client.ConnectionID,
		ActiveAtUnix:    now.Unix(),
		ConnectedAtUnix: connectedAt,
	}
}

func encodeKafkaWSRouteRecord(record kafkaWSRouteRecord) (string, error) {
	payload, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeKafkaWSRouteRecord(raw string) (kafkaWSRouteRecord, error) {
	var record kafkaWSRouteRecord
	if raw == "" {
		return record, errors.New("empty route record")
	}
	if raw[0] != '{' {
		record.InstanceID = raw
		return record, nil
	}
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return kafkaWSRouteRecord{}, err
	}
	if record.InstanceID == "" {
		return kafkaWSRouteRecord{}, errors.New("route record missing instance_id")
	}
	return record, nil
}

func deleteKafkaWSRouteIfMatches(clientID string, expectedInstanceID string, expectedConnectionID string) error {
	raw, err := myredis.GetKeyNilIsErr(kafkaWSRouteKey(clientID))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return err
	}
	record, err := decodeKafkaWSRouteRecord(raw)
	if err != nil {
		return err
	}
	if record.InstanceID != expectedInstanceID {
		return nil
	}
	if expectedConnectionID != "" && record.ConnectionID != expectedConnectionID {
		return nil
	}
	return myredis.DelKeyIfValueMatches(kafkaWSRouteKey(clientID), raw)
}

func (k *KafkaServer) registerClientRoute(client *Client) {
	if client == nil || client.Uuid == "" {
		return
	}
	record := newKafkaWSRouteRecord(client, k.instanceID, time.Now())
	payload, err := encodeKafkaWSRouteRecord(record)
	if err != nil {
		zlog.Error(
			"kafka.ws.route.encode_failed",
			zap.String("event", "kafka.ws.route.encode_failed"),
			zap.String("module", "chat.kafka"),
			zap.String("instance_id", k.instanceID),
			zap.String("client_id", client.Uuid),
			zap.String("connection_id", client.ConnectionID),
			zap.String("error", err.Error()),
		)
		return
	}
	if err := myredis.SetKeyEx(kafkaWSRouteKey(client.Uuid), payload, config.GetConfig().KafkaConfig.ResolveWSRouteTTL()); err != nil {
		zlog.Error(
			"kafka.ws.route.register_failed",
			zap.String("event", "kafka.ws.route.register_failed"),
			zap.String("module", "chat.kafka"),
			zap.String("instance_id", k.instanceID),
			zap.String("client_id", client.Uuid),
			zap.String("connection_id", client.ConnectionID),
			zap.String("error", err.Error()),
		)
	}
}

func (k *KafkaServer) renewClientRoute(client *Client, now time.Time) {
	if client == nil || client.Uuid == "" || client.ConnectionID == "" {
		return
	}
	record := newKafkaWSRouteRecord(client, k.instanceID, now)
	payload, err := encodeKafkaWSRouteRecord(record)
	if err != nil {
		zlog.Error(
			"kafka.ws.route.renew_encode_failed",
			zap.String("event", "kafka.ws.route.renew_encode_failed"),
			zap.String("module", "chat.kafka"),
			zap.String("instance_id", k.instanceID),
			zap.String("client_id", client.Uuid),
			zap.String("connection_id", client.ConnectionID),
			zap.String("error", err.Error()),
		)
		return
	}
	if err := myredis.SetKeyEx(kafkaWSRouteKey(client.Uuid), payload, config.GetConfig().KafkaConfig.ResolveWSRouteTTL()); err != nil {
		zlog.Error(
			"kafka.ws.route.renew_failed",
			zap.String("event", "kafka.ws.route.renew_failed"),
			zap.String("module", "chat.kafka"),
			zap.String("instance_id", k.instanceID),
			zap.String("client_id", client.Uuid),
			zap.String("connection_id", client.ConnectionID),
			zap.String("error", err.Error()),
		)
	}
}

func (k *KafkaServer) unregisterClientRoute(client *Client) {
	if client == nil || client.Uuid == "" {
		return
	}
	if err := deleteKafkaWSRouteIfMatches(client.Uuid, k.instanceID, client.ConnectionID); err != nil {
		zlog.Error(
			"kafka.ws.route.unregister_failed",
			zap.String("event", "kafka.ws.route.unregister_failed"),
			zap.String("module", "chat.kafka"),
			zap.String("instance_id", k.instanceID),
			zap.String("client_id", client.Uuid),
			zap.String("connection_id", client.ConnectionID),
			zap.String("error", err.Error()),
		)
	}
}

func (k *KafkaServer) remoteRouteForClient(clientID string) string {
	if clientID == "" {
		return ""
	}
	value, err := myredis.GetKeyNilIsErr(kafkaWSRouteKey(clientID))
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			zlog.Error(
				"kafka.ws.route.lookup_failed",
				zap.String("event", "kafka.ws.route.lookup_failed"),
				zap.String("module", "chat.kafka"),
				zap.String("instance_id", k.instanceID),
				zap.String("client_id", clientID),
				zap.String("error", err.Error()),
			)
		}
		return ""
	}
	record, err := decodeKafkaWSRouteRecord(value)
	if err != nil {
		zlog.Error(
			"kafka.ws.route.decode_failed",
			zap.String("event", "kafka.ws.route.decode_failed"),
			zap.String("module", "chat.kafka"),
			zap.String("instance_id", k.instanceID),
			zap.String("client_id", clientID),
			zap.String("error", err.Error()),
		)
		_ = myredis.DelKeyIfExists(kafkaWSRouteKey(clientID))
		return ""
	}
	return record.InstanceID
}

func (k *KafkaServer) startRemoteDispatchSubscriber() {
	channelName := kafkaWSDispatchChannel(k.instanceID)
	pubsub := myredis.Subscribe(channelName)
	channel := pubsub.Channel()
	defer func() {
		_ = pubsub.Close()
		close(k.dispatchStopped)
	}()

	for {
		select {
		case <-k.done:
			return
		case message, ok := <-channel:
			if !ok {
				return
			}
			var envelope kafkaWSDispatchEnvelope
			if err := json.Unmarshal([]byte(message.Payload), &envelope); err != nil {
				zlog.Error(
					"kafka.ws.dispatch.decode_failed",
					zap.String("event", "kafka.ws.dispatch.decode_failed"),
					zap.String("module", "chat.kafka"),
					zap.String("instance_id", k.instanceID),
					zap.String("channel", channelName),
					zap.String("error", err.Error()),
				)
				continue
			}
			client := k.GetClient(envelope.ClientID)
			if client == nil {
				observability.ObserveKafkaWSDispatchEvent(k.instanceID, "remote_receive_client_miss")
				_ = deleteKafkaWSRouteIfMatches(envelope.ClientID, k.instanceID, "")
				continue
			}
			enqueueStart := time.Now()
			client.enqueueBack(&MessageBack{
				Message: []byte(envelope.Message),
				Uuid:    envelope.UUID,
			})
			observability.ObserveKafkaWSDispatchEvent(k.instanceID, "remote_receive_local_enqueue")
			observability.ObserveKafkaWSDispatchDuration(k.instanceID, "remote_receive_local_enqueue", time.Since(enqueueStart))
		}
	}
}

func (k *KafkaServer) dispatchToKafkaRecipients(recipientIDs []string, message []byte, messageUUID string) {
	targets := uniqueClientIDs(recipientIDs...)
	updateAssigned := false

	for _, recipientID := range targets {
		if recipientID == "" {
			continue
		}

		dispatchUUID := ""
		if !updateAssigned && messageUUID != "" {
			dispatchUUID = messageUUID
			updateAssigned = true
		}

		if client := k.GetClient(recipientID); client != nil {
			enqueueStart := time.Now()
			if dispatchUUID != "" {
				pressure.ObserveBenchmarkEventAt(dispatchUUID, "dispatch_after_persist_start", enqueueStart, map[string]interface{}{
					"dispatch_mode": "local_direct",
					"client_id":     recipientID,
				})
			}
			client.enqueueBack(&MessageBack{
				Message: message,
				Uuid:    dispatchUUID,
			})
			observability.ObserveKafkaWSDispatchEvent(k.instanceID, "local_direct_enqueue")
			observability.ObserveKafkaWSDispatchDuration(k.instanceID, "local_direct_enqueue", time.Since(enqueueStart))
			continue
		}

		lookupStart := time.Now()
		remoteInstanceID := k.remoteRouteForClient(recipientID)
		observability.ObserveKafkaWSDispatchDuration(k.instanceID, "route_lookup", time.Since(lookupStart))
		if remoteInstanceID == "" {
			observability.ObserveKafkaWSDispatchEvent(k.instanceID, "route_lookup_miss")
			continue
		}
		observability.ObserveKafkaWSDispatchEvent(k.instanceID, "route_lookup_hit")
		if remoteInstanceID == k.instanceID {
			observability.ObserveKafkaWSDispatchEvent(k.instanceID, "route_lookup_stale_self")
			_ = deleteKafkaWSRouteIfMatches(recipientID, k.instanceID, "")
			continue
		}
		envelope := kafkaWSDispatchEnvelope{
			ClientID: recipientID,
			Message:  string(message),
			UUID:     dispatchUUID,
		}
		payload, err := json.Marshal(envelope)
		if err != nil {
			zlog.Error(
				"kafka.ws.dispatch.encode_failed",
				zap.String("event", "kafka.ws.dispatch.encode_failed"),
				zap.String("module", "chat.kafka"),
				zap.String("instance_id", k.instanceID),
				zap.String("client_id", recipientID),
				zap.String("remote_instance_id", remoteInstanceID),
				zap.String("error", err.Error()),
			)
			continue
		}

		publishStart := time.Now()
		if dispatchUUID != "" {
			pressure.ObserveBenchmarkEventAt(dispatchUUID, "dispatch_after_persist_start", publishStart, map[string]interface{}{
				"dispatch_mode": "remote_publish",
				"client_id":     recipientID,
				"remote_instance_id": remoteInstanceID,
			})
		}
		if err := myredis.Publish(kafkaWSDispatchChannel(remoteInstanceID), string(payload)); err != nil {
			observability.ObserveKafkaWSDispatchEvent(k.instanceID, "remote_publish_failed")
			observability.ObserveKafkaWSDispatchDuration(k.instanceID, "remote_publish", time.Since(publishStart))
			zlog.Error(
				"kafka.ws.dispatch.publish_failed",
				zap.String("event", "kafka.ws.dispatch.publish_failed"),
				zap.String("module", "chat.kafka"),
				zap.String("instance_id", k.instanceID),
				zap.String("client_id", recipientID),
				zap.String("remote_instance_id", remoteInstanceID),
				zap.String("error", err.Error()),
			)
		} else {
			observability.ObserveKafkaWSDispatchEvent(k.instanceID, "remote_publish_success")
			observability.ObserveKafkaWSDispatchDuration(k.instanceID, "remote_publish", time.Since(publishStart))
		}
	}
}
