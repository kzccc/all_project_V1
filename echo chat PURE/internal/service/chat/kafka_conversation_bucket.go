package chat

import (
	"container/list"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/IBM/sarama"

	"echo_chat_server/internal/config"
	"echo_chat_server/pkg/zlog"

	"go.uber.org/zap"
)

const (
	conversationBucketMaxBuckets         = 100000
	conversationBucketIdleTTL            = 5 * time.Minute
	conversationBucketGCInterval         = 1 * time.Minute
	conversationBucketMaxGCPerRun        = 1024
	conversationBucketPauseThreshold     = 20000
	conversationBucketResumeThreshold    = 12000
	conversationBucketMaxQueueDepth      = 2048
	conversationBucketCompleteDrainBatch = 64
)

type conversationBucketJob struct {
	kafkaMessage  *sarama.ConsumerMessage
	decoded       decodedConsumedMessage
	readyEnqueued bool
	readyDequeued bool
}

type conversationBucketState struct {
	key          string
	mu           sync.Mutex
	queue        []*conversationBucketJob
	running      bool
	queued       bool
	lastActiveAt int64
	lruElem      *list.Element
}

type conversationBucketProcessResult struct {
	job       *conversationBucketJob
	err       error
	markable  bool
	retryable bool
}

type conversationBucketClaimRunner struct {
	server     *KafkaServer
	session    sarama.ConsumerGroupSession
	claim      sarama.ConsumerGroupClaim
	readyCh    chan *conversationBucketState
	completeCh chan *conversationBucketProcessResult
	stopCh     chan struct{}

	workerCount        int
	maxMessagesPerTurn int
	maxRunDuration     time.Duration
	drainTimeout       time.Duration

	commitBatchSize int
	commitInterval  time.Duration

	wg                sync.WaitGroup
	buckets           map[string]*conversationBucketState
	bucketsMu         sync.Mutex
	lruMu             sync.Mutex
	stateMu           sync.Mutex
	lruList           *list.List
	maxBuckets        int
	bucketIdleTTL     time.Duration
	gcInterval        time.Duration
	maxGCPerRun       int
	totalBucketQueued int
	pauseThreshold    int
	resumeThreshold   int
	maxBucketDepth    int
	paused            bool
	pendingMessage    *decodedConsumedMessage

	nextMarkOffset     int64
	inflight           int
	completed          map[int64]*conversationBucketProcessResult
	completeDrainBatch int
}

func (k *KafkaServer) consumeClaimConversationBucket(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	return newConversationBucketClaimRunner(k, session, claim).run()
}

func newConversationBucketClaimRunner(server *KafkaServer, session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) *conversationBucketClaimRunner {
	kafkaCfg := config.GetConfig().KafkaConfig
	completeChSize := kafkaCfg.ResolveConversationBucketReadyQueueSize() * 8
	workerSizedCompleteCh := kafkaCfg.ResolveConversationBucketWorkerCount() * 16
	if completeChSize < workerSizedCompleteCh {
		completeChSize = workerSizedCompleteCh
	}
	return &conversationBucketClaimRunner{
		server:             server,
		session:            session,
		claim:              claim,
		readyCh:            make(chan *conversationBucketState, kafkaCfg.ResolveConversationBucketReadyQueueSize()),
		completeCh:         make(chan *conversationBucketProcessResult, completeChSize),
		stopCh:             make(chan struct{}),
		workerCount:        kafkaCfg.ResolveConversationBucketWorkerCount(),
		maxMessagesPerTurn: kafkaCfg.ResolveConversationBucketMaxMessagesPerTurn(),
		maxRunDuration:     kafkaCfg.ResolveConversationBucketMaxRunDuration(),
		drainTimeout:       kafkaCfg.ResolveConversationBucketDrainTimeout(),
		commitBatchSize:    kafkaCfg.ResolveConsumerCommitBatchSize(),
		commitInterval:     kafkaCfg.ResolveConsumerCommitInterval(),
		buckets:            make(map[string]*conversationBucketState),
		lruList:            list.New(),
		maxBuckets:         conversationBucketMaxBuckets,
		bucketIdleTTL:      conversationBucketIdleTTL,
		gcInterval:         conversationBucketGCInterval,
		maxGCPerRun:        conversationBucketMaxGCPerRun,
		pauseThreshold:     conversationBucketPauseThreshold,
		resumeThreshold:    conversationBucketResumeThreshold,
		maxBucketDepth:     conversationBucketMaxQueueDepth,
		completed:          make(map[int64]*conversationBucketProcessResult),
		nextMarkOffset:     -1,
		completeDrainBatch: conversationBucketCompleteDrainBatch,
	}
}

