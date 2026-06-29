package chat

import (
	"fmt"
	"hash/fnv"
	"time"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/model"
	"echo_chat_server/pkg/zlog"

	"go.uber.org/zap"
)

type kafkaGroupAsyncJob struct {
	message    model.Message
	sendAvatar string
	retryCount int
}

type kafkaGroupAsyncDispatcher struct {
	started bool
	workers []chan kafkaGroupAsyncJob
}

var kafkaGroupAsync = &kafkaGroupAsyncDispatcher{}

func (d *kafkaGroupAsyncDispatcher) ensureStarted() {
	if d.started {
		return
	}
	d.started = true
	workerCount := config.GetConfig().KafkaConfig.ResolveGroupAsyncShardCount()
	if workerCount < 1 {
		workerCount = 1
	}
	d.workers = make([]chan kafkaGroupAsyncJob, 0, workerCount)
	for i := 0; i < workerCount; i++ {
		jobs := make(chan kafkaGroupAsyncJob, 2048)
		d.workers = append(d.workers, jobs)
		go d.runWorker(jobs)
	}
}

func (d *kafkaGroupAsyncDispatcher) enqueue(job kafkaGroupAsyncJob) error {
	d.ensureStarted()
	workerIndex := d.workerIndex(job.message)
	select {
	case d.workers[workerIndex] <- job:
		return nil
	default:
		return fmt.Errorf("group async worker queue %d is full", workerIndex)
	}
}

func (d *kafkaGroupAsyncDispatcher) workerIndex(message model.Message) int {
	if len(d.workers) == 0 {
		return 0
	}
	if message.SessionSeq > 0 {
		idx := int(message.SessionSeq % int64(len(d.workers)))
		if idx < 0 {
			idx += len(d.workers)
		}
		return idx
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(message.ReceiveId))
	return int(hasher.Sum32() % uint32(len(d.workers)))
}

func (d *kafkaGroupAsyncDispatcher) runWorker(jobs <-chan kafkaGroupAsyncJob) {
	for job := range jobs {
		if err := KafkaChatServer.processAsyncGroupJob(job); err != nil {
			if job.retryCount >= 3 {
				zlog.Error(
					"kafka.group.async.drop",
					zap.String("event", "kafka.group.async.drop"),
					zap.String("module", "chat.kafka"),
					zap.String("group_id", job.message.ReceiveId),
					zap.String("message_id", job.message.Uuid),
					zap.Int64("session_seq", job.message.SessionSeq),
					zap.Int("retry_count", job.retryCount),
					zap.String("error", err.Error()),
				)
				continue
			}
			retryJob := job
			retryJob.retryCount++
			time.AfterFunc(20*time.Millisecond, func() {
				if enqueueErr := d.enqueue(retryJob); enqueueErr != nil {
					zlog.Error(
						"kafka.group.async.retry_enqueue_failed",
						zap.String("event", "kafka.group.async.retry_enqueue_failed"),
						zap.String("module", "chat.kafka"),
						zap.String("group_id", retryJob.message.ReceiveId),
						zap.String("message_id", retryJob.message.Uuid),
						zap.Int64("session_seq", retryJob.message.SessionSeq),
						zap.Int("retry_count", retryJob.retryCount),
						zap.String("error", enqueueErr.Error()),
					)
				}
			})
		}
	}
}

func (k *KafkaServer) enqueueConsumedGroupMessage(message model.Message, sendAvatar string) error {
	return kafkaGroupAsync.enqueue(kafkaGroupAsyncJob{
		message:    message,
		sendAvatar: sendAvatar,
	})
}

func (k *KafkaServer) processAsyncGroupJob(job kafkaGroupAsyncJob) error {
	message := job.message
	alreadyProcessed, persisted, err := saveKafkaMessage(&message)
	if err != nil {
		return err
	}
	if alreadyProcessed {
		return nil
	}
	messageBackUUID := ""
	if persisted {
		messageBackUUID = message.Uuid
	}
	return k.handleConsumedGroupMessage(nil, &message, job.sendAvatar, messageBackUUID)
}
