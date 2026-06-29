package model

// 本文件实现 message 相关逻辑。

import (
	"database/sql"
	"fmt"
	"time"
)

// BuildConversationKey 把单聊/群聊统一映射成稳定会话键。
func BuildConversationKey(sendID, receiveID string) string {
	if len(receiveID) > 0 && receiveID[0] == 'G' {
		return "group:" + receiveID
	}
	if sendID < receiveID {
		return fmt.Sprintf("user:%s:%s", sendID, receiveID)
	}
	return fmt.Sprintf("user:%s:%s", receiveID, sendID)
}

// Message 定义了 `message` 表在代码中的映射结构。
//
// 这张表是聊天系统最核心的消息持久化表，承担三类职责：
// 1. 保存单聊、群聊中的历史消息，支持后续消息列表查询与离线补拉；
// 2. 为文件消息、音视频信令等不同消息形态提供统一落库结构，避免前后端和业务层维护多套消息表；
// 3. 记录消息从“生成”到“成功投递”的关键状态，便于后续做消息回显、失败重试和状态追踪。
//
// 设计上，这个结构不是只为“文本聊天”服务，而是把多种消息类型抽象成一套通用消息模型：
// - 文本消息主要使用 Content；
// - 文件消息主要使用 Url / FileType / FileName / FileSize；
// - 音视频消息主要使用 AVdata；
// 这样可以保证后端分发链路、数据库表结构和前端消息渲染入口相对统一。
type Message struct {
	// Id 是数据库自增主键，只用于表内排序、主键约束和数据库内部管理。
	// 它不暴露给前端，也不作为业务侧消息唯一标识，因为业务层统一使用 Uuid。
	Id int64 `gorm:"column:id;primaryKey;comment:自增id"`
	// Uuid 是消息的业务唯一标识。
	// 它用于在后端、前端、WebSocket 回写和数据库之间稳定标识同一条消息，
	// 例如消息落库后，后端会根据这个字段把状态从“未发送”更新为“已发送”。
	Uuid string `gorm:"column:uuid;uniqueIndex;type:char(20);not null;comment:消息uuid"`
	// SessionId 表示这条消息所属的会话。
	// 单聊场景下，它对应某个用户会话；群聊场景下，它对应群会话。
	// 设计这个字段的目的是把“消息归属到哪个聊天上下文”明确下来，便于按会话维度查询历史消息。
	SessionId string `gorm:"column:session_id;type:char(20);not null;comment:会话uuid"`
	// Type 表示消息类型，用来决定这条消息应该如何解释和渲染。
	// 当前约定：
	// 0 表示文本消息；
	// 1 表示语音消息；
	// 2 表示文件消息；
	// 3 表示通话/音视频信令消息。
	// 这个字段是消息模型抽象的关键入口，后端分发和前端展示都会先看这个值。
	Type int8 `gorm:"column:type;not null;comment:消息类型，0.文本，1.语音，2.文件，3.通话"` // 通话不用存消息内容或者url
	// Content 保存文本类消息的正文内容。
	// 对文本消息来说，这里是核心字段；对文件或通话消息来说，这里通常为空，
	// 因为这些消息的主要信息不在文本内容里。
	Content string `gorm:"column:content;type:TEXT;comment:消息内容"`
	// Url 保存资源型消息的访问地址。
	// 常见场景是图片、文件、语音等需要通过 URL 获取资源的数据。
	// 文本消息通常不会使用这个字段，通话信令也一般不依赖它。
	Url string `gorm:"column:url;type:char(255);comment:消息url"`
	// SendId 是发送者的业务唯一标识。
	// 这个字段决定了消息是谁发出的，也是前端区分“我发送的消息”还是“对方发送的消息”的基础字段。
	SendId string `gorm:"column:send_id;type:char(20);not null;comment:发送者uuid"`
	// SendName 是发送消息时冗余保存的发送者昵称快照。
	// 这里做冗余存储的意图是：即使用户后续改了昵称，历史消息仍然可以按当时的展示信息回显，
	// 减少查用户表的压力，也避免历史记录展示不稳定。
	SendName string `gorm:"column:send_name;type:varchar(20);not null;comment:发送者昵称"`
	// SendAvatar 是发送消息时冗余保存的发送者头像快照。
	// 它和 SendName 的设计意图类似，都是为了让历史消息展示尽量自洽，
	// 同时降低每次拉取消息列表时再回表查询用户资料的成本。
	SendAvatar string `gorm:"column:send_avatar;type:varchar(255);not null;comment:发送者头像"`
	// ReceiveId 是接收对象的业务唯一标识。
	// 这个字段既可能指向单个用户，也可能指向一个群组，
	// 具体含义由业务约定和前缀规则共同决定，例如 U 开头表示用户，G 开头表示群。
	ReceiveId string `gorm:"column:receive_id;type:char(20);not null;comment:接受者uuid"`
	// ConversationKey 是统一的会话键。
	// 单聊用排序后的双方用户 ID，群聊直接用群 ID。
	// 历史查询统一按这个字段过滤，再按 SessionSeq 排序。
	ConversationKey string `gorm:"column:conversation_key;uniqueIndex:uniq_message_conversation_seq,priority:1;type:varchar(64);not null;default:'';comment:统一会话键"`
	// FileType 保存文件类消息的资源类型或扩展类型，例如 png、pdf、mp4 等。
	// 这个字段主要服务于前端展示和下载逻辑，帮助前端判断如何渲染或提示用户。
	FileType string `gorm:"column:file_type;type:char(10);comment:文件类型"`
	// FileName 保存文件类消息的原始文件名或展示名。
	// 它的作用是让前端在消息列表里可以直接展示“用户发了什么文件”，
	// 而不需要再从 URL 中反推文件名。
	FileName string `gorm:"column:file_name;type:varchar(50);comment:文件名"`
	// FileSize 保存文件大小的展示值。
	// 当前设计里它保存的是类似 10KB、2MB 这样的业务展示数据，而不是纯数字字节数，
	// 这样前端可直接展示，但代价是如果后续要做精确统计，可能还需要单独引入字节级字段。
	FileSize string `gorm:"column:file_size;type:char(20);comment:文件大小"`
	// Status 表示消息当前的投递状态。
	// 当前主要有两种状态：
	// 0 表示消息已生成并落库，但还没有确认成功写回前端；
	// 1 表示消息已经通过 WebSocket 成功写出，后端将其标记为“已发送”。
	// 这个字段的设计意图是把“消息创建成功”和“消息真正发出成功”区分开。
	Status int8 `gorm:"column:status;not null;comment:状态，0.未发送，1.已发送"`
	// SessionSeq 表示这条消息在当前聊天流中的顺序号。
	// 单聊场景下，同一对用户共享一套递增序号；群聊场景下，同一群共享一套递增序号。
	// 前端实时展示和历史查询都以这个字段作为主排序依据，避免仅靠 created_at 带来的时序抖动。
	SessionSeq int64 `gorm:"column:session_seq;not null;uniqueIndex:uniq_message_conversation_seq,priority:2;comment:会话内顺序号"`
	// CreatedAt 表示消息在服务端创建并落库的时间。
	// 它是历史消息排序、分页和前端时间展示的核心依据。
	CreatedAt time.Time `gorm:"column:created_at;not null;comment:创建时间"`
	// SendAt 表示消息完成发送的时间点。
	// 该字段使用 sql.NullTime，是因为消息刚创建时可能还未真正发送成功，
	// 因此需要允许这个时间为空，用来表达“尚未发送完成”这一状态。
	SendAt sql.NullTime `gorm:"column:send_at;comment:发送时间"`
	// AVdata 保存音视频或通话相关的信令数据。
	// 对通话消息来说，这里通常会存储序列化后的业务载荷，例如 start_call、reject_call、
	// WebRTC 相关的代理数据等。它的设计目的是在统一消息模型下兼容通话场景，
	// 避免为音视频信令再单独拆一套完全独立的消息表。
	AVdata string `gorm:"column:av_data;comment:通话传递数据"`
}

// TableName 返回 `message` 模型对应的表名。
func (Message) TableName() string {
	return "message"
}