func (r *conversationBucketClaimRunner) run() error {
	if r.workerCount < 1 {
		r.workerCount = 1
	}
	if r.maxMessagesPerTurn < 1 {
		r.maxMessagesPerTurn = 1
	}
	if r.maxRunDuration <= 0 {
		r.maxRunDuration = 5 * time.Millisecond
	}
	if r.drainTimeout <= 0 {
		r.drainTimeout = 3 * time.Second
	}

	for i := 0; i < r.workerCount; i++ {
		r.wg.Add(1)
		go r.runWorker(i)
	}
	r.wg.Add(1)
	go r.runBucketGC()
	defer func() {
		close(r.stopCh)
		r.wg.Wait()
	}()

	commitTimer := time.NewTimer(r.commitInterval)
	if !commitTimer.Stop() {
		select {
		case <-commitTimer.C:
		default:
		}
	}
	defer commitTimer.Stop()

	pendingCommitCount := 0
	var commitTimerC <-chan time.Time
	stopCommitTimer := func() {
		if commitTimerC == nil {
			return
		}
		if !commitTimer.Stop() {
			select {
			case <-commitTimer.C:
			default:
			}
		}
		commitTimerC = nil
	}
	armCommitTimer := func() {
		if pendingCommitCount == 0 {
			stopCommitTimer()
			return
		}
		stopCommitTimer()
		commitTimer.Reset(r.commitInterval)
		commitTimerC = commitTimer.C
	}
	flushPending := func() {
		if pendingCommitCount == 0 {
			return
		}
		stopCommitTimer()
		r.session.Commit()
		pendingCommitCount = 0
	}
	defer flushPending()

	for {
		if r.drainCompleteBatch(&pendingCommitCount, flushPending, armCommitTimer) > 0 {
			continue
		}

		if r.shouldPauseConsume() {
			select {
			case <-r.server.done:
				r.drainInflight(&pendingCommitCount, flushPending, armCommitTimer)
				return nil
			case <-commitTimerC:
				flushPending()
			case result := <-r.completeCh:
				r.handleComplete(result, &pendingCommitCount, flushPending, armCommitTimer)
			case <-time.After(2 * time.Millisecond):
			}
			continue
		}

		if r.hasPendingMessage() {
			if !r.tryEnqueuePending() {
				select {
				case <-r.server.done:
					r.drainInflight(&pendingCommitCount, flushPending, armCommitTimer)
					return nil
				case <-commitTimerC:
					flushPending()
				case result := <-r.completeCh:
					r.handleComplete(result, &pendingCommitCount, flushPending, armCommitTimer)
				case <-time.After(2 * time.Millisecond):
				}
				continue
			}
			continue
		}

		select {
		case <-r.server.done:
			r.drainInflight(&pendingCommitCount, flushPending, armCommitTimer)
			return nil
		case <-commitTimerC:
			flushPending()
		case result := <-r.completeCh:
			r.handleComplete(result, &pendingCommitCount, flushPending, armCommitTimer)
		case kafkaMessage, ok := <-r.claim.Messages():
			if !ok {
				r.drainInflight(&pendingCommitCount, flushPending, armCommitTimer)
				return nil
			}
			if kafkaMessage == nil {
				continue
			}
			lag := r.claim.HighWaterMarkOffset() - kafkaMessage.Offset - 1
			if lag < 0 {
				lag = 0
			}
			decoded, err := decodeConsumedMessage(kafkaMessage)
			if err != nil {
				_, retryable := consumerErrorMeta(err)
				if retryable {
					continue
				}
				r.session.MarkMessage(kafkaMessage, "")
				pendingCommitCount++
				if pendingCommitCount >= r.commitBatchSize {
					flushPending()
					continue
				}
				armCommitTimer()
				continue
			}
			if r.nextMarkOffset < 0 {
				r.nextMarkOffset = kafkaMessage.Offset
			}
			r.incrementInflight()
			if !r.enqueue(decoded) {
				r.setPendingMessage(&decoded)
			}
		}
	}
}

