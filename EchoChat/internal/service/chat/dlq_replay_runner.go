package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/dto/respond"
	"echo_chat_server/internal/model"
	dlqservice "echo_chat_server/internal/service/dlq"
	"echo_chat_server/pkg/enum/message/message_status_enum"
	"echo_chat_server/pkg/enum/message/message_type_enum"
)

func RunDLQReplay(record *model.DLQMessage) dlqservice.ReplayResult {
	if record == nil {
		return dlqservice.ReplayResult{Success: false, Retryable: false, FinalError: fmt.Errorf("dlq record is nil")}
	}
	switch record.Stage {
	case "session_seq":
		return replaySessionSeq(record)
	case "mysql_persist":
		return replayMysqlPersist(record)
	case "websocket_dispatch":
		return replayWebsocketDispatch(record)
	case "group_member_query":
		return replayGroupMemberQuery(record)
	default:
		return dlqservice.ReplayResult{Success: false, Retryable: false, FinalError: fmt.Errorf("unsupported replay stage: %s", record.Stage)}
	}
}

func replaySessionSeq(record *model.DLQMessage) dlqservice.ReplayResult {
	req, err := decodeChatMessageRequest(record)
	if err != nil {
		return dlqservice.ReplayResult{Success: false, Retryable: false, FinalError: err}
	}
	if req.SendId == "" || req.ReceiveId == "" {
		return dlqservice.ReplayResult{Success: false, Retryable: false, FinalError: fmt.Errorf("send_id or receive_id is empty")}
	}
	sessionSeq, _, err := nextMessageSessionSeqDetail(req.SendId, req.ReceiveId)
	if err != nil {
		nextRetryAt := time.Now().Add(10 * time.Second)
		return dlqservice.ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: err}
	}
	message := buildMessageFromRequest(req, sessionSeq)
	if _, _, err := saveKafkaMessage(&message); err != nil {
		nextRetryAt := time.Now().Add(10 * time.Second)
		return dlqservice.ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: err}
	}
	if err := dispatchPersistedMessage(&message, req.SendAvatar, message.Uuid); err != nil {
		nextRetryAt := time.Now().Add(10 * time.Second)
		return dlqservice.ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: err}
	}
	return dlqservice.ReplayResult{Success: true}
}

func replayMysqlPersist(record *model.DLQMessage) dlqservice.ReplayResult {
	message, err := decodeDLQMessage(record)
	if err != nil {
		return dlqservice.ReplayResult{Success: false, Retryable: false, FinalError: err}
	}
	alreadyProcessed, persisted, err := saveKafkaMessage(&message)
	if err != nil {
		nextRetryAt := time.Now().Add(10 * time.Second)
		return dlqservice.ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: err}
	}
	if alreadyProcessed {
		return dlqservice.ReplayResult{Success: true}
	}
	if persisted {
		if err := dispatchPersistedMessage(&message, message.SendAvatar, message.Uuid); err != nil {
			nextRetryAt := time.Now().Add(10 * time.Second)
			return dlqservice.ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: err}
		}
	}
	return dlqservice.ReplayResult{Success: true}
}

func replayWebsocketDispatch(record *model.DLQMessage) dlqservice.ReplayResult {
	message, err := decodeDLQMessage(record)
	if err != nil {
		return dlqservice.ReplayResult{Success: false, Retryable: false, FinalError: err}
	}
	if err := dispatchPersistedMessage(&message, message.SendAvatar, message.Uuid); err != nil {
		nextRetryAt := time.Now().Add(30 * time.Second)
		return dlqservice.ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: err}
	}
	return dlqservice.ReplayResult{Success: true}
}

func replayGroupMemberQuery(record *model.DLQMessage) dlqservice.ReplayResult {
	message, err := decodeDLQMessage(record)
	if err != nil {
		return dlqservice.ReplayResult{Success: false, Retryable: false, FinalError: err}
	}
	if KafkaChatServer == nil {
		nextRetryAt := time.Now().Add(30 * time.Second)
		return dlqservice.ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: fmt.Errorf("kafka server unavailable")}
	}
	if err := KafkaChatServer.handleConsumedGroupMessage(nil, &message, message.SendAvatar, message.Uuid); err != nil {
		nextRetryAt := time.Now().Add(30 * time.Second)
		return dlqservice.ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: err}
	}
	return dlqservice.ReplayResult{Success: true}
}

