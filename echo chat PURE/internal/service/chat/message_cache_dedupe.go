package chat

import (
	"fmt"
	"echo_chat_server/internal/dto/respond"
)

func cacheMessageIdentity(messageID string, sessionSeq int64) string {
	if messageID != "" {
		return "message_id:" + messageID
	}
	if sessionSeq > 0 {
		return fmt.Sprintf("session_seq:%d", sessionSeq)
	}
	return ""
}

func appendUniqueUserCacheMessage(cacheKey string, list []respond.GetMessageListRespond, message respond.GetMessageListRespond, mode string) ([]respond.GetMessageListRespond, bool) {
	messageIdentity := cacheMessageIdentity(message.MessageId, message.SessionSeq)
	if messageIdentity == "" {
		return append(list, message), false
	}
	for _, cachedMessage := range list {
		if cacheMessageIdentity(cachedMessage.MessageId, cachedMessage.SessionSeq) == messageIdentity {
			logMessageCacheDuplicateSkip(mode, cacheKey, message.MessageId, message.SessionSeq)
			return list, true
		}
	}
	return append(list, message), false
}

func appendUniqueGroupCacheMessage(cacheKey string, list []respond.GetGroupMessageListRespond, message respond.GetGroupMessageListRespond, mode string) ([]respond.GetGroupMessageListRespond, bool) {
	messageIdentity := cacheMessageIdentity(message.MessageId, message.SessionSeq)
	if messageIdentity == "" {
		return append(list, message), false
	}
	for _, cachedMessage := range list {
		if cacheMessageIdentity(cachedMessage.MessageId, cachedMessage.SessionSeq) == messageIdentity {
			logMessageCacheDuplicateSkip(mode, cacheKey, message.MessageId, message.SessionSeq)
			return list, true
		}
	}
	return append(list, message), false
}
