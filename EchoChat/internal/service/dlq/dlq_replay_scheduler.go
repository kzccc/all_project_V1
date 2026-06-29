package dlq

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/model"
	"echo_chat_server/pkg/zlog"
)

type ReplayScheduler struct {
	interval  time.Duration
	batchSize int
	runner    func(*model.DLQMessage) ReplayResult
}

func NewReplayScheduler(interval time.Duration, batchSize int, runner func(*model.DLQMessage) ReplayResult) *ReplayScheduler {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return &ReplayScheduler{
		interval:  interval,
		batchSize: batchSize,
		runner:    runner,
	}
}

func (s *ReplayScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.runOnce(); err != nil {
				zlog.Error(err.Error())
			}
		}
	}
}

func (s *ReplayScheduler) runOnce() error {
	records, err := s.claimDueRecords()
	if err != nil {
		return err
	}
	for i := range records {
		if err := s.runRecord(&records[i]); err != nil {
			zlog.Error(err.Error())
		}
	}
	return nil
}

func (s *ReplayScheduler) runRecord(record *model.DLQMessage) error {
	result := HandleDLQRecord(record)
	if s.runner != nil {
		result = s.runner(record)
	}
	return dao.GormDB.Transaction(func(tx *gorm.DB) error {
		latest, err := getDLQMessageForUpdate(tx, record.ID)
		if err != nil {
			return err
		}
		now := time.Now()
		beforeManualStatus := latest.ManualStatus
		latest.UpdatedAt = now
		latest.AttemptCount++
		if result.Success {
			latest.Status = model.DLQStatusDone
			latest.LastError = ""
			latest.NextRetryAt = nil
			latest.ResolvedAt = &now
			if err := tx.Save(&latest).Error; err != nil {
				return err
			}
			return createDLQOperationLogTx(tx, latest.ID, model.DLQActionDone, "system", beforeManualStatus, latest.ManualStatus, "auto replay success")
		}
		latest.LastFailedAt = now
		latest.LastError = errorString(result.FinalError)
		if result.Retryable && latest.AttemptCount < latest.MaxAttemptCount {
			latest.Status = model.DLQStatusPending
			if result.NextRetryAt != nil {
				latest.NextRetryAt = result.NextRetryAt
			}
			if err := tx.Save(&latest).Error; err != nil {
				return err
			}
			return createDLQOperationLogTx(tx, latest.ID, model.DLQActionAutoRetry, "system", beforeManualStatus, latest.ManualStatus, buildAutoRetryRemark(latest, result.FinalError))
		}
		latest.Status = model.DLQStatusManual
		latest.ManualStatus = model.DLQManualStatusOpen
		latest.NextRetryAt = nil
		if err := tx.Save(&latest).Error; err != nil {
			return err
		}
		return createDLQOperationLogTx(tx, latest.ID, model.DLQActionAutoRetry, "system", beforeManualStatus, latest.ManualStatus, buildEscalateRemark(latest, result.FinalError))
	})
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *ReplayScheduler) claimDueRecords() ([]model.DLQMessage, error) {
	var claimed []model.DLQMessage
	err := dao.GormDB.Transaction(func(tx *gorm.DB) error {
		var records []model.DLQMessage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND handle_type = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", model.DLQStatusPending, model.DLQHandleTypeAuto, time.Now()).
			Order("id ASC").
			Limit(s.batchSize).
			Find(&records).Error; err != nil {
			return err
		}
		for i := range records {
			record := &records[i]
			beforeManualStatus := record.ManualStatus
			record.Status = model.DLQStatusRetrying
			record.UpdatedAt = time.Now()
			if err := tx.Save(record).Error; err != nil {
				return err
			}
			if err := createDLQOperationLogTx(tx, record.ID, model.DLQActionAutoRetry, "system", beforeManualStatus, record.ManualStatus, "auto replay claimed"); err != nil {
				return err
			}
		}
		claimed = records
		return nil
	})
	return claimed, err
}

func buildAutoRetryRemark(record model.DLQMessage, finalErr error) string {
	remark := "auto replay failed"
	if record.NextRetryAt != nil {
		remark = "auto replay failed, requeue at " + record.NextRetryAt.Format(time.DateTime)
	}
	if finalErr != nil {
		remark += ": " + finalErr.Error()
	}
	return remark
}

func buildEscalateRemark(record model.DLQMessage, finalErr error) string {
	remark := "auto replay exhausted, transfer to manual"
	if record.MaxAttemptCount > 0 {
		remark = "auto replay exhausted after attempt_count=" + fmt.Sprintf("%d", record.AttemptCount)
	}
	if finalErr != nil {
		remark += ": " + finalErr.Error()
	}
	return remark
}
