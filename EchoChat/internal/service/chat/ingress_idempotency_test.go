package chat

import (
	"testing"

	myredis "echo_chat_server/internal/service/redis"
)

func TestBuildIngressIdempotencyKey(t *testing.T) {
	got := buildIngressIdempotencyKey("U1001", "M2002")
	want := "ingress_idempotency:U1001:M2002"
	if got != want {
		t.Fatalf("buildIngressIdempotencyKey = %q, want %q", got, want)
	}
}

func TestShouldShortCircuitIngressDuplicatePending(t *testing.T) {
	client := &Client{
		SendBack: make(chan *MessageBack, 1),
	}

	shortCircuited := shouldShortCircuitIngressDuplicate(client, "M1001", myredis.IngressIdempotencyRecord{
		Status: myredis.IngressIdempotencyStatusPending,
	})
	if !shortCircuited {
		t.Fatalf("expected pending duplicate to short circuit")
	}
	if len(client.SendBack) != 0 {
		t.Fatalf("pending duplicate should not enqueue replay payload")
	}
}

func TestShouldShortCircuitIngressDuplicateDoneReplaysPayload(t *testing.T) {
	client := &Client{
		SendBack: make(chan *MessageBack, 1),
	}
	payload := `{"message_id":"M1002","content":"hello"}`

	shortCircuited := shouldShortCircuitIngressDuplicate(client, "M1002", myredis.IngressIdempotencyRecord{
		Status:  myredis.IngressIdempotencyStatusDone,
		Payload: payload,
	})
	if !shortCircuited {
		t.Fatalf("expected done duplicate to short circuit")
	}

	select {
	case replay := <-client.SendBack:
		if replay == nil {
			t.Fatalf("expected replay payload")
		}
		if string(replay.Message) != payload {
			t.Fatalf("replay payload = %q, want %q", string(replay.Message), payload)
		}
	default:
		t.Fatalf("expected done duplicate to enqueue replay payload")
	}
}

func TestShouldShortCircuitIngressDuplicateDoneWithInvalidPayloadStillStopsChain(t *testing.T) {
	client := &Client{
		SendBack: make(chan *MessageBack, 1),
	}

	shortCircuited := shouldShortCircuitIngressDuplicate(client, "M1003", myredis.IngressIdempotencyRecord{
		Status:  myredis.IngressIdempotencyStatusDone,
		Payload: "not-json",
	})
	if !shortCircuited {
		t.Fatalf("expected invalid done payload to still short circuit")
	}
	if len(client.SendBack) != 0 {
		t.Fatalf("invalid payload should not enqueue replay payload")
	}
}

func TestReplayIngressDoneRecordRejectsNonDonePayload(t *testing.T) {
	client := &Client{
		SendBack: make(chan *MessageBack, 1),
	}

	replayed := replayIngressDoneRecord(client, myredis.IngressIdempotencyRecord{
		Status:  myredis.IngressIdempotencyStatusPending,
		Payload: `{"message_id":"M1004"}`,
	})
	if replayed {
		t.Fatalf("expected non-done record to skip replay")
	}
	if len(client.SendBack) != 0 {
		t.Fatalf("non-done record should not enqueue replay payload")
	}
}
