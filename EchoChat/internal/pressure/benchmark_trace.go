package pressure

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

const benchmarkMessagePrefix = "BENCH:"

type BenchmarkMessageMeta struct {
	RunID    string `json:"run_id"`
	Scenario string `json:"scenario"`
	BenchID  string `json:"bench_id"`
	SendTsMs int64  `json:"send_ts_ms"`
}

type BenchmarkTraceEvent struct {
	BenchID      string                 `json:"bench_id"`
	RunID        string                 `json:"run_id,omitempty"`
	Scenario     string                 `json:"scenario,omitempty"`
	Event        string                 `json:"event"`
	OccurredUnix int64                  `json:"occurred_unix_ns"`
	Fields       map[string]interface{} `json:"fields,omitempty"`
}

type benchmarkTraceStore struct {
	mu       sync.RWMutex
	metaByID map[string]BenchmarkMessageMeta
	events   []BenchmarkTraceEvent
}

var traceStore = &benchmarkTraceStore{
	metaByID: make(map[string]BenchmarkMessageMeta),
	events:   make([]BenchmarkTraceEvent, 0, 4096),
}

func parseBenchmarkPayload(content string) (BenchmarkMessageMeta, bool) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, benchmarkMessagePrefix) {
		return BenchmarkMessageMeta{}, false
	}
	var meta BenchmarkMessageMeta
	if err := json.Unmarshal([]byte(content[len(benchmarkMessagePrefix):]), &meta); err != nil {
		return BenchmarkMessageMeta{}, false
	}
	if meta.BenchID == "" {
		return BenchmarkMessageMeta{}, false
	}
	return meta, true
}

func ResetBenchmarkTrace() {
	traceStore.mu.Lock()
	traceStore.metaByID = make(map[string]BenchmarkMessageMeta)
	traceStore.events = traceStore.events[:0]
	traceStore.mu.Unlock()
}

func RegisterBenchmarkMessage(messageID string, content string) {
	if messageID == "" {
		return
	}
	meta, ok := parseBenchmarkPayload(content)
	if !ok {
		return
	}
	traceStore.mu.Lock()
	traceStore.metaByID[messageID] = meta
	traceStore.mu.Unlock()
}

func EnsureBenchmarkMessageMeta(messageID string, content string) bool {
	if messageID == "" {
		return false
	}
	meta, ok := parseBenchmarkPayload(content)
	if !ok {
		return false
	}
	traceStore.mu.Lock()
	if _, exists := traceStore.metaByID[messageID]; !exists {
		traceStore.metaByID[messageID] = meta
	}
	traceStore.mu.Unlock()
	return true
}

func ObserveBenchmarkEvent(messageID string, event string, fields map[string]interface{}) {
	if messageID == "" || event == "" {
		return
	}
	ObserveBenchmarkEventAt(messageID, event, time.Now(), fields)
}

func ObserveBenchmarkEventAt(messageID string, event string, occurredAt time.Time, fields map[string]interface{}) {
	if messageID == "" || event == "" {
		return
	}
	traceStore.mu.RLock()
	meta, ok := traceStore.metaByID[messageID]
	traceStore.mu.RUnlock()
	if !ok {
		return
	}
	traceStore.mu.Lock()
	traceStore.events = append(traceStore.events, BenchmarkTraceEvent{
		BenchID:      meta.BenchID,
		RunID:        meta.RunID,
		Scenario:     meta.Scenario,
		Event:        event,
		OccurredUnix: occurredAt.UnixNano(),
		Fields:       cloneTraceFields(fields),
	})
	traceStore.mu.Unlock()
}

func SnapshotBenchmarkTrace() map[string]interface{} {
	traceStore.mu.RLock()
	defer traceStore.mu.RUnlock()
	metas := make([]BenchmarkMessageMeta, 0, len(traceStore.metaByID))
	for _, meta := range traceStore.metaByID {
		metas = append(metas, meta)
	}
	events := make([]BenchmarkTraceEvent, len(traceStore.events))
	copy(events, traceStore.events)
	return map[string]interface{}{
		"messages": metas,
		"events":   events,
	}
}

func SnapshotBenchmarkMetrics() map[string]interface{} {
	traceStore.mu.RLock()
	defer traceStore.mu.RUnlock()
	return map[string]interface{}{
		"registered_messages": len(traceStore.metaByID),
		"event_count":         len(traceStore.events),
	}
}

func cloneTraceFields(fields map[string]interface{}) map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}
	cloned := make(map[string]interface{}, len(fields))
	for key, value := range fields {
		cloned[key] = value
	}
	return cloned
}