func (r *conversationBucketClaimRunner) drainCompleteBatch(pendingCommitCount *int, flushPending func(), armCommitTimer func()) int {
	if r.completeDrainBatch <= 0 {
		r.completeDrainBatch = conversationBucketCompleteDrainBatch
	}
	drained := 0
	for drained < r.completeDrainBatch {
		select {
		case result := <-r.completeCh:
			r.handleComplete(result, pendingCommitCount, flushPending, armCommitTimer)
			drained++
		default:
			return drained
		}
	}
	return drained
}

func (r *conversationBucketClaimRunner) drainInflight(pendingCommitCount *int, flushPending func(), armCommitTimer func()) {
	deadline := time.NewTimer(r.drainTimeout)
	defer deadline.Stop()
	for r.currentInflight() > 0 {
		select {
		case result := <-r.completeCh:
			r.handleComplete(result, pendingCommitCount, flushPending, armCommitTimer)
		case <-deadline.C:
			zlog.Error(
				"kafka.consume.conversation_bucket_drain_timeout",
				zap.String("event", "kafka.consume.conversation_bucket_drain_timeout"),
				zap.String("module", "chat.kafka"),
				zap.String("topic", r.claim.Topic()),
				zap.Int32("partition", r.claim.Partition()),
				zap.Int("inflight", r.currentInflight()),
			)
			return
		}
	}
}

func (r *conversationBucketClaimRunner) handleComplete(result *conversationBucketProcessResult, pendingCommitCount *int, flushPending func(), armCommitTimer func()) {
	if result == nil || result.job == nil || result.job.kafkaMessage == nil {
		return
	}
	r.decrementInflight()
	r.completed[result.job.kafkaMessage.Offset] = result
	for {
		next, ok := r.completed[r.nextMarkOffset]
		if !ok {
			break
		}
		delete(r.completed, r.nextMarkOffset)
		if next.markable {
			r.session.MarkMessage(next.job.kafkaMessage, "")
			*pendingCommitCount = *pendingCommitCount + 1
			if *pendingCommitCount >= r.commitBatchSize {
				flushPending()
			} else {
				armCommitTimer()
			}
		}
		r.nextMarkOffset++
	}
}

func (r *conversationBucketClaimRunner) enqueue(decoded decodedConsumedMessage) bool {
	job := &conversationBucketJob{
		kafkaMessage: decoded.kafkaMessage,
		decoded:      decoded,
	}

	bucket := r.getOrCreateBucket(decoded.conversationKey)
	bucket.mu.Lock()
	if len(bucket.queue) >= r.maxBucketDepth {
		bucket.mu.Unlock()
		r.observeBucketGCSkipped("bucket_depth_limit")
		return false
	}
	bucket.queue = append(bucket.queue, job)
	bucket.mu.Unlock()
	r.incrementQueued()
	r.enqueueBucketReady(bucket)
	return true
}

func (r *conversationBucketClaimRunner) getOrCreateBucket(conversationKey string) *conversationBucketState {
	now := time.Now().UnixNano()
	r.bucketsMu.Lock()
	if bucket, ok := r.buckets[conversationKey]; ok {
		r.lruMu.Lock()
		bucket.mu.Lock()
		r.touchBucketLocked(bucket, now)
		bucket.mu.Unlock()
		r.lruMu.Unlock()
		r.bucketsMu.Unlock()
		return bucket
	}
	bucket := &conversationBucketState{
		key:          conversationKey,
		queue:        make([]*conversationBucketJob, 0, config.GetConfig().KafkaConfig.ResolveConversationBucketQueueSize()),
		lastActiveAt: now,
	}
	r.lruMu.Lock()
	bucket.mu.Lock()
	r.attachNewBucketLocked(bucket, now)
	bucket.mu.Unlock()
	total := len(r.buckets)
	lruTotal := r.lruList.Len()
	r.lruMu.Unlock()
	r.bucketsMu.Unlock()
	r.observeBucketTotalConsistency("create", total, lruTotal)
	r.evictBucketsUntilWithinLimit()
	return bucket
}

