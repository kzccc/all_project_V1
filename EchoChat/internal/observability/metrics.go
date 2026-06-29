package observability

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/pressure"
)

var (
	wsHandshakeTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_ws_handshake_total",
			Help: "Total number of websocket handshake attempts by route.",
		},
		[]string{"route"},
	)
	wsHandshakeResultTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_ws_handshake_result_total",
			Help: "Total number of websocket handshake results by route and result.",
		},
		[]string{"route", "result"},
	)
	wsOnlineConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "echochat_ws_online_connections",
			Help: "Current number of online websocket connections by route.",
		},
		[]string{"route"},
	)
	authRejectTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_auth_reject_total",
			Help: "Total number of auth rejections by route and reason.",
		},
		[]string{"route", "reason"},
	)
	wsCloseTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_ws_close_total",
			Help: "Total number of websocket close events by route, source and reason.",
		},
		[]string{"route", "source", "reason"},
	)
	wsSendBackQueueLength = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_ws_sendback_queue_length",
			Help:    "Observed websocket SendBack queue length before enqueue by route and result.",
			Buckets: []float64{0, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000},
		},
		[]string{"route", "result"},
	)
	wsSendBackEnqueueDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_ws_sendback_enqueue_duration_seconds",
			Help:    "Time spent enqueueing websocket SendBack messages by route and result.",
			Buckets: []float64{0.000001, 0.000005, 0.00001, 0.00005, 0.0001, 0.0005, 0.001, 0.003, 0.01, 0.03, 0.1, 0.3, 1},
		},
		[]string{"route", "result"},
	)
	wsWriteDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_ws_write_duration_seconds",
			Help:    "Websocket write duration by route and result.",
			Buckets: []float64{0.0001, 0.0003, 0.0005, 0.001, 0.003, 0.005, 0.01, 0.03, 0.05, 0.1, 0.3, 1, 3},
		},
		[]string{"route", "result"},
	)
	wsStatusUpdateDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_ws_status_update_duration_seconds",
			Help:    "Duration of message status updates after websocket writes by route and result.",
			Buckets: []float64{0.0001, 0.0003, 0.0005, 0.001, 0.003, 0.005, 0.01, 0.03, 0.05, 0.1, 0.3, 1, 3},
		},
		[]string{"route", "result"},
	)
	kafkaProducerMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_kafka_producer_messages_total",
			Help: "Total number of Kafka produce attempts by topic and result.",
		},
		[]string{"topic", "result"},
	)
	kafkaProducerPartitionMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_kafka_producer_partition_messages_total",
			Help: "Total number of successfully produced messages by topic and partition.",
		},
		[]string{"topic", "partition"},
	)
	kafkaProducerLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_kafka_producer_latency_seconds",
			Help:    "Kafka producer send latency in seconds by topic and result.",
			Buckets: []float64{0.001, 0.003, 0.005, 0.01, 0.03, 0.05, 0.1, 0.3, 0.5, 1, 3, 5},
		},
		[]string{"topic", "result"},
	)
	kafkaConsumerMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_kafka_consumer_messages_total",
			Help: "Total number of Kafka messages pulled by consumer group, topic and partition.",
		},
		[]string{"consumer_group", "topic", "partition"},
	)
	kafkaConsumerFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_kafka_consumer_failures_total",
			Help: "Total number of Kafka consumer processing failures by stage.",
		},
		[]string{"consumer_group", "topic", "partition", "stage"},
	)
	kafkaConsumerHandledTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_kafka_consumer_handled_total",
			Help: "Total number of Kafka messages that finished a consumer handling attempt by result.",
		},
		[]string{"consumer_group", "topic", "partition", "result"},
	)
	kafkaConsumerMarkedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_kafka_consumer_marked_total",
			Help: "Total number of Kafka messages marked for offset commit after handling.",
		},
		[]string{"consumer_group", "topic", "partition"},
	)
	kafkaConsumerLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "echochat_kafka_consumer_lag",
			Help: "Current Kafka consumer lag by consumer group, topic and partition.",
		},
		[]string{"consumer_group", "topic", "partition"},
	)
	kafkaConsumerSessionReady = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "echochat_kafka_consumer_session_ready",
			Help: "Whether the Kafka consumer group session is currently ready.",
		},
		[]string{"consumer_group"},
	)
	kafkaConsumerStageDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_kafka_consumer_stage_duration_seconds",
			Help:    "Kafka consumer processing duration in seconds by stage.",
			Buckets: []float64{0.0005, 0.001, 0.003, 0.005, 0.01, 0.03, 0.05, 0.1, 0.3, 0.5, 1, 3, 5},
		},
		[]string{"consumer_group", "topic", "partition", "stage"},
	)
	kafkaOffsetCommitTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_kafka_offset_commit_total",
			Help: "Total number of Kafka offset commit attempts by consumer group, topic and partition.",
		},
		[]string{"consumer_group", "topic", "partition"},
	)
	kafkaOffsetCommitDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_kafka_offset_commit_duration_seconds",
			Help:    "Kafka offset commit duration in seconds by consumer group, topic and partition.",
			Buckets: []float64{0.001, 0.003, 0.005, 0.01, 0.03, 0.05, 0.1, 0.3, 0.5, 1, 3},
		},
		[]string{"consumer_group", "topic", "partition"},
	)
	kafkaOffsetCommitBatchSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_kafka_offset_commit_batch_size",
			Help:    "Kafka offset commit batch size by consumer group, topic and partition.",
			Buckets: []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000},
		},
		[]string{"consumer_group", "topic", "partition"},
	)
	kafkaWSDispatchEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_kafka_ws_dispatch_events_total",
			Help: "Total number of Kafka websocket dispatch path events by instance and event type.",
		},
		[]string{"instance_id", "event"},
	)
	kafkaWSDispatchDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_kafka_ws_dispatch_duration_seconds",
			Help:    "Kafka websocket dispatch duration by instance and stage.",
			Buckets: []float64{0.00001, 0.00005, 0.0001, 0.0003, 0.0005, 0.001, 0.003, 0.005, 0.01, 0.03, 0.05, 0.1, 0.3, 1, 3},
		},
		[]string{"instance_id", "stage"},
	)
	mysqlOpenConnections = promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "echochat_mysql_open_connections",
			Help: "Current number of open MySQL connections.",
		},
		func() float64 {
			return float64(mysqlDBStats().OpenConnections)
		},
	)
	mysqlInUseConnections = promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "echochat_mysql_in_use_connections",
			Help: "Current number of in-use MySQL connections.",
		},
		func() float64 {
			return float64(mysqlDBStats().InUse)
		},
	)
	mysqlIdleConnections = promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "echochat_mysql_idle_connections",
			Help: "Current number of idle MySQL connections.",
		},
		func() float64 {
			return float64(mysqlDBStats().Idle)
		},
	)
	mysqlWaitCountTotal = promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "echochat_mysql_wait_count_total",
			Help: "Total number of waits for a free MySQL connection.",
		},
		func() float64 {
			return float64(mysqlDBStats().WaitCount)
		},
	)
	mysqlWaitDurationSeconds = promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "echochat_mysql_wait_duration_seconds",
			Help: "Total time blocked waiting for a free MySQL connection.",
		},
		func() float64 {
			return mysqlDBStats().WaitDuration.Seconds()
		},
	)
	redisCommandDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_redis_command_duration_seconds",
			Help:    "Redis command duration by command and result.",
			Buckets: []float64{0.0001, 0.0003, 0.0005, 0.001, 0.003, 0.005, 0.01, 0.03, 0.05, 0.1, 0.3, 1, 3},
		},
		[]string{"cmd", "result"},
	)
	mysqlPersistQueueDepth = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "echochat_mysql_persist_queue_depth",
			Help:    "Observed mysql_persist worker queue depth before enqueue.",
			Buckets: []float64{0, 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384},
		},
	)
	mysqlPersistEnqueueBlockDurationSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "echochat_mysql_persist_enqueue_block_duration_seconds",
			Help:    "Blocking duration when enqueueing mysql_persist jobs.",
			Buckets: []float64{0.000001, 0.000005, 0.00001, 0.00005, 0.0001, 0.0005, 0.001, 0.003, 0.01, 0.03, 0.1, 0.3, 1, 3},
		},
	)
	mysqlPersistFlushBatchSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_mysql_persist_flush_batch_size",
			Help:    "Batch size of each mysql_persist flush by flush reason.",
			Buckets: []float64{1, 2, 4, 8, 16, 32, 64, 128, 192, 256, 320, 384, 512, 768, 1024},
		},
		[]string{"reason"},
	)
	mysqlPersistFlushDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "echochat_mysql_persist_flush_duration_seconds",
			Help:    "Flush duration of mysql_persist batches by flush reason and result.",
			Buckets: []float64{0.0001, 0.0003, 0.0005, 0.001, 0.003, 0.005, 0.01, 0.03, 0.05, 0.1, 0.3, 1, 3, 5},
		},
		[]string{"reason", "result"},
	)
	mysqlPersistFlushTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_mysql_persist_flush_total",
			Help: "Total number of mysql_persist flushes by reason and result.",
		},
		[]string{"reason", "result"},
	)
	conversationSeqRecoveryTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_conversation_seq_recovery_total",
			Help: "Total number of conversation session_seq recovery floor resolutions by source.",
		},
		[]string{"source"},
	)
	conversationSeqConflictTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_conversation_seq_conflict_total",
			Help: "Total number of conversation session_seq conflicts rejected by durable guards.",
		},
		[]string{"reason"},
	)
	conversationBucketTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "echochat_conversation_bucket_total",
			Help: "Current number of conversation buckets by consumer group, topic and partition.",
		},
		[]string{"consumer_group", "topic", "partition"},
	)
	conversationBucketLRUEvictTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_conversation_bucket_lru_evict_total",
			Help: "Total number of conversation buckets evicted by LRU by consumer group, topic and partition.",
		},
		[]string{"consumer_group", "topic", "partition"},
	)
	conversationBucketGCScanTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_conversation_bucket_gc_scan_total",
			Help: "Total number of conversation bucket GC scan rounds by consumer group, topic and partition.",
		},
		[]string{"consumer_group", "topic", "partition"},
	)
	conversationBucketGCSkippedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "echochat_conversation_bucket_gc_skipped_total",
			Help: "Total number of conversation bucket GC skips by consumer group, topic, partition and reason.",
		},
		[]string{"consumer_group", "topic", "partition", "reason"},
	)
)

