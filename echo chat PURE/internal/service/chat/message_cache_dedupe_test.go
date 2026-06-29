package chat

import (
	"echo_chat_server/internal/dto/respond"
	"testing"
)

func TestAppendUniqueUserCacheMessageByMessageID(t *testing.T) {
	cacheKey := "message_list_U1_U2"
	list := []respond.GetMessageListRespond{
		{MessageId: "M1", SessionSeq: 1},
	}

	nextList, skipped := appendUniqueUserCacheMessage(cacheKey, list, respond.GetMessageListRespond{
		MessageId:  "M1",
		SessionSeq: 2,
	}, "channel")

	if !skipped {
		t.Fatalf("expected duplicate to be skipped")
	}
	if len(nextList) != 1 {
		t.Fatalf("expected list size to remain 1, got %d", len(nextList))
	}
}

func TestAppendUniqueGroupCacheMessageBySessionSeqFallback(t *testing.T) {
	cacheKey := "group_messagelist_G1"
	list := []respond.GetGroupMessageListRespond{
		{SessionSeq: 7},
	}

	nextList, skipped := appendUniqueGroupCacheMessage(cacheKey, list, respond.GetGroupMessageListRespond{
		SessionSeq: 7,
	}, "kafka")

	if !skipped {
		t.Fatalf("expected duplicate session_seq to be skipped")
	}
	if len(nextList) != 1 {
		t.Fatalf("expected list size to remain 1, got %d", len(nextList))
	}
}

func TestAppendUniqueUserCacheMessageAppendsWhenNew(t *testing.T) {
	cacheKey := "message_list_U1_U2"
	list := []respond.GetMessageListRespond{
		{MessageId: "M1", SessionSeq: 1},
	}

	nextList, skipped := appendUniqueUserCacheMessage(cacheKey, list, respond.GetMessageListRespond{
		MessageId:  "M2",
		SessionSeq: 2,
	}, "channel")

	if skipped {
		t.Fatalf("expected new message to be appended")
	}
	if len(nextList) != 2 {
		t.Fatalf("expected list size 2, got %d", len(nextList))
	}
}