func (r *conversationBucketClaimRunner) enqueueBucketReady(bucket *conversationBucketState) {
	bucket.mu.Lock()
	if bucket.running || bucket.queued || len(bucket.queue) == 0 {
		bucket.mu.Unlock()
		return
	}
	bucket.queued = true
	for _, job := range bucket.queue {
		if job.readyEnqueued {
			continue
		}
		job.readyEnqueued = true
	}
	bucket.mu.Unlock()

	select {
	case <-r.stopCh:
		return
	case r.readyCh <- bucket:
	}
}

func (r *conversationBucketClaimRunner) runWorker(workerIndex int) {
	defer r.wg.Done()
	for {
		select {
		case <-r.stopCh:
			return
		case bucket := <-r.readyCh:
			if bucket == nil {
				continue
			}
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						r.logWorkerPanic(workerIndex, nil, recovered)
					}
				}()
				r.runBucketTurn(workerIndex, bucket)
			}()
		}
	}
}

func (r *conversationBucketClaimRunner) runBucketTurn(workerIndex int, bucket *conversationBucketState) {
	start := time.Now()
	processed := 0

	bucket.mu.Lock()
	bucket.queued = false
	bucket.running = true
	bucket.mu.Unlock()

	for processed < r.maxMessagesPerTurn && time.Since(start) < r.maxRunDuration {
		bucket.mu.Lock()
		if len(bucket.queue) == 0 {
			bucket.running = false
			bucket.mu.Unlock()
			return
		}
		job := bucket.queue[0]
		bucket.queue = bucket.queue[1:]
		bucket.mu.Unlock()
		r.decrementQueued()

		if !job.readyDequeued {
			job.readyDequeued = true
		}
		result := r.safeProcessJob(workerIndex, job)
		select {
		case <-r.stopCh:
			return
		case r.completeCh <- result:
		}
		processed++
	}

	bucket.mu.Lock()
	hasMore := len(bucket.queue) > 0
	bucket.running = false
	bucket.mu.Unlock()
	if hasMore {
		r.enqueueBucketReady(bucket)
	}
}

func (r *conversationBucketClaimRunner) safeProcessJob(workerIndex int, job *conversationBucketJob) (result *conversationBucketProcessResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.logWorkerPanic(workerIndex, job, recovered)
			result = &conversationBucketProcessResult{
				job:       job,
				err:       fmt.Errorf("conversation bucket panic: %v", recovered),
				markable:  true,
				retryable: false,
			}
		}
	}()
	return r.processJob(workerIndex, job)
}

func (r *conversationBucketClaimRunner) logWorkerPanic(workerIndex int, job *conversationBucketJob, recovered interface{}) {
	fields := []zap.Field{
		zap.String("event", "kafka.consume.conversation_bucket_worker_panic"),
		zap.String("module", "chat.kafka"),
		zap.String("topic", r.claim.Topic()),
		zap.Int32("partition", r.claim.Partition()),
		zap.Int("worker_index", workerIndex),
		zap.Any("panic", recovered),
		zap.ByteString("stack", debug.Stack()),
	}
	if job != nil {
		fields = append(fields,
			zap.String("conversation_key", job.decoded.conversationKey),
			zap.Int64("offset", job.kafkaMessage.Offset),
			zap.String("message_id", job.decoded.messageID),
		)
	}
	zlog.Error("kafka.consume.conversation_bucket_worker_panic", fields...)
}

func (r *conversationBucketClaimRunner) totalBacklog() int {
	return r.currentStateBacklog()
}

