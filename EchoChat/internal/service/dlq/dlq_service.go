package dlq

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/dto/respond"
	"echo_chat_server/internal/model"
)

type service struct{}

var Service = &service{}

func (s *service) List(req request.DLQListRequest) (respond.DLQListRespond, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := dao.GormDB.Model(&model.DLQMessage{})
	query = applyDLQFilters(query, req)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return respond.DLQListRespond{}, err
	}

	var records []model.DLQMessage
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return respond.DLQListRespond{}, err
	}

	items := make([]respond.DLQMessageItem, 0, len(records))
	for _, record := range records {
		items = append(items, buildDLQMessageItem(record))
	}
	return respond.DLQListRespond{Total: total, List: items}, nil
}

func (s *service) Detail(id int64) (respond.DLQMessageDetail, error) {
	record, err := s.getByID(id)
	if err != nil {
		return respond.DLQMessageDetail{}, err
	}
	return respond.DLQMessageDetail{
		DLQMessageItem:  buildDLQMessageItem(record),
		RawPayload:      record.RawPayload,
		PayloadSnapshot: record.PayloadSnapshot,
		ContextSnapshot: record.ContextSnapshot,
	}, nil
}

func (s *service) Logs(id int64) ([]respond.DLQOperationLogItem, error) {
	var logs []model.DLQOperationLog
	if err := dao.GormDB.Where("dlq_id = ?", id).Order("id DESC").Find(&logs).Error; err != nil {
		return nil, err
	}
	items := make([]respond.DLQOperationLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, respond.DLQOperationLogItem{
			ID:                 log.ID,
			DLQID:              log.DLQID,
			Action:             log.Action,
			Operator:           log.Operator,
			Remark:             log.Remark,
			BeforeManualStatus: log.BeforeManualStatus,
			AfterManualStatus:  log.AfterManualStatus,
			CreatedAt:          log.CreatedAt,
		})
	}
	return items, nil
}

func (s *service) Stats() (respond.DLQStatsRespond, error) {
	stats := respond.DLQStatsRespond{}

	type aggregateRow struct {
		Key   string
		Total int64
	}

	var statusRows []aggregateRow
	if err := dao.GormDB.Model(&model.DLQMessage{}).
		Select("status AS key, COUNT(*) AS total").
		Group("status").
		Scan(&statusRows).Error; err != nil {
		return respond.DLQStatsRespond{}, err
	}
	for _, row := range statusRows {
		stats.Total += row.Total
		switch row.Key {
		case model.DLQStatusPending:
			stats.AutoPending = row.Total
		case model.DLQStatusRetrying:
			stats.AutoRetrying = row.Total
		case model.DLQStatusManual:
			stats.ManualTotal = row.Total
		case model.DLQStatusDone:
			stats.DoneTotal = row.Total
		}
	}

	var manualRows []aggregateRow
	if err := dao.GormDB.Model(&model.DLQMessage{}).
		Where("status = ?", model.DLQStatusManual).
		Select("manual_status AS key, COUNT(*) AS total").
		Group("manual_status").
		Scan(&manualRows).Error; err != nil {
		return respond.DLQStatsRespond{}, err
	}
	for _, row := range manualRows {
		switch row.Key {
		case model.DLQManualStatusOpen:
			stats.ManualOpen = row.Total
		case model.DLQManualStatusInProgress:
			stats.ManualInProgress = row.Total
		case model.DLQManualStatusClosed:
			stats.ManualClosed = row.Total
		}
	}

	type stageRow struct {
		Stage        string
		Status       string
		ManualStatus string
		Total        int64
	}
	var rows []stageRow
	if err := dao.GormDB.Model(&model.DLQMessage{}).
		Select("stage, status, manual_status, COUNT(*) AS total").
		Group("stage, status, manual_status").
		Scan(&rows).Error; err != nil {
		return respond.DLQStatsRespond{}, err
	}
	stageMap := make(map[string]*respond.DLQStageStat)
	for _, row := range rows {
		item, ok := stageMap[row.Stage]
		if !ok {
			item = &respond.DLQStageStat{Stage: row.Stage}
			stageMap[row.Stage] = item
		}
		item.Total += row.Total
		switch row.Status {
		case model.DLQStatusPending:
			item.Pending += row.Total
		case model.DLQStatusRetrying:
			item.Retrying += row.Total
		case model.DLQStatusManual:
			item.Manual += row.Total
			switch row.ManualStatus {
			case model.DLQManualStatusOpen:
				item.Open += row.Total
			case model.DLQManualStatusInProgress:
				item.InProgress += row.Total
			case model.DLQManualStatusClosed:
				item.Closed += row.Total
			}
		case model.DLQStatusDone:
			item.Done += row.Total
		}
	}
	for _, item := range stageMap {
		stats.StageStats = append(stats.StageStats, *item)
	}
	sort.Slice(stats.StageStats, func(i, j int) bool {
		return stats.StageStats[i].Stage < stats.StageStats[j].Stage
	})
	return stats, nil
}

