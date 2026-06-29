package chat

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/model"
	"echo_chat_server/internal/observability"
	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/zlog"
)

var conversationSeqInitialized sync.Map

var conversationSequenceTracker = newConversationSequenceHighWaterTracker()

func init() {
	conversationSequenceTracker.start()
}

// buildConversationSequenceScope 返回消息所属聊天流的顺序作用域。
// 单聊使用排序后的双方用户 ID，保证同一对用户共享一套顺序号；
// 群聊直接使用群 ID，保证同一群共享一套顺序号。
func buildConversationSequenceScope(sendID, receiveID string) string {
	return model.BuildConversationKey(sendID, receiveID)
}

func loadConversationMaxSessionSeq(sendID, receiveID string) (int64, error) {
	var maxSeq sql.NullInt64
	query := dao.GormDB.Model(&model.Message{}).Select("MAX(session_seq)")
	scope := buildConversationSequenceScope(sendID, receiveID)
	if err := query.Where("conversation_key = ?", scope).Scan(&maxSeq).Error; err != nil {
		return 0, err
	}
	if !maxSeq.Valid {
		return 0, nil
	}
	return maxSeq.Int64, nil
}

func upsertConversationHighWater(scope string, seq int64) error {
	entry := model.ConversationSequence{
		ConversationKey: scope,
		LastSeq:         seq,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	return dao.GormDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "conversation_key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_seq":   gorm.Expr("GREATEST(last_seq, VALUES(last_seq))"),
			"updated_at": gorm.Expr("VALUES(updated_at)"),
		}),
	}).Create(&entry).Error
}

func loadConversationPersistedSeq(scope, sendID, receiveID string) (int64, error) {
	highWaterSeq := int64(0)
	var seq model.ConversationSequence
	if err := dao.GormDB.Where("conversation_key = ?", scope).Take(&seq).Error; err == nil {
		highWaterSeq = seq.LastSeq
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	// 兼容已有历史会话：恢复起点不再只信高水位表，而是和 message 表当前真实最大 seq 取更大值。
	// 这样即使高水位 flush 落后，只要消息已经真实落库，恢复时也不会把 floor 拉回去。
	messageMaxSeq, err := loadConversationMaxSessionSeq(sendID, receiveID)
	if err != nil {
		return 0, err
	}
	floor, recoverySource := resolveConversationRecoveryFloor(highWaterSeq, messageMaxSeq)
	if err := upsertConversationHighWater(scope, floor); err != nil {
		return 0, err
	}
	observability.ObserveConversationSeqRecovery(recoverySource)
	zlog.Info(
		"session_seq.recovery_floor_resolved",
		zap.String("event", "session_seq.recovery_floor_resolved"),
		zap.String("module", "chat.session_seq"),
		zap.String("conversation_key", scope),
		zap.Int64("high_water_seq", highWaterSeq),
		zap.Int64("message_max_seq", messageMaxSeq),
		zap.Int64("recovery_floor", floor),
		zap.String("source", recoverySource),
	)
	return floor, nil
}

func resolveConversationRecoveryFloor(highWaterSeq, messageMaxSeq int64) (int64, string) {
	switch {
	case highWaterSeq == 0 && messageMaxSeq == 0:
		return 0, "empty"
	case highWaterSeq == 0:
		return messageMaxSeq, "message_table_only"
	case messageMaxSeq == 0:
		return highWaterSeq, "high_water_only"
	case messageMaxSeq > highWaterSeq:
		return messageMaxSeq, "message_table_catchup"
	case highWaterSeq > messageMaxSeq:
		return highWaterSeq, "high_water_ahead"
	default:
		return highWaterSeq, "equal"
	}
}

// nextMessageSessionSeq 返回当前消息在所属聊天流中的下一个顺序号。
func nextMessageSessionSeq(sendID, receiveID string) (int64, error) {
	value, _, err := nextMessageSessionSeqDetail(sendID, receiveID)
	return value, err
}

func nextMessageSessionSeqDetail(sendID, receiveID string) (int64, string, error) {
	scope := buildConversationSequenceScope(sendID, receiveID)
	redisKey := "message_session_seq_" + scope

	if _, initialized := conversationSeqInitialized.Load(scope); initialized {
		value, err := myredis.IncrKey(redisKey)
		if err != nil {
			return 0, "", err
		}
		if value == 1 {
			floor, err := loadConversationPersistedSeq(scope, sendID, receiveID)
			if err != nil {
				return 0, "", err
			}
			value, err = myredis.EnsureMinAndIncr(redisKey, floor)
			if err != nil {
				return 0, "", err
			}
			conversationSequenceTracker.record(scope, value)
			return value, "redis_floor_recovery", nil
		}
		conversationSequenceTracker.record(scope, value)
		return value, "hot_path", nil
	}

	floor, err := loadConversationPersistedSeq(scope, sendID, receiveID)
	if err != nil {
		return 0, "", err
	}
	value, err := myredis.InitFloorAndIncr(redisKey, floor)
	if err != nil {
		return 0, "", err
	}
	conversationSeqInitialized.Store(scope, struct{}{})
	conversationSequenceTracker.record(scope, value)
	return value, "cold_start", nil
}

// BackfillMessageSessionSeq 为历史消息补齐 session_seq，保证旧数据也能按统一顺序展示。
func BackfillMessageSessionSeq() error {
	var messages []model.Message
	if err := dao.GormDB.Where("session_seq = 0").Order("created_at ASC").Order("id ASC").Find(&messages).Error; err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}

	counters := make(map[string]int64, len(messages))
	for _, message := range messages {
		scope := buildConversationSequenceScope(message.SendId, message.ReceiveId)
		if _, ok := counters[scope]; !ok {
			floor, err := loadConversationPersistedSeq(scope, message.SendId, message.ReceiveId)
			if err != nil {
				return err
			}
			counters[scope] = floor
		}
		counters[scope]++
		if err := dao.GormDB.Model(&model.Message{}).Where("id = ?", message.Id).Update("session_seq", counters[scope]).Error; err != nil {
			return err
		}
		conversationSequenceTracker.record(scope, counters[scope])
	}
	return nil
}