func (r *conversationBucketClaimRunner) updatePauseStateLocked(backlog int) {
	wasPaused := r.paused
	if !r.paused && backlog >= r.pauseThreshold {
		r.paused = true
	}
	if r.paused && backlog <= r.resumeThreshold {
		r.paused = false
	}
	if wasPaused == r.paused {
		return
	}
	zlog.Info(
		"kafka.consume.conversation_bucket_backpressure_state",
		zap.String("event", "kafka.consume.conversation_bucket_backpressure_state"),
		zap.String("module", "chat.kafka"),
		zap.String("topic", r.claim.Topic()),
		zap.Int32("partition", r.claim.Partition()),
		zap.Bool("paused", r.paused),
		zap.Int("backlog", backlog),
		zap.Int("pause_threshold", r.pauseThreshold),
		zap.Int("resume_threshold", r.resumeThreshold),
	)
}

func (r *conversationBucketClaimRunner) shouldPauseConsume() bool {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	return r.paused
}

func (r *conversationBucketClaimRunner) tryEnqueuePending() bool {
	decoded, ok := r.getPendingMessage()
	if !ok {
		return true
	}
	if !r.enqueue(decoded) {
		return false
	}
	r.clearPendingMessage()
	return true
}

func (r *conversationBucketClaimRunner) incrementInflight() {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.inflight++
	r.updatePauseStateLocked(r.inflight + r.totalBucketQueued)
}

func (r *conversationBucketClaimRunner) decrementInflight() {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if r.inflight > 0 {
		r.inflight--
	}
	r.updatePauseStateLocked(r.inflight + r.totalBucketQueued)
}

func (r *conversationBucketClaimRunner) incrementQueued() {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.totalBucketQueued++
	r.updatePauseStateLocked(r.inflight + r.totalBucketQueued)
}

func (r *conversationBucketClaimRunner) decrementQueued() {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if r.totalBucketQueued > 0 {
		r.totalBucketQueued--
	}
	r.updatePauseStateLocked(r.inflight + r.totalBucketQueued)
}

func (r *conversationBucketClaimRunner) currentInflight() int {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	return r.inflight
}

func (r *conversationBucketClaimRunner) currentStateBacklog() int {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	return r.inflight + r.totalBucketQueued
}

func (r *conversationBucketClaimRunner) hasPendingMessage() bool {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	return r.pendingMessage != nil
}

func (r *conversationBucketClaimRunner) getPendingMessage() (decodedConsumedMessage, bool) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if r.pendingMessage == nil {
		return decodedConsumedMessage{}, false
	}
	return *r.pendingMessage, true
}

func (r *conversationBucketClaimRunner) setPendingMessage(decoded *decodedConsumedMessage) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.pendingMessage = decoded
}

func (r *conversationBucketClaimRunner) clearPendingMessage() {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.pendingMessage = nil
}

func (r *conversationBucketClaimRunner) runBucketGC() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.gcInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.observeBucketGCScan()
			r.evictBucketsUntilWithinLimit()
			r.gcEvictIdleBuckets()
		}
	}
}

func (r *conversationBucketClaimRunner) touchBucketLocked(bucket *conversationBucketState, now int64) {
	bucket.lastActiveAt = now
	if bucket.lruElem != nil {
		r.lruList.MoveToFront(bucket.lruElem)
		return
	}
	bucket.lruElem = r.lruList.PushFront(bucket)
}

func (r *conversationBucketClaimRunner) attachNewBucketLocked(bucket *conversationBucketState, now int64) {
	bucket.lastActiveAt = now
	bucket.lruElem = r.lruList.PushFront(bucket)
	r.buckets[bucket.key] = bucket
}

func (r *conversationBucketClaimRunner) removeBucketLocked(bucket *conversationBucketState) {
	delete(r.buckets, bucket.key)
	if bucket.lruElem != nil {
		r.lruList.Remove(bucket.lruElem)
		bucket.lruElem = nil
	}
}

func (r *conversationBucketClaimRunner) isBucketEvictableLocked(bucket *conversationBucketState, now int64) bool {
	if bucket == nil {
		return false
	}
	if bucket.running || bucket.queued || len(bucket.queue) > 0 {
		return false
	}
	if now-bucket.lastActiveAt <= r.bucketIdleTTL.Nanoseconds() {
		return false
	}
	return true
}

