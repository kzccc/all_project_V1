package model

import "time"

// ConversationSequence 保存每条会话流当前已持久化的最大 session_seq，高压下避免反复扫消息大表。
type ConversationSequence struct {
	ConversationKey string    `gorm:"column:conversation_key;primaryKey;type:varchar(64);comment:会话顺序作用域"`
	LastSeq         int64     `gorm:"column:last_seq;not null;comment:当前已知最大会话序号"`
	CreatedAt       time.Time `gorm:"column:created_at;not null;comment:创建时间"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null;comment:更新时间"`
}

// TableName 返回 ConversationSequence 对应的表名。
func (ConversationSequence) TableName() string {
	return "conversation_sequence"
}