type conversationSequenceHighWaterTracker struct {
	mu      sync.Mutex
	pending map[string]int64
}

func newConversationSequenceHighWaterTracker() *conversationSequenceHighWaterTracker {
	return &conversationSequenceHighWaterTracker{
		pending: make(map[string]int64),
	}
}

func (t *conversationSequenceHighWaterTracker) start() {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			t.flush()
		}
	}()
}

func (t *conversationSequenceHighWaterTracker) record(scope string, seq int64) {
	t.mu.Lock()
	if current, ok := t.pending[scope]; !ok || seq > current {
		t.pending[scope] = seq
	}
	t.mu.Unlock()
}

func (t *conversationSequenceHighWaterTracker) flush() {
	t.mu.Lock()
	if len(t.pending) == 0 {
		t.mu.Unlock()
		return
	}
	entries := make([]model.ConversationSequence, 0, len(t.pending))
	now := time.Now()
	for scope, seq := range t.pending {
		entries = append(entries, model.ConversationSequence{
			ConversationKey: scope,
			LastSeq:         seq,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}
	t.pending = make(map[string]int64)
	t.mu.Unlock()

	if err := dao.GormDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "conversation_key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_seq":   gorm.Expr("GREATEST(last_seq, VALUES(last_seq))"),
			"updated_at": gorm.Expr("VALUES(updated_at)"),
		}),
	}).CreateInBatches(entries, 128).Error; err != nil {
		zlog.Error(err.Error())
		t.mu.Lock()
		for _, entry := range entries {
			if current, ok := t.pending[entry.ConversationKey]; !ok || entry.LastSeq > current {
				t.pending[entry.ConversationKey] = entry.LastSeq
			}
		}
		t.mu.Unlock()
	}
}

func ResetConversationSequenceBenchState() {
	conversationSeqInitialized = sync.Map{}
}

func FlushConversationSequenceBenchState() error {
	conversationSequenceTracker.flush()
	return nil
}