func decodeChatMessageRequest(record *model.DLQMessage) (request.ChatMessageRequest, error) {
	var req request.ChatMessageRequest
	if err := json.Unmarshal([]byte(record.PayloadSnapshot), &req); err == nil {
		if req.MessageId == "" {
			req.MessageId = record.MessageID
		}
		return req, nil
	}
	if err := json.Unmarshal([]byte(record.RawPayload), &req); err != nil {
		return request.ChatMessageRequest{}, err
	}
	if req.MessageId == "" {
		req.MessageId = record.MessageID
	}
	return req, nil
}

func decodeDLQMessage(record *model.DLQMessage) (model.Message, error) {
	var message model.Message
	if err := json.Unmarshal([]byte(record.PayloadSnapshot), &message); err == nil {
		return message, nil
	}
	if err := json.Unmarshal([]byte(record.RawPayload), &message); err != nil {
		return model.Message{}, err
	}
	if message.Uuid == "" {
		message.Uuid = record.MessageID
	}
	return message, nil
}

func buildMessageFromRequest(req request.ChatMessageRequest, sessionSeq int64) model.Message {
	message := model.Message{
		Uuid:            req.MessageId,
		SessionId:       req.SessionId,
		Type:            req.Type,
		Content:         req.Content,
		Url:             req.Url,
		SendId:          req.SendId,
		SendName:        req.SendName,
		SendAvatar:      normalizePath(req.SendAvatar),
		ReceiveId:       req.ReceiveId,
		ConversationKey: model.BuildConversationKey(req.SendId, req.ReceiveId),
		FileSize:        req.FileSize,
		FileType:        req.FileType,
		FileName:        req.FileName,
		Status:          message_status_enum.Unsent,
		SessionSeq:      sessionSeq,
		CreatedAt:       time.Now(),
		AVdata:          req.AVdata,
	}
	switch req.Type {
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
	}
	return message
}

func persistAndDispatchMessage(message model.Message, sendAvatar string, messageBack bool) error {
	messageBackUUID := ""
	if messageBack {
		messageBackUUID = message.Uuid
	}
	return dispatchPersistedMessage(&message, sendAvatar, messageBackUUID)
}

func dispatchPersistedMessage(message *model.Message, sendAvatar string, messageBackUUID string) error {
	if message == nil {
		return fmt.Errorf("message is nil")
	}
	switch {
	case message.ReceiveId == "":
		return fmt.Errorf("receive_id is empty")
	case message.ReceiveId[0] == 'U':
		return dispatchUserMessage(message, sendAvatar, messageBackUUID)
	case message.ReceiveId[0] == 'G':
		if message.Type == message_type_enum.AudioOrVideo {
			return fmt.Errorf("group audio/video is not supported")
		}
		if KafkaChatServer == nil {
			return fmt.Errorf("kafka server unavailable")
		}
		return KafkaChatServer.handleConsumedGroupMessage(nil, message, sendAvatar, messageBackUUID)
	default:
		return fmt.Errorf("unsupported receive_id: %s", message.ReceiveId)
	}
}

func dispatchUserMessage(message *model.Message, sendAvatar string, messageBackUUID string) error {
	if KafkaChatServer == nil {
		return fmt.Errorf("kafka server unavailable")
	}
	switch message.Type {
	case message_type_enum.Text, message_type_enum.File:
		messageRsp := respond.GetMessageListRespond{
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
		payload, err := json.Marshal(messageRsp)
		if err != nil {
			return err
		}
		KafkaChatServer.dispatchToKafkaRecipients([]string{message.ReceiveId, message.SendId}, payload, messageBackUUID)
		return nil
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
		payload, err := json.Marshal(messageRsp)
		if err != nil {
			return err
		}
		KafkaChatServer.dispatchToKafkaRecipients([]string{message.ReceiveId}, payload, messageBackUUID)
		return nil
	default:
		return fmt.Errorf("unsupported message type: %d", message.Type)
	}
}

func isGroupReceiveID(receiveID string) bool {
	return strings.HasPrefix(receiveID, "G")
}
