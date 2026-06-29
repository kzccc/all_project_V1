package chat

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gomysql "github.com/go-sql-driver/mysql"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/model"
	"echo_chat_server/internal/observability"
	"echo_chat_server/internal/pressure"
	"echo_chat_server/pkg/enum/message/message_status_enum"
	"echo_chat_server/pkg/util/random"
	"echo_chat_server/pkg/zlog"

	"go.uber.org/zap"
)

type consumerProcessError struct {
	stage     string
	retryable bool
	err       error
}

func (e *consumerProcessError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *consumerProcessError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func retryableConsumerError(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &consumerProcessError{
		stage:     stage,
		retryable: true,
		err:       err,
	}
}

func nonRetryableConsumerError(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &consumerProcessError{
		stage:     stage,
		retryable: false,
		err:       err,
	}
}

func consumerErrorMeta(err error) (stage string, retryable bool) {
	var processErr *consumerProcessError
	if errors.As(err, &processErr) {
		return processErr.stage, processErr.retryable
	}
	return "unknown", true
}

func buildMessageID() string {
	return fmt.Sprintf("M%s", random.GetNowAndLenRandomString(11))
}

func ensureMessageID(messageID string) string {
	if messageID != "" {
		return messageID
	}
	return buildMessageID()
}

func isDuplicateMessageError(err error) bool {
	var mysqlErr *gomysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return false
}

func isMessageUUIDDuplicateError(err error) bool {
	var mysqlErr *gomysql.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		return false
	}
	return strings.Contains(strings.ToLower(mysqlErr.Message), "for key 'message.uuid'") ||
		strings.Contains(strings.ToLower(mysqlErr.Message), "for key 'uuid'")
}

func isConversationSeqDuplicateError(err error) bool {
	var mysqlErr *gomysql.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		return false
	}
	return strings.Contains(strings.ToLower(mysqlErr.Message), "uniq_message_conversation_seq")
}

type kafkaPersistResult struct {
	message          model.Message
	alreadyProcessed bool
	persisted        bool
	err              error
}

type kafkaPersistJob struct {
	message       model.Message
	result        chan kafkaPersistResult
	workerIndex   int
	enqueueDoneAt time.Time
	enqueueStart  time.Time
}

type kafkaMessagePersister struct {
	startOnce    sync.Once
	workers      []chan *kafkaPersistJob
	workerLoads  []atomic.Int64
	activeWorker []atomic.Int64
	flushSeq     atomic.Int64
}

var kafkaPersister = &kafkaMessagePersister{}

func (p *kafkaMessagePersister) ensureStarted() {
	p.startOnce.Do(func() {
		workerCount := config.GetConfig().KafkaConfig.ResolveMysqlPersistWorkerCount()
		queueSize := config.GetConfig().KafkaConfig.ResolveMysqlPersistQueueSize()
		if workerCount < 1 {
			workerCount = 1
		}
		if queueSize < 1 {
			queueSize = 1
		}
		p.workers = make([]chan *kafkaPersistJob, 0, workerCount)
		p.workerLoads = make([]atomic.Int64, workerCount)
		p.activeWorker = make([]atomic.Int64, workerCount)
		for i := 0; i < workerCount; i++ {
			workerJobs := make(chan *kafkaPersistJob, queueSize)
			p.workers = append(p.workers, workerJobs)
			go p.run(i, workerJobs)
		}
	})
}

