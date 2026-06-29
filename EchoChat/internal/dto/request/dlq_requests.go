package request

type DLQListRequest struct {
	MessageID       string `json:"message_id"`
	ConversationKey string `json:"conversation_key"`
	Stage           string `json:"stage"`
	ErrorCode       string `json:"error_code"`
	FailureType     string `json:"failure_type"`
	HandleType      string `json:"handle_type"`
	Status          string `json:"status"`
	ManualStatus    string `json:"manual_status"`
	Page            int    `json:"page"`
	PageSize        int    `json:"page_size"`
}

type DLQIDRequest struct {
	ID int64 `json:"id"`
}

type DLQClaimRequest struct {
	ID int64 `json:"id"`
}

type DLQReopenRequest struct {
	ID int64 `json:"id"`
}

type DLQCloseRequest struct {
	ID          int64  `json:"id"`
	CloseReason string `json:"close_reason"`
	Remark      string `json:"remark"`
}

type DLQRemarkRequest struct {
	ID     int64  `json:"id"`
	Remark string `json:"remark"`
}
