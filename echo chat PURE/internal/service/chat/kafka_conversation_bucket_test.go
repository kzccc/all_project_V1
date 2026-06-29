package chat

import (
	"container/list"
	"testing"
	"time"
)

func newTestConversationBucketRunner() *conversationBucketClaimRunner {
	return &conversationBucketClaimRunner{
		buckets:       make(map[string]*conversationBucketState),
		lruList:       list.New(),
		maxBuckets:    2,
		bucketIdleTTL: time.Second,
		gcInterval:    time.Second,
		maxGCPerRun:   8,
	}
}

func TestConversationBucketGetOrCreateTracksLRU(t *testing.T) {
	r := newTestConversationBucketRunner()

	b1 := r.getOrCreateBucket("c1")
	if b1 == nil {
		t.Fatalf("expected bucket")
	}
	if len(r.buckets) != 1 {
		t.Fatalf("bucket count = %d, want 1", len(r.buckets))
	}
	if r.lruList.Len() != 1 {
		t.Fatalf("lru len = %d, want 1", r.lruList.Len())
	}
	if r.lruList.Front() == nil || r.lruList.Front().Value.(*conversationBucketState) != b1 {
		t.Fatalf("expected c1 at LRU front")
	}

	b2 := r.getOrCreateBucket("c2")
	if b2 == nil {
		t.Fatalf("expected second bucket")
	}
	if r.lruList.Front() == nil || r.lruList.Front().Value.(*conversationBucketState) != b2 {
		t.Fatalf("expected c2 at LRU front after create")
	}

	b1Again := r.getOrCreateBucket("c1")
	if b1Again != b1 {
		t.Fatalf("expected existing bucket reuse")
	}
	if r.lruList.Front() == nil || r.lruList.Front().Value.(*conversationBucketState) != b1 {
		t.Fatalf("expected c1 moved to LRU front after hit")
	}
}

func TestConversationBucketEvictsIdleTailWhenOverLimit(t *testing.T) {
	r := newTestConversationBucketRunner()
	now := time.Now().Add(-2 * time.Second).UnixNano()

	b1 := &conversationBucketState{key: "c1", lastActiveAt: now}
	b2 := &conversationBucketState{key: "c2", lastActiveAt: now}
	r.buckets["c1"] = b1
	r.buckets["c2"] = b2
	b1.lruElem = r.lruList.PushBack(b1)
	b2.lruElem = r.lruList.PushFront(b2)

	if !r.tryEvictOneFromTail(time.Now().UnixNano()) {
		t.Fatalf("expected eviction to succeed")
	}
	if _, ok := r.buckets["c1"]; ok {
		t.Fatalf("expected oldest idle bucket to be evicted")
	}
	if r.lruList.Len() != 1 {
		t.Fatalf("lru len = %d, want 1", r.lruList.Len())
	}
}

func TestConversationBucketDoesNotEvictActiveBucket(t *testing.T) {
	r := newTestConversationBucketRunner()
	now := time.Now().Add(-2 * time.Second).UnixNano()

	b := &conversationBucketState{
		key:          "c1",
		lastActiveAt: now,
		running:      true,
	}
	r.buckets["c1"] = b
	b.lruElem = r.lruList.PushBack(b)

	if r.tryEvictOneFromTail(time.Now().UnixNano()) {
		t.Fatalf("expected active bucket not to be evicted")
	}
	if _, ok := r.buckets["c1"]; !ok {
		t.Fatalf("bucket should remain")
	}
}
