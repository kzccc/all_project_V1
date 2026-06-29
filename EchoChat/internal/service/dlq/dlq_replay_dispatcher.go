package dlq

import (
	"encoding/json"
	"fmt"
	"time"

	"echo_chat_server/internal/model"
)

type ReplayResult struct {
	Success     bool
	Retryable   bool
	NextRetryAt *time.Time
	FinalError  error
}

func HandleDLQRecord(record *model.DLQMessage) ReplayResult {
	if record == nil {
		return ReplayResult{Success: false, Retryable: false, FinalError: fmt.Errorf("dlq record is nil")}
	}
	switch record.Stage {
	case "session_seq":
		return handleSessionSeqReplay(record)
	case "mysql_persist":
		return handleMysqlPersistReplay(record)
	case "websocket_dispatch":
		return handleWebsocketDispatchReplay(record)
	case "group_member_query":
		return handleGroupMemberQueryReplay(record)
	default:
		return ReplayResult{Success: false, Retryable: false, FinalError: fmt.Errorf("unsupported replay stage: %s", record.Stage)}
	}
}

func handleSessionSeqReplay(record *model.DLQMessage) ReplayResult {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(record.PayloadSnapshot), &payload); err != nil {
		return ReplayResult{Success: false, Retryable: false, FinalError: err}
	}
	nextRetryAt := time.Now().Add(10 * time.Second)
	return ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: fmt.Errorf("session_seq replay not implemented")}
}

func handleMysqlPersistReplay(record *model.DLQMessage) ReplayResult {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(record.PayloadSnapshot), &payload); err != nil {
		return ReplayResult{Success: false, Retryable: false, FinalError: err}
	}
	nextRetryAt := time.Now().Add(10 * time.Second)
	return ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: fmt.Errorf("mysql_persist replay not implemented")}
}

func handleWebsocketDispatchReplay(record *model.DLQMessage) ReplayResult {
	nextRetryAt := time.Now().Add(30 * time.Second)
	return ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: fmt.Errorf("websocket_dispatch replay not implemented")}
}

func handleGroupMemberQueryReplay(record *model.DLQMessage) ReplayResult {
	nextRetryAt := time.Now().Add(30 * time.Second)
	return ReplayResult{Success: false, Retryable: true, NextRetryAt: &nextRetryAt, FinalError: fmt.Errorf("group_member_query replay not implemented")}
}