func Init() {}

func MetricsHandler() http.Handler {
	Init()
	return promhttp.Handler()
}

func routeLabel(path string) string {
	switch {
	case pressure.IsBenchmarkPath(path):
		return "bench"
	case path == "/wss":
		return "default"
	default:
		return "other"
	}
}

func ObserveWSHandshakeAttempt(path string) {
	wsHandshakeTotal.WithLabelValues(routeLabel(path)).Inc()
}

func ObserveWSHandshakeResult(path string, success bool) {
	result := "failure"
	if success {
		result = "success"
	}
	wsHandshakeResultTotal.WithLabelValues(routeLabel(path), result).Inc()
}

func IncWSOnline(path string) {
	wsOnlineConnections.WithLabelValues(routeLabel(path)).Inc()
}

func DecWSOnline(path string) {
	wsOnlineConnections.WithLabelValues(routeLabel(path)).Dec()
}

func ObserveAuthReject(path string, reason string) {
	authRejectTotal.WithLabelValues(routeLabel(path), reason).Inc()
}

func ObserveWSClose(path string, source string, reason string) {
	wsCloseTotal.WithLabelValues(routeLabel(path), source, reason).Inc()
}

func ObserveWSSendBackEnqueue(path string, queueLength int, duration time.Duration, result string) {
	label := routeLabel(path)
	wsSendBackQueueLength.WithLabelValues(label, result).Observe(float64(queueLength))
	wsSendBackEnqueueDurationSeconds.WithLabelValues(label, result).Observe(duration.Seconds())
}

