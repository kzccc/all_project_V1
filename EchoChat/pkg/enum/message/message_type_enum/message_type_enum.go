package message_type_enum

// 本文件定义 message type enum 相关的枚举值，供业务状态判断时复用。

const (
	Text = iota
	// 语音
	Voice
	// 文件
	File
	// 通话
	AudioOrVideo
)