func (r *conversationBucketClaimRunner) tryEvictOneFromTail(now int64) bool {
	r.bucketsMu.Lock()
	defer r.bucketsMu.Unlock()
	r.lruMu.Lock()
	defer r.lruMu.Unlock()
	for e := r.lruList.Back(); e != nil; e = e.Prev() {
		bucket, ok := e.Value.(*conversationBucketState)
		if !ok || bucket == nil {
			continue
		}
		bucket.mu.Lock()
		evictable := r.isBucketEvictableLocked(bucket, now)
		bucket.mu.Unlock()
		if !evictable {
			r.observeBucketGCSkipped("not_evictable")
			continue
		}
		if current, ok := r.buckets[bucket.key]; !ok || current != bucket {
			continue
		}
		r.removeBucketLocked(bucket)
		total := len(r.buckets)
		lruTotal := r.lruList.Len()
		r.observeBucketLRUEvict(total, lruTotal)
		return true
	}
	return false
}

func (r *conversationBucketClaimRunner) evictBucketsUntilWithinLimit() {
	for {
		total, _ := r.bucketCountsSnapshot()
		if total <= r.maxBuckets {
			r.observeBucketTotalConsistency("limit", total, -1)
			return
		}
		if !r.tryEvictOneFromTail(time.Now().UnixNano()) {
			r.observeBucketTotalConsistency("limit", total, -1)
			return
		}
	}
}

func (r *conversationBucketClaimRunner) gcEvictIdleBuckets() {
	now := time.Now().UnixNano()
	evicted := 0
	for evicted < r.maxGCPerRun {
		if !r.tryEvictOneFromTail(now) {
			r.observeBucketTotalConsistency("gc", -1, -1)
			return
		}
		evicted++
	}
	r.observeBucketTotalConsistency("gc", -1, -1)
}

func (r *conversationBucketClaimRunner) bucketCountsSnapshot() (int, int) {
	r.bucketsMu.Lock()
	defer r.bucketsMu.Unlock()
	r.lruMu.Lock()
	defer r.lruMu.Unlock()
	return len(r.buckets), r.lruList.Len()
}

func (r *conversationBucketClaimRunner) observeBucketTotalConsistency(stage string, total int, lruTotal int) {
	if total < 0 || lruTotal < 0 {
		total, lruTotal = r.bucketCountsSnapshot()
	}
	if total != lruTotal {
		zlog.Error(
			"kafka.consume.conversation_bucket_total_inconsistent",
			zap.String("event", "kafka.consume.conversation_bucket_total_inconsistent"),
			zap.String("module", "chat.kafka"),
			zap.String("topic", r.claim.Topic()),
			zap.Int32("partition", r.claim.Partition()),
			zap.String("stage", stage),
			zap.Int("bucket_total", total),
			zap.Int("lru_total", lruTotal),
		)
		r.observeBucketGCSkipped("inconsistent_total")
		return
	}
}

func (r *conversationBucketClaimRunner) observeBucketLRUEvict(total int, lruTotal int) {
	r.observeBucketTotalConsistency("lru_evict", total, lruTotal)
}

func (r *conversationBucketClaimRunner) observeBucketGCScan() {}

func (r *conversationBucketClaimRunner) observeBucketGCSkipped(reason string) {}

func (r *conversationBucketClaimRunner) processJob(workerIndex int, job *conversationBucketJob) *conversationBucketProcessResult {
	_ = workerIndex
	for {
		totalStart := time.Now()
		_ = totalStart
		err := handleDecodedConsumedMessage(r.server, job.decoded)
		if err == nil {
			return &conversationBucketProcessResult{job: job, markable: true}
		}

		_, retryable := consumerErrorMeta(err)
		if !retryable {
			return &conversationBucketProcessResult{job: job, err: err, markable: true, retryable: false}
		}
		select {
		case <-r.stopCh:
			return &conversationBucketProcessResult{job: job, err: err, markable: false, retryable: true}
		case <-time.After(200 * time.Millisecond):
		}
	}
}