func ObserveWSWrite(path string, duration time.Duration, result string) {
	wsWriteDurationSeconds.WithLabelValues(routeLabel(path), result).Observe(duration.Seconds())
}

func ObserveWSStatusUpdate(path string, duration time.Duration, result string) {
	wsStatusUpdateDurationSeconds.WithLabelValues(routeLabel(path), result).Observe(duration.Seconds())
}

func ObserveKafkaProduce(topic string, partition int32, success bool, duration time.Duration) {
	result := "failure"
	if success {
		result = "success"
		kafkaProducerPartitionMessagesTotal.WithLabelValues(topic, strconv.FormatInt(int64(partition), 10)).Inc()
	}
	kafkaProducerMessagesTotal.WithLabelValues(topic, result).Inc()
	kafkaProducerLatencySeconds.WithLabelValues(topic, result).Observe(duration.Seconds())
}

func ObserveKafkaConsumePull(consumerGroup string, topic string, partition int32, lag int64) {
	partitionLabel := strconv.FormatInt(int64(partition), 10)
	kafkaConsumerMessagesTotal.WithLabelValues(consumerGroup, topic, partitionLabel).Inc()
	kafkaConsumerLag.WithLabelValues(consumerGroup, topic, partitionLabel).Set(float64(lag))
}

func ObserveKafkaConsumeFailure(consumerGroup string, topic string, partition int32, stage string) {
	kafkaConsumerFailuresTotal.WithLabelValues(consumerGroup, topic, strconv.FormatInt(int64(partition), 10), stage).Inc()
}

