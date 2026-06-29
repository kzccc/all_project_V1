package respond

import "time"

type DLQMessageItem struct {
	ID              int64      `json:"id"`
	MessageID       string     `json:"message_id"`
	ConversationKey string     `json:"conversation_key"`
	SessionSeq      int64      `json:"session_seq"`
	Topic           string     `json:"topic"`
	PartitionID     int32      `json:"partition_id"`
	OffsetID        int64      `json:"offset_id"`
	Stage           string     `json:"stage"`
	ErrorCode       string     `json:"error_code"`
	LastError       string     `json:"last_error"`
	FailureType     string     `json:"failure_type"`
	HandleType      string     `json:"handle_type"`
	Status          string     `json:"status"`
	ManualStatus    string     `json:"manual_status"`
	CloseReason     string     `json:"close_reason"`
	AttemptCount    int        `json:"attempt_count"`
	MaxAttemptCount int        `json:"max_attempt_count"`
	NextRetryAt     *time.Time `json:"next_retry_at"`
	Assignee        string     `json:"assignee"`
	ClaimedAt       *time.Time `json:"claimed_at"`
	Remark          string     `json:"remark"`
	FirstFailedAt   time.Time  `json:"first_failed_at"`
	LastFailedAt    time.Time  `json:"last_failed_at"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type DLQMessageDetail struct {
	DLQMessageItem
	RawPayload      string `json:"raw_payload"`
	PayloadSnapshot string `json:"payload_snapshot"`
	ContextSnapshot string `json:"context_snapshot"`
}

type DLQListRespond struct {
	Total int64            `json:"total"`
	List  []DLQMessageItem `json:"list"`
}

type DLQStageStat struct {
	Stage      string `json:"stage"`
	Total      int64  `json:"total"`
	Pending    int64  `json:"pending"`
	Retrying   int64  `json:"retrying"`
	Manual     int64  `json:"manual"`
	Done       int64  `json:"done"`
	Open       int64  `json:"open"`
	InProgress int64  `json:"in_progress"`
	Closed     int64  `json:"closed"`
}

type DLQStatsRespond struct {
	Total            int64          `json:"total"`
	AutoPending      int64          `json:"auto_pending"`
	AutoRetrying     int64          `json:"auto_retrying"`
	ManualTotal      int64          `json:"manual_total"`
	DoneTotal        int64          `json:"done_total"`
	ManualOpen       int64          `json:"manual_open"`
	ManualInProgress int64          `json:"manual_in_progress"`
	ManualClosed     int64          `json:"manual_closed"`
	StageStats       []DLQStageStat `json:"stage_stats"`
}

type DLQOperationLogItem struct {
	ID                 int64     `json:"id"`
	DLQID              int64     `json:"dlq_id"`
	Action             string    `json:"action"`
	Operator           string    `json:"operator"`
	Remark             string    `json:"remark"`
	BeforeManualStatus string    `json:"before_manual_status"`
	AfterManualStatus  string    `json:"after_manual_status"`
	CreatedAt          time.Time `json:"created_at"`
}
