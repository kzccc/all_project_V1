package model

import "time"

const (
	DLQHandleTypeAuto   = "auto"
	DLQHandleTypeManual = "manual"

	DLQFailureTypeTemporary = "temporary"
	DLQFailureTypePermanent = "permanent"

	DLQStatusPending  = "pending"
	DLQStatusRetrying = "retrying"
	DLQStatusDone     = "done"
	DLQStatusManual   = "manual"

	DLQManualStatusOpen       = "open"
	DLQManualStatusInProgress = "in_progress"
	DLQManualStatusClosed     = "closed"

	DLQCloseReasonDiscarded       = "discarded"
	DLQCloseReasonExternallyFixed = "externally_fixed"
	DLQCloseReasonExpected        = "expected"
	DLQCloseReasonMergedIncident  = "merged_into_incident"
)

// DLQMessage 保存一条失败治理记录。
type DLQMessage struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement;comment:主键"`
	MessageID       string     `gorm:"column:message_id;type:varchar(64);not null;index:idx_dlq_message_id;comment:业务消息ID"`
	ConversationKey string     `gorm:"column:conversation_key;type:varchar(128);not null;default:'';index:idx_dlq_conversation_key;comment:会话标识"`
	SessionSeq      int64      `gorm:"column:session_seq;not null;default:0;comment:会话内顺序号"`
	RawPayload      string     `gorm:"column:raw_payload;type:longtext;not null;comment:原始消息体"`
	PayloadSnapshot string     `gorm:"column:payload_snapshot;type:json;comment:业务快照"`
	ContextSnapshot string     `gorm:"column:context_snapshot;type:json;comment:上下文快照"`
	Topic           string     `gorm:"column:topic;type:varchar(128);not null;uniqueIndex:uniq_dlq_source,priority:1;comment:Kafka topic"`
	PartitionID     int32      `gorm:"column:partition_id;not null;uniqueIndex:uniq_dlq_source,priority:2;comment:Kafka partition"`
	OffsetID        int64      `gorm:"column:offset_id;not null;uniqueIndex:uniq_dlq_source,priority:3;comment:Kafka offset"`
	Stage           string     `gorm:"column:stage;type:varchar(64);not null;uniqueIndex:uniq_dlq_source,priority:4;index:idx_dlq_stage_status,priority:1;comment:失败阶段"`
	ErrorCode       string     `gorm:"column:error_code;type:varchar(128);not null;comment:结构化错误码"`
	LastError       string     `gorm:"column:last_error;type:text;not null;comment:最后错误信息"`
	FailureType     string     `gorm:"column:failure_type;type:varchar(16);not null;comment:失败类型"`
	HandleType      string     `gorm:"column:handle_type;type:varchar(16);not null;comment:治理方式"`
	Status          string     `gorm:"column:status;type:varchar(16);not null;index:idx_dlq_status_next_retry,priority:1;index:idx_dlq_stage_status,priority:2;comment:自动治理状态"`
	ManualStatus    string     `gorm:"column:manual_status;type:varchar(16);not null;default:'open';comment:人工治理状态"`
	CloseReason     string     `gorm:"column:close_reason;type:varchar(32);comment:人工关闭原因"`
	AttemptCount    int        `gorm:"column:attempt_count;not null;default:0;comment:已自动治理次数"`
	MaxAttemptCount int        `gorm:"column:max_attempt_count;not null;default:0;comment:最大自动治理次数"`
	NextRetryAt     *time.Time `gorm:"column:next_retry_at;index:idx_dlq_status_next_retry,priority:2;comment:下次重试时间"`
	Assignee        string     `gorm:"column:assignee;type:varchar(64);not null;default:'';comment:当前处理人"`
	ClaimedAt       *time.Time `gorm:"column:claimed_at;comment:人工接手时间"`
	Remark          string     `gorm:"column:remark;type:text;comment:人工备注"`
	FirstFailedAt   time.Time  `gorm:"column:first_failed_at;not null;comment:首次失败时间"`
	LastFailedAt    time.Time  `gorm:"column:last_failed_at;not null;comment:最近失败时间"`
	ResolvedAt      *time.Time `gorm:"column:resolved_at;comment:处理完成时间"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;comment:创建时间"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null;comment:更新时间"`
}

func (DLQMessage) TableName() string {
	return "dlq_message"
}
