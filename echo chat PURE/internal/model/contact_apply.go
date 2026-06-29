package model

// 本文件实现 contact apply 相关逻辑。

import (
	"gorm.io/gorm"
	"time"
)

// ContactApply 定义了 `contact_apply` 表在代码中的映射结构。
type ContactApply struct {
	// Id 自增id。
	Id int64 `gorm:"column:id;primaryKey;comment:自增id"`
	// Uuid 申请id。
	Uuid string `gorm:"column:uuid;uniqueIndex;type:char(20);comment:申请id"`
	// UserId 申请人id。
	UserId string `gorm:"column:user_id;index;type:char(20);not null;comment:申请人id"`
	// ContactId 被申请id。
	ContactId string `gorm:"column:contact_id;index;type:char(20);not null;comment:被申请id"`
	// ContactType 被申请类型，0.用户，1.群聊。
	ContactType int8 `gorm:"column:contact_type;not null;comment:被申请类型，0.用户，1.群聊"`
	// Status 申请状态，0.申请中，1.通过，2.拒绝，3.拉黑。
	Status int8 `gorm:"column:status;not null;comment:申请状态，0.申请中，1.通过，2.拒绝，3.拉黑"`
	// Message 申请信息。
	Message string `gorm:"column:message;type:varchar(100);comment:申请信息"`
	// LastApplyAt 最后申请时间。
	LastApplyAt time.Time `gorm:"column:last_apply_at;type:datetime;not null;comment:最后申请时间"`
	// DeletedAt 删除时间。
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index;type:datetime;comment:删除时间"`
}

// TableName 返回 `contact_apply` 模型对应的表名。
func (ContactApply) TableName() string {
	return "contact_apply"
}
