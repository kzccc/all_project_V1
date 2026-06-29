package chat

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"echo_chat_server/internal/config"
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

func (k *KafkaServer) registerClientRoute(client *Client) {
	if client == nil || client.Uuid == "" {
		return
	}
	if err := myredis.SetKey(kafkaWSRouteKey(client.Uuid), k.instanceID); err != nil {
		zlog.Error(
			"kafka.ws.route.register_failed",
			zap.String("event", "kafka.ws.route.register_failed"),
			zap.String("module", "chat.kafka"),
			zap.String("instance_id", k.instanceID),
			zap.String("client_id", client.Uuid),
			zap.String("error", err.Error()),
		)
	}
}

func (k *KafkaServer) unregisterClientRoute(client *Client) {
	if client == nil || client.Uuid == "" {
		return
	}
	if err := myredis.DelKeyIfValueMatches(kafkaWSRouteKey(client.Uuid), k.instanceID); err != nil {
		zlog.Error(
			"kafka.ws.route.unregister_failed",
			zap.String("event", "kafka.ws.route.unregister_failed"),
			zap.String("module", "chat.kafka"),
			zap.String("instance_id", k.instanceID),
			zap.String("client_id", client.Uuid),
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
	return value
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
				_ = myredis.DelKeyIfValueMatches(kafkaWSRouteKey(envelope.ClientID), k.instanceID)
				continue
			}
			client.enqueueBack(&MessageBack{
				Message: []byte(envelope.Message),
				Uuid:    envelope.UUID,
			})
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
			client.enqueueBack(&MessageBack{
				Message: message,
				Uuid:    dispatchUUID,
			})
			continue
		}

		remoteInstanceID := k.remoteRouteForClient(recipientID)
		if remoteInstanceID == "" {
			continue
		}
		if remoteInstanceID == k.instanceID {
			_ = myredis.DelKeyIfValueMatches(kafkaWSRouteKey(recipientID), k.instanceID)
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

		if err := myredis.Publish(kafkaWSDispatchChannel(remoteInstanceID), string(payload)); err != nil {
			zlog.Error(
				"kafka.ws.dispatch.publish_failed",
				zap.String("event", "kafka.ws.dispatch.publish_failed"),
				zap.String("module", "chat.kafka"),
				zap.String("instance_id", k.instanceID),
				zap.String("client_id", recipientID),
				zap.String("remote_instance_id", remoteInstanceID),
				zap.String("error", err.Error()),
			)
		}
	}
}