// 输入：id 是要接手的 DLQ 记录 ID，operator 是当前执行接手动作的操作人。
// 输出：成功时返回 nil，失败时返回错误信息。
// 作用：把一条已经进入人工治理池的 DLQ 记录标记为“人工处理中”，并同步写入一条 claim 操作日志。
func (s *service) Claim(id int64, operator string) error {
	// 用一个数据库事务把“改主记录状态”和“写操作日志”包在一起，保证两者要么一起成功，要么一起失败。
	return dao.GormDB.Transaction(func(tx *gorm.DB) error {
		// 先用 FOR UPDATE 锁住目标记录，避免并发下被其他人同时接手或修改。
		record, err := getDLQMessageForUpdate(tx, id)
		// 如果记录不存在，或者加锁查询失败，直接返回错误并回滚事务。
		if err != nil {
			return err
		}
		// 只有已经转入人工治理池的记录，才允许人工接手。
		if record.Status != model.DLQStatusManual {
			return fmt.Errorf("当前记录仍处于自动治理流程，不能人工接手")
		}
		// 已经关闭的记录不能直接接手，必须先 reopen 回到可处理状态。
		if record.ManualStatus == model.DLQManualStatusClosed {
			return fmt.Errorf("当前记录已关闭，请先 reopen")
		}
		// 记录本次接手操作发生的时间，后面会同时写到主记录里。
		now := time.Now()
		// 先保存修改前的人工状态，便于后面写操作日志时记录状态流转。
		beforeManualStatus := record.ManualStatus
		// 把人工状态从 open 改成 in_progress，表示这条记录已经有人接手处理中。
		record.ManualStatus = model.DLQManualStatusInProgress
		// 把当前操作人写入 assignee，表示当前负责人是谁。
		record.Assignee = operator
		// 记录这条 DLQ 被人工接手的时间点。
		record.ClaimedAt = &now
		// 更新主记录的更新时间，反映本次状态变更。
		record.UpdatedAt = now
		// 把更新后的主记录写回 dlq_message 表；如果失败，事务整体回滚。
		if err := tx.Save(&record).Error; err != nil {
			return err
		}
		// 在同一个事务里补一条 claim 审计日志，记录是谁把状态从 before 改成了现在的状态。
		return createDLQOperationLogTx(tx, record.ID, model.DLQActionClaim, operator, beforeManualStatus, record.ManualStatus, "")
	})
}

func (s *service) Reopen(id int64, operator string) error {
	return dao.GormDB.Transaction(func(tx *gorm.DB) error {
		record, err := getDLQMessageForUpdate(tx, id)
		if err != nil {
			return err
		}
		if record.Status != model.DLQStatusManual {
			return fmt.Errorf("当前记录不在人工治理池中")
		}
		if record.ManualStatus != model.DLQManualStatusClosed {
			return fmt.Errorf("当前记录不是 closed 状态，不能 reopen")
		}
		beforeManualStatus := record.ManualStatus
		record.ManualStatus = model.DLQManualStatusOpen
		record.CloseReason = ""
		record.Assignee = ""
		record.ClaimedAt = nil
		record.ResolvedAt = nil
		record.UpdatedAt = time.Now()
		if err := tx.Save(&record).Error; err != nil {
			return err
		}
		return createDLQOperationLogTx(tx, record.ID, model.DLQActionReopen, operator, beforeManualStatus, record.ManualStatus, "")
	})
}

func (s *service) Close(id int64, operator, closeReason, remark string) error {
	return dao.GormDB.Transaction(func(tx *gorm.DB) error {
		record, err := getDLQMessageForUpdate(tx, id)
		if err != nil {
			return err
		}
		if record.Status != model.DLQStatusManual {
			return fmt.Errorf("当前记录不在人工治理池中")
		}
		if record.ManualStatus == model.DLQManualStatusClosed {
			return fmt.Errorf("当前记录已关闭")
		}
		if !isValidDLQCloseReason(closeReason) {
			return fmt.Errorf("close_reason 非法")
		}
		now := time.Now()
		beforeManualStatus := record.ManualStatus
		record.ManualStatus = model.DLQManualStatusClosed
		record.CloseReason = closeReason
		record.Remark = remark
		record.ResolvedAt = &now
		record.UpdatedAt = now
		if err := tx.Save(&record).Error; err != nil {
			return err
		}
		return createDLQOperationLogTx(tx, record.ID, model.DLQActionClose, operator, beforeManualStatus, record.ManualStatus, remark)
	})
}

func (s *service) UpdateRemark(id int64, operator, remark string) error {
	return dao.GormDB.Transaction(func(tx *gorm.DB) error {
		record, err := getDLQMessageForUpdate(tx, id)
		if err != nil {
			return err
		}
		beforeManualStatus := record.ManualStatus
		record.Remark = remark
		record.UpdatedAt = time.Now()
		if err := tx.Save(&record).Error; err != nil {
			return err
		}
		return createDLQOperationLogTx(tx, record.ID, model.DLQActionRemark, operator, beforeManualStatus, record.ManualStatus, remark)
	})
}

