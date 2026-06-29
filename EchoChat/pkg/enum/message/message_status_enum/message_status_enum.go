package message_status_enum

// 本文件定义 message status enum 相关的枚举值，供业务状态判断时复用。

const (
	// 未发送
	Unsent = iota
	// 已发送
	Sent
)