func (p *kafkaMessagePersister) run(workerIndex int, jobs <-chan *kafkaPersistJob) {
	batchSize := config.GetConfig().KafkaConfig.ResolveMysqlPersistBatchSize()
	firstJobHold := config.GetConfig().KafkaConfig.ResolveMysqlPersistFirstJobHold()
	flushInterval := config.GetConfig().KafkaConfig.ResolveMysqlPersistFlushInterval()
	if batchSize <= 1 {
		batchSize = 1
	}
	if flushInterval <= 0 {
		flushInterval = 5 * time.Millisecond
	}

	for {
		firstJob := <-jobs
		p.workerLoads[workerIndex].Add(-1)
		p.activeWorker[workerIndex].Store(1)
		workerStartAt := time.Now()
		pressure.ObserveBenchmarkEventAt(firstJob.message.Uuid, "mysql_persist_worker_start", workerStartAt, map[string]interface{}{
			"worker_index": workerIndex,
		})
		batch := []*kafkaPersistJob{firstJob}
		flushReason := "single"
		if batchSize == 1 {
			p.flush(workerIndex, batch, "batch_full")
			p.activeWorker[workerIndex].Store(0)
			continue
		}

		if firstJobHold > 0 {
			collectStartAt := time.Now()
			pressure.ObserveBenchmarkEventAt(firstJob.message.Uuid, "mysql_persist_batch_collect_start", collectStartAt, map[string]interface{}{
				"worker_index":      workerIndex,
				"target_batch_size": batchSize,
			})
			holdTimer := time.NewTimer(firstJobHold)
			select {
			case job := <-jobs:
				p.workerLoads[workerIndex].Add(-1)
				batch = append(batch, job)
			case <-holdTimer.C:
			}
			if !holdTimer.Stop() {
				select {
				case <-holdTimer.C:
				default:
				}
			}
		}

		for len(batch) < batchSize {
			select {
			case job := <-jobs:
				p.workerLoads[workerIndex].Add(-1)
				batch = append(batch, job)
			default:
				goto maybeFlush
			}
		}
		flushReason = "batch_full"

	maybeFlush:
		if len(batch) == 1 {
			p.flush(workerIndex, batch, flushReason)
			p.activeWorker[workerIndex].Store(0)
			continue
		}

		collectStartAt := time.Now()
		pressure.ObserveBenchmarkEventAt(firstJob.message.Uuid, "mysql_persist_batch_collect_start", collectStartAt, map[string]interface{}{
			"worker_index":      workerIndex,
			"target_batch_size": batchSize,
		})
		timer := time.NewTimer(flushInterval)
	collect:
		for len(batch) < batchSize {
			select {
			case job := <-jobs:
				p.workerLoads[workerIndex].Add(-1)
				batch = append(batch, job)
				if len(batch) >= batchSize {
					flushReason = "batch_full"
					break collect
				}
			case <-timer.C:
				flushReason = "timer"
				break collect
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		p.flush(workerIndex, batch, flushReason)
		p.activeWorker[workerIndex].Store(0)
	}
}

func (p *kafkaMessagePersister) workerIndexForMessage(message model.Message) int {
	if len(p.workers) == 0 {
		return 0
	}
	if len(p.workers) == 1 {
		return 0
	}
	bestIndex := 0
	bestLoad := p.persistWorkerLoad(0)
	for i := 1; i < len(p.workers); i++ {
		load := p.persistWorkerLoad(i)
		if load < bestLoad {
			bestIndex = i
			bestLoad = load
		}
	}
	return bestIndex
}

func (p *kafkaMessagePersister) persistWorkerLoad(workerIndex int) int64 {
	load := p.workerLoads[workerIndex].Load()
	if p.activeWorker[workerIndex].Load() > 0 {
		load++
	}
	return load
}

func (p *kafkaMessagePersister) flush(workerIndex int, batch []*kafkaPersistJob, reason string) {
	if len(batch) == 0 {
		return
	}
	sqlExecStartAt := time.Now()
	for _, job := range batch {
		pressure.ObserveBenchmarkEventAt(job.message.Uuid, "mysql_persist_sql_exec_start", sqlExecStartAt, map[string]interface{}{
			"worker_index":     workerIndex,
			"flush_batch_size": len(batch),
			"flush_reason":     reason,
		})
	}
	start := time.Now()
	err := persistKafkaMessages(batch)
	observability.ObserveMysqlPersistFlush(reason, len(batch), time.Since(start), err == nil)
	flushDoneAt := time.Now()
	flushSeq := p.flushSeq.Add(1)
	for _, job := range batch {
		pressure.ObserveBenchmarkEventAt(job.message.Uuid, "mysql_persist_sql_exec_done", flushDoneAt, map[string]interface{}{
			"worker_index":     workerIndex,
			"flush_batch_size": len(batch),
			"flush_reason":     reason,
		})
		pressure.ObserveBenchmarkEventAt(job.message.Uuid, "mysql_persist_flush_done", flushDoneAt, map[string]interface{}{
			"worker_index":     workerIndex,
			"flush_seq":        flushSeq,
			"flush_batch_size": len(batch),
			"flush_reason":     reason,
		})
	}
	if err != nil {
		for _, job := range batch {
			job.result <- kafkaPersistResult{err: err}
		}
		return
	}
}

func buildMessageInsertSQL(batchSize int) string {
	columns := []string{
		"uuid", "session_id", "type", "content", "url", "send_id", "send_name", "send_avatar",
		"receive_id", "conversation_key", "file_type", "file_name", "file_size", "status", "session_seq",
		"created_at", "send_at", "av_data",
	}
	valuePlaceholder := "(" + strings.TrimRight(strings.Repeat("?,", len(columns)), ",") + ")"
	valueParts := make([]string, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		valueParts = append(valueParts, valuePlaceholder)
	}
	return fmt.Sprintf(
		"INSERT INTO message (%s) VALUES %s",
		strings.Join(columns, ", "),
		strings.Join(valueParts, ", "),
	)
}

func buildMessageInsertArgs(batch []*kafkaPersistJob) []interface{} {
	args := make([]interface{}, 0, len(batch)*18)
	for _, job := range batch {
		message := job.message
		var sendAt interface{}
		if message.SendAt.Valid {
			sendAt = message.SendAt.Time
		} else {
			sendAt = nil
		}
		args = append(args,
			message.Uuid,
			message.SessionId,
			message.Type,
			message.Content,
			message.Url,
			message.SendId,
			message.SendName,
			message.SendAvatar,
			message.ReceiveId,
			message.ConversationKey,
			message.FileType,
			message.FileName,
			message.FileSize,
			message.Status,
			message.SessionSeq,
			message.CreatedAt,
			sendAt,
			message.AVdata,
		)
	}
	return args
}

func queryExistingMessage(uuid string) (model.Message, error) {
	var existing model.Message
	if res := dao.GormDB.Where("uuid = ?", uuid).First(&existing); res.Error != nil {
		return model.Message{}, res.Error
	}
	return existing, nil
}

func resolveDuplicateJob(job *kafkaPersistJob) {
	existingMessage, err := queryExistingMessage(job.message.Uuid)
	if err != nil {
		job.result <- kafkaPersistResult{err: err}
		return
	}
	if existingMessage.Status == message_status_enum.Sent {
		zlog.Info(
			"kafka.consume.chat.duplicate_skipped",
			zap.String("event", "kafka.consume.chat.duplicate_skipped"),
			zap.String("module", "chat.kafka"),
			zap.String("message_id", existingMessage.Uuid),
			zap.String("session_id", existingMessage.SessionId),
		)
		job.result <- kafkaPersistResult{
			message:          existingMessage,
			alreadyProcessed: true,
			persisted:        true,
		}
		return
	}
	zlog.Info(
		"kafka.consume.chat.duplicate_replayed",
		zap.String("event", "kafka.consume.chat.duplicate_replayed"),
		zap.String("module", "chat.kafka"),
		zap.String("message_id", existingMessage.Uuid),
		zap.String("session_id", existingMessage.SessionId),
	)
	job.result <- kafkaPersistResult{
		message:          existingMessage,
		alreadyProcessed: false,
		persisted:        true,
	}
}

func persistKafkaMessagesFallback(sqlDB *sql.DB, batch []*kafkaPersistJob) {
	for _, job := range batch {
		singleBatch := []*kafkaPersistJob{job}
		query := buildMessageInsertSQL(1)
		args := buildMessageInsertArgs(singleBatch)
		_, err := sqlDB.Exec(query, args...)
		if err == nil {
			logMessagePersist("kafka", &job.message)
			job.result <- kafkaPersistResult{
				message:          job.message,
				alreadyProcessed: false,
				persisted:        true,
			}
			continue
		}
		if !isDuplicateMessageError(err) {
			if dlqErr := writePersistFailureToDLQ(job.message, err); dlqErr != nil {
				zlog.Error(dlqErr.Error())
			}
			job.result <- kafkaPersistResult{err: err}
			continue
		}
		if isConversationSeqDuplicateError(err) {
			observability.ObserveConversationSeqConflict("duplicate_session_seq_insert")
			zlog.Error(
				"session_seq.conflict_rejected",
				zap.String("event", "session_seq.conflict_rejected"),
				zap.String("module", "chat.kafka"),
				zap.String("conversation_key", job.message.ConversationKey),
				zap.Int64("session_seq", job.message.SessionSeq),
				zap.String("message_id", job.message.Uuid),
				zap.String("error", err.Error()),
			)
			if dlqErr := writePersistFailureToDLQ(job.message, err); dlqErr != nil {
				zlog.Error(dlqErr.Error())
			}
			job.result <- kafkaPersistResult{err: err}
			continue
		}
		if !isMessageUUIDDuplicateError(err) {
			if dlqErr := writePersistFailureToDLQ(job.message, err); dlqErr != nil {
				zlog.Error(dlqErr.Error())
			}
			job.result <- kafkaPersistResult{err: err}
			continue
		}
		resolveDuplicateJob(job)
	}
}

func persistKafkaMessages(batch []*kafkaPersistJob) error {
	sqlDB, err := dao.GormDB.DB()
	if err != nil {
		return err
	}

	query := buildMessageInsertSQL(len(batch))
	args := buildMessageInsertArgs(batch)
	if _, err := sqlDB.Exec(query, args...); err != nil {
		if isDuplicateMessageError(err) {
			persistKafkaMessagesFallback(sqlDB, batch)
			return nil
		}
		for _, job := range batch {
			if dlqErr := writePersistFailureToDLQ(job.message, err); dlqErr != nil {
				zlog.Error(dlqErr.Error())
			}
		}
		return err
	}

	for _, job := range batch {
		logMessagePersist("kafka", &job.message)
		job.result <- kafkaPersistResult{
			message:          job.message,
			alreadyProcessed: false,
			persisted:        true,
		}
	}
	return nil
}

func saveKafkaMessage(message *model.Message) (alreadyProcessed bool, persisted bool, err error) {
	if config.GetConfig().KafkaConfig.UseMysqlPersistNoopExperimental() {
		return false, false, nil
	}

	kafkaPersister.ensureStarted()
	job := &kafkaPersistJob{
		message: *message,
		result:  make(chan kafkaPersistResult, 1),
	}
	workerIndex := kafkaPersister.workerIndexForMessage(*message)
	job.workerIndex = workerIndex
	workerJobs := kafkaPersister.workers[workerIndex]
	kafkaPersister.workerLoads[workerIndex].Add(1)
	queueDepth := len(workerJobs)
	enqueueStart := time.Now()
	job.enqueueStart = enqueueStart
	workerJobs <- job
	job.enqueueDoneAt = time.Now()
	observability.ObserveMysqlPersistEnqueue(queueDepth, job.enqueueDoneAt.Sub(enqueueStart))
	pressure.ObserveBenchmarkEventAt(message.Uuid, "mysql_persist_enqueue_start", enqueueStart, map[string]interface{}{
		"worker_index": workerIndex,
		"queue_depth":  queueDepth,
	})
	pressure.ObserveBenchmarkEventAt(message.Uuid, "mysql_persist_enqueue_done", job.enqueueDoneAt, map[string]interface{}{
		"worker_index": workerIndex,
		"queue_depth":  queueDepth,
	})
	result := <-job.result
	if result.err != nil {
		return false, false, result.err
	}
	*message = result.message
	return result.alreadyProcessed, result.persisted, nil
}