func (s *service) Create(record *model.DLQMessage) error {
	if record == nil {
		return errors.New("dlq record is nil")
	}
	now := time.Now()
	if record.ManualStatus == "" {
		record.ManualStatus = model.DLQManualStatusOpen
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}
	if record.FirstFailedAt.IsZero() {
		record.FirstFailedAt = now
	}
	if record.LastFailedAt.IsZero() {
		record.LastFailedAt = now
	}
	if err := dao.GormDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "topic"},
			{Name: "partition_id"},
			{Name: "offset_id"},
			{Name: "stage"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"message_id",
			"conversation_key",
			"session_seq",
			"raw_payload",
			"payload_snapshot",
			"context_snapshot",
			"error_code",
			"last_error",
			"failure_type",
			"handle_type",
			"status",
			"manual_status",
			"close_reason",
			"max_attempt_count",
			"next_retry_at",
			"last_failed_at",
			"updated_at",
		}),
	}).Create(record).Error; err != nil {
		return err
	}
	if record.ID == 0 {
		if err := dao.GormDB.Model(&model.DLQMessage{}).
			Select("id").
			Where("topic = ? AND partition_id = ? AND offset_id = ? AND stage = ?", record.Topic, record.PartitionID, record.OffsetID, record.Stage).
			First(record).Error; err != nil {
			return err
		}
	}
	return createDLQOperationLog(record.ID, model.DLQActionCreate, "system", "", record.ManualStatus, "")
}

func (s *service) getByID(id int64) (model.DLQMessage, error) {
	var record model.DLQMessage
	if err := dao.GormDB.Where("id = ?", id).First(&record).Error; err != nil {
		return model.DLQMessage{}, err
	}
	return record, nil
}

func applyDLQFilters(query *gorm.DB, req request.DLQListRequest) *gorm.DB {
	if value := strings.TrimSpace(req.MessageID); value != "" {
		query = query.Where("message_id = ?", value)
	}
	if value := strings.TrimSpace(req.ConversationKey); value != "" {
		query = query.Where("conversation_key = ?", value)
	}
	if value := strings.TrimSpace(req.Stage); value != "" {
		query = query.Where("stage = ?", value)
	}
	if value := strings.TrimSpace(req.ErrorCode); value != "" {
		query = query.Where("error_code = ?", value)
	}
	if value := strings.TrimSpace(req.FailureType); value != "" {
		query = query.Where("failure_type = ?", value)
	}
	if value := strings.TrimSpace(req.HandleType); value != "" {
		query = query.Where("handle_type = ?", value)
	}
	if value := strings.TrimSpace(req.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(req.ManualStatus); value != "" {
		query = query.Where("manual_status = ?", value)
	}
	return query
}

func buildDLQMessageItem(record model.DLQMessage) respond.DLQMessageItem {
	return respond.DLQMessageItem{
		ID:              record.ID,
		MessageID:       record.MessageID,
		ConversationKey: record.ConversationKey,
		SessionSeq:      record.SessionSeq,
		Topic:           record.Topic,
		PartitionID:     record.PartitionID,
		OffsetID:        record.OffsetID,
		Stage:           record.Stage,
		ErrorCode:       record.ErrorCode,
		LastError:       record.LastError,
		FailureType:     record.FailureType,
		HandleType:      record.HandleType,
		Status:          record.Status,
		ManualStatus:    record.ManualStatus,
		CloseReason:     record.CloseReason,
		AttemptCount:    record.AttemptCount,
		MaxAttemptCount: record.MaxAttemptCount,
		NextRetryAt:     record.NextRetryAt,
		Assignee:        record.Assignee,
		ClaimedAt:       record.ClaimedAt,
		Remark:          record.Remark,
		FirstFailedAt:   record.FirstFailedAt,
		LastFailedAt:    record.LastFailedAt,
		ResolvedAt:      record.ResolvedAt,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
}

func getDLQMessageForUpdate(tx *gorm.DB, id int64) (model.DLQMessage, error) {
	var record model.DLQMessage
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", id).First(&record).Error; err != nil {
		return model.DLQMessage{}, err
	}
	return record, nil
}

func createDLQOperationLog(dlqID int64, action, operator, beforeManualStatus, afterManualStatus, remark string) error {
	return createDLQOperationLogTx(dao.GormDB, dlqID, action, operator, beforeManualStatus, afterManualStatus, remark)
}

func createDLQOperationLogTx(tx *gorm.DB, dlqID int64, action, operator, beforeManualStatus, afterManualStatus, remark string) error {
	entry := model.DLQOperationLog{
		DLQID:              dlqID,
		Action:             action,
		Operator:           operator,
		Remark:             remark,
		BeforeManualStatus: beforeManualStatus,
		AfterManualStatus:  afterManualStatus,
		CreatedAt:          time.Now(),
	}
	return tx.Create(&entry).Error
}

func isValidDLQCloseReason(closeReason string) bool {
	switch closeReason {
	case model.DLQCloseReasonDiscarded,
		model.DLQCloseReasonExternallyFixed,
		model.DLQCloseReasonExpected,
		model.DLQCloseReasonMergedIncident:
		return true
	default:
		return false
	}
}