func ObserveKafkaConsumeHandled(consumerGroup string, topic string, partition int32, result string) {
	kafkaConsumerHandledTotal.WithLabelValues(consumerGroup, topic, strconv.FormatInt(int64(partition), 10), result).Inc()
}

func ObserveKafkaConsumeMarked(consumerGroup string, topic string, partition int32) {
	kafkaConsumerMarkedTotal.WithLabelValues(consumerGroup, topic, strconv.FormatInt(int64(partition), 10)).Inc()
}

func ObserveKafkaConsumerSessionReady(consumerGroup string, ready bool) {
	value := 0.0
	if ready {
		value = 1.0
	}
	kafkaConsumerSessionReady.WithLabelValues(consumerGroup).Set(value)
}

func ObserveKafkaConsumerStageDuration(consumerGroup string, topic string, partition int32, stage string, duration time.Duration) {
	kafkaConsumerStageDurationSeconds.WithLabelValues(
		consumerGroup,
		topic,
		strconv.FormatInt(int64(partition), 10),
		stage,
	).Observe(duration.Seconds())
}

func ObserveKafkaOffsetCommit(consumerGroup string, topic string, partition int32, duration time.Duration, batchSize int) {
	partitionLabel := strconv.FormatInt(int64(partition), 10)
	kafkaOffsetCommitTotal.WithLabelValues(consumerGroup, topic, partitionLabel).Inc()
	kafkaOffsetCommitDurationSeconds.WithLabelValues(consumerGroup, topic, partitionLabel).Observe(duration.Seconds())
	kafkaOffsetCommitBatchSize.WithLabelValues(consumerGroup, topic, partitionLabel).Observe(float64(batchSize))
}

func ObserveKafkaWSDispatchEvent(instanceID string, event string) {
	kafkaWSDispatchEventsTotal.WithLabelValues(instanceID, event).Inc()
}

func ObserveKafkaWSDispatchDuration(instanceID string, stage string, duration time.Duration) {
	kafkaWSDispatchDurationSeconds.WithLabelValues(instanceID, stage).Observe(duration.Seconds())
}

func ObserveRedisCommand(cmd string, duration time.Duration, result string) {
	redisCommandDurationSeconds.WithLabelValues(cmd, result).Observe(duration.Seconds())
}

func ObserveMysqlPersistEnqueue(queueDepth int, duration time.Duration) {
	mysqlPersistQueueDepth.Observe(float64(queueDepth))
	mysqlPersistEnqueueBlockDurationSeconds.Observe(duration.Seconds())
}

func ObserveMysqlPersistFlush(reason string, batchSize int, duration time.Duration, success bool) {
	result := "failure"
	if success {
		result = "success"
	}
	mysqlPersistFlushBatchSize.WithLabelValues(reason).Observe(float64(batchSize))
	mysqlPersistFlushDurationSeconds.WithLabelValues(reason, result).Observe(duration.Seconds())
	mysqlPersistFlushTotal.WithLabelValues(reason, result).Inc()
}

func ObserveConversationSeqRecovery(source string) {
	conversationSeqRecoveryTotal.WithLabelValues(source).Inc()
}

func ObserveConversationSeqConflict(reason string) {
	conversationSeqConflictTotal.WithLabelValues(reason).Inc()
}

func ObserveConversationBucketTotal(consumerGroup string, topic string, partition int32, total int) {
	partitionLabel := strconv.FormatInt(int64(partition), 10)
	conversationBucketTotal.WithLabelValues(consumerGroup, topic, partitionLabel).Set(float64(total))
}

func ObserveConversationBucketLRUEvict(consumerGroup string, topic string, partition int32) {
	partitionLabel := strconv.FormatInt(int64(partition), 10)
	conversationBucketLRUEvictTotal.WithLabelValues(consumerGroup, topic, partitionLabel).Inc()
}

func ObserveConversationBucketGCScan(consumerGroup string, topic string, partition int32) {
	partitionLabel := strconv.FormatInt(int64(partition), 10)
	conversationBucketGCScanTotal.WithLabelValues(consumerGroup, topic, partitionLabel).Inc()
}

func ObserveConversationBucketGCSkipped(consumerGroup string, topic string, partition int32, reason string) {
	partitionLabel := strconv.FormatInt(int64(partition), 10)
	conversationBucketGCSkippedTotal.WithLabelValues(consumerGroup, topic, partitionLabel, reason).Inc()
}

func mysqlDBStats() sql.DBStats {
	if dao.GormDB == nil {
		return sql.DBStats{}
	}
	sqlDB, err := dao.GormDB.DB()
	if err != nil || sqlDB == nil {
		return sql.DBStats{}
	}
	return sqlDB.Stats()
}
