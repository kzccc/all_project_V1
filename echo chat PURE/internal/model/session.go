package model

// 本文件实现 session 相关逻辑。

import (
	"database/sql"
	"gorm.io/gorm"
	"time"
)

// Session 定义了 `session` 表在代码中的映射结构。
type Session struct {
	// Id 自增id。
	Id int64 `gorm:"column:id;primaryKey;comment:自增id"`
	// Uuid 会话uuid。
	Uuid string `gorm:"column:uuid;uniqueIndex;type:char(20);comment:会话uuid"`
	// SendId 创建会话人id。
	SendId string `gorm:"column:send_id;Index;type:char(20);not null;comment:创建会话人id"`
	// ReceiveId 接受会话人id。
	ReceiveId string `gorm:"column:receive_id;Index;type:char(20);not null;comment:接受会话人id"`
	// ReceiveName 名称。
	ReceiveName string `gorm:"column:receive_name;type:varchar(20);not null;comment:名称"`
	// Avatar 头像。
	Avatar string `gorm:"column:avatar;type:char(255);default:default_avatar.png;not null;comment:头像"`
	// LastMessage 最新的消息。
	LastMessage string `gorm:"column:last_message;type:TEXT;comment:最新的消息"`
	// LastMessageAt 最近接收时间。
	LastMessageAt sql.NullTime `gorm:"column:last_message_at;type:datetime;comment:最近接收时间"`
	// CreatedAt 创建时间。
	CreatedAt time.Time `gorm:"column:created_at;Index;type:datetime;comment:创建时间"`
	// DeletedAt 删除时间。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;Index;type:datetime;comment:删除时间"`
}

// TableName 返回 `session` 模型对应的表名。
func (Session) TableName() string {
	return "session"
}
