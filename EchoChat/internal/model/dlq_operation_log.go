package model

import "time"

const (
	DLQActionCreate      = "create"
	DLQActionAutoRetry   = "auto_retry"
	DLQActionClaim       = "claim"
	DLQActionReopen      = "reopen"
	DLQActionClose       = "close"
	DLQActionRemark      = "remark"
	DLQActionDone        = "done"
)

// DLQOperationLog 保存治理动作审计日志。
type DLQOperationLog struct {
    ID           int64     `gorm:"column:id;primaryKey;autoIncrement;comment:主键"`
    DLQID        int64     `gorm:"column:dlq_id;not null;index:idx_dlq_log_dlq_id;comment:DLQ记录ID"`
    Action       string    `gorm:"column:action;type:varchar(32);not null;comment:动作"`
    Operator     string    `gorm:"column:operator;type:varchar(64);not null;comment:操作人"`
    Remark       string    `gorm:"column:remark;type:text;comment:备注"`
    BeforeManualStatus string `gorm:"column:before_manual_status;type:varchar(16);not null;comment:操作前人工治理状态"`
    AfterManualStatus  string `gorm:"column:after_manual_status;type:varchar(16);not null;comment:操作后人工治理状态"`
    CreatedAt    time.Time `gorm:"column:created_at;not null;comment:创建时间"`
}

func (DLQOperationLog) TableName() string {
	return "dlq_operation_log"
}
