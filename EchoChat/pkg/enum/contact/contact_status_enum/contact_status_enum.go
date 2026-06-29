package contact_status_enum

// 本文件定义 contact status enum 相关的枚举值，供业务状态判断时复用。

const (
	NORMAL = iota
	BE_BLACK
	BLACK
	BE_DELETE
	DELETE
	SILENCE
	QUIT_GROUP
	KICK_OUT_GROUP
)
