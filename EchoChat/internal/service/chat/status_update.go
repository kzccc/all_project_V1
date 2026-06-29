package chat

import (
	"hash/fnv"
	"time"

	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/model"
	"echo_chat_server/internal/observability"
	"echo_chat_server/pkg/enum/message/message_status_enum"
	"echo_chat_server/pkg/zlog"
)

type statusUpdateJob struct {
	uuid  string
	route string
}

type statusUpdateDispatcher struct {
	started bool
	workers []chan statusUpdateJob
}

var kafkaStatusUpdater = &statusUpdateDispatcher{}

func init() {
	kafkaStatusUpdater.start()
}

func (d *statusUpdateDispatcher) start() {
	if d.started {
		return
	}
	d.started = true
	workerCount := 4
	d.workers = make([]chan statusUpdateJob, 0, workerCount)
	for i := 0; i < workerCount; i++ {
		jobs := make(chan statusUpdateJob, 4096)
		d.workers = append(d.workers, jobs)
		go d.runWorker(jobs)
	}
}

func (d *statusUpdateDispatcher) workerIndex(uuid string) int {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(uuid))
	return int(hasher.Sum32() % uint32(len(d.workers)))
}

func (d *statusUpdateDispatcher) enqueue(uuid, route string) {
	if uuid == "" {
		return
	}
	d.workers[d.workerIndex(uuid)] <- statusUpdateJob{uuid: uuid, route: route}
}

func (d *statusUpdateDispatcher) runWorker(jobs <-chan statusUpdateJob) {
	const batchSize = 128
	const flushInterval = 5 * time.Millisecond

	for {
		first := <-jobs
		batch := []statusUpdateJob{first}

		for len(batch) < batchSize {
			select {
			case job := <-jobs:
				batch = append(batch, job)
			default:
				goto flush
			}
		}

	flush:
		timer := time.NewTimer(flushInterval)
	collect:
		for len(batch) < batchSize {
			select {
			case job := <-jobs:
				batch = append(batch, job)
			case <-timer.C:
				break collect
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		d.flush(batch)
	}
}

func (d *statusUpdateDispatcher) flush(batch []statusUpdateJob) {
	if len(batch) == 0 {
		return
	}

	seen := make(map[string]struct{}, len(batch))
	uuids := make([]string, 0, len(batch))
	for _, job := range batch {
		if _, ok := seen[job.uuid]; ok {
			continue
		}
		seen[job.uuid] = struct{}{}
		uuids = append(uuids, job.uuid)
	}

	start := time.Now()
	err := dao.GormDB.Model(&model.Message{}).
		Where("uuid IN ? AND status <> ?", uuids, message_status_enum.Sent).
		Update("status", message_status_enum.Sent).Error
	duration := time.Since(start)
	perItemDuration := duration / time.Duration(len(batch))

	if err != nil {
		for _, job := range batch {
			observability.ObserveWSStatusUpdate(job.route, perItemDuration, "failure")
			go func(retryJob statusUpdateJob) {
				time.Sleep(20 * time.Millisecond)
				d.enqueue(retryJob.uuid, retryJob.route)
			}(job)
		}
		zlog.Error(err.Error())
		return
	}

	for _, job := range batch {
		observability.ObserveWSStatusUpdate(job.route, perItemDuration, "success")
	}
}
