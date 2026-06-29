package chat

// 本文件实现 server 相关的实时消息链路与在线连接管理逻辑。

import (
	"context"
	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/dto/respond"
	"echo_chat_server/internal/model"
	"echo_chat_server/internal/pressure"
	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/enum/message/message_status_enum"
	"echo_chat_server/pkg/enum/message/message_type_enum"
	"echo_chat_server/pkg/zlog"
	"encoding/json"
	"errors"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	// Clients 保存当前 channel 模式下的在线连接。
	Clients map[string]*Client
	// mutex 保护在线连接表。
	mutex *sync.Mutex
	// Transmit 承接待转发的聊天消息。
	Transmit chan []byte // 转发通道
	// Login 承接新登录的客户端。
	Login chan *Client // 登录通道
	// Logout 承接待下线的客户端。
	Logout chan *Client // 退出登录通道
	// done 在服务进入停机阶段时关闭，用于通知各发送方停止继续投递消息。
	done chan struct{}
	// stopped 在 Start 主循环退出时关闭，供关机流程等待。
	stopped chan struct{}
	// shutdownOnce 保证停机流程只执行一次。
	shutdownOnce sync.Once
	// shuttingDown 标记服务是否已经进入停机阶段。
	shuttingDown atomic.Bool
}

// ChatServer 是基于内存 channel 的实时聊天服务单例。
var ChatServer *Server

// init 在包加载时完成当前模块的默认实例或运行期资源初始化。
func init() {
	if ChatServer == nil {
		ChatServer = &Server{
			Clients:  make(map[string]*Client),
			mutex:    &sync.Mutex{},
			Transmit: make(chan []byte, constants.CHANNEL_SIZE),
			Login:    make(chan *Client, constants.CHANNEL_SIZE),
			Logout:   make(chan *Client, constants.CHANNEL_SIZE),
			done:     make(chan struct{}),
			stopped:  make(chan struct{}),
		}
	}
}

// normalizePath 把前端或数据库里出现的资源地址统一规范化为“后端可稳定存储”的形式。
// 这个函数的核心原则是：
// “能安全规范化就规范化，不能确定时宁可保留原值，也不要把路径改坏或直接崩溃。”
func normalizePath(path string) string {
	// 空值直接返回，表示当前没有可用资源路径。
	if path == "" {
		return ""
	}
	// 已经是项目内部的静态资源相对路径，不需要再做任何裁剪。
	if strings.HasPrefix(path, "/static/") {
		return path
	}
	// 如果是完整 URL，说明它可能来自前端已经拼好的绝对地址，或者第三方外链。
	// 这里先尝试从中提取 /static/...，提取不到就保留原样。
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		// 对站内静态资源，统一裁成相对路径，避免把当前域名/端口写死进数据库。
		staticIndex := strings.Index(path, "/static/")
		if staticIndex >= 0 {
			return path[staticIndex:]
		}
		// 完整 URL 但不包含 /static/，通常表示第三方资源，不属于异常情况。
		// 压测 seed 也会大量使用这类头像地址，因此这里直接透传，避免制造无意义噪音日志。
		return path
	}
	// 非完整 URL 但包含 /static/ 时，也统一从 /static/ 开始截取。
	staticIndex := strings.Index(path, "/static/")
	if staticIndex >= 0 {
		return path[staticIndex:]
	}
	// 走到这里说明既不是空值、也不是站内相对路径、也不是可识别的站内绝对 URL。
	// 这类值我们记日志后原样返回，至少保持程序可继续运行，不因为下标越界而 panic。
	zlog.Error(
		"path.normalize.invalid",
		zap.String("event", "path.normalize.invalid"),
		zap.String("module", "chat.channel"),
		zap.String("raw_path", path),
	)
	return path
}

// Start 启动函数，Server端用主进程起，Client端可以用协程起
func (s *Server) Start() {
	defer close(s.stopped)
	for {
		select {
		case <-s.done:
			return
		case client := <-s.Login:
			{
				// 登录事件只维护在线表，不涉及数据库写入。
				s.mutex.Lock()
				s.Clients[client.Uuid] = client
				s.mutex.Unlock()
				if pressure.ShouldLogHotPath(client.Benchmark) {
					zlog.Info("ws.connection.login", client.wsFields(zap.String("event", "ws.connection.login"), zap.String("module", "chat.channel"))...)
				}
				if !client.Benchmark {
					err := client.writeText([]byte("欢迎来到EchoChat聊天服务器"))
					if err != nil {
						zlog.Error(err.Error())
					}
				}
			}

		case client := <-s.Logout:
			{
				// 登出事件会把连接从在线表移除，并向前端回写一个结束提示。
				s.mutex.Lock()
				delete(s.Clients, client.Uuid)
				s.mutex.Unlock()
				zlog.Info("ws.connection.logout", client.wsFields(zap.String("event", "ws.connection.logout"), zap.String("module", "chat.channel"))...)
				if err := client.writeText([]byte("已退出登录")); err != nil {
					zlog.Error(err.Error())
				}
			}

		case data := <-s.Transmit: //这里取出来的是字节切片
			{
				// 统一在服务端解析消息，便于同时完成落库、分发和缓存维护。
				//? 这里把字节切片反序列化解析转为结构体，方便后续处理。
				var chatMessageReq request.ChatMessageRequest
				if err := json.Unmarshal(data, &chatMessageReq); err != nil {
					zlog.Error(
						"message.decode.failed",
						zap.String("event", "message.decode.failed"),
						zap.String("module", "chat.channel"),
						zap.Int("payload_size", len(data)),
						zap.String("error", err.Error()),
					)
					continue
				}
				// log.Println("原消息为：", data, "反序列化后为：", chatMessageReq)
				//? 这里判断消息类型，如果是文本消息，则落库
				if chatMessageReq.Type == message_type_enum.Text {
					// 文本消息先落库，随后根据接收对象类型选择单聊或群聊回推逻辑。
					sessionSeq, err := nextMessageSessionSeq(chatMessageReq.SendId, chatMessageReq.ReceiveId)
					if err != nil {
						zlog.Error(err.Error())
						continue
					}
					message := model.Message{
						Uuid:       chatMessageReq.MessageId,
						SessionId:  chatMessageReq.SessionId,
						Type:       chatMessageReq.Type,
						Content:    chatMessageReq.Content,
						Url:        "",
						SendId:     chatMessageReq.SendId,
						SendName:   chatMessageReq.SendName,
						SendAvatar: chatMessageReq.SendAvatar,
						ReceiveId:  chatMessageReq.ReceiveId,
						ConversationKey: model.BuildConversationKey(
							chatMessageReq.SendId,
							chatMessageReq.ReceiveId,
						),
						FileSize:   "0B",
						FileType:   "",
						FileName:   "",
						Status:     message_status_enum.Unsent,
						SessionSeq: sessionSeq,
						CreatedAt:  time.Now(),
						AVdata:     "",
					}
					// 对SendAvatar去除前面/static之前的所有内容，防止ip前缀引入
					message.SendAvatar = normalizePath(message.SendAvatar)
					if res := dao.GormDB.Create(&message); res.Error != nil {
						zlog.Error(res.Error.Error())
						clearIngressIdempotencyPending(message.SendId, message.Uuid)
						continue
					}
					logMessagePersist("channel", &message)

					//? 进一步判断是否单聊
					if message.ReceiveId[0] == 'U' { // 发送给User
						//?构造响应体
						messageRsp := respond.GetMessageListRespond{
							MessageId:  message.Uuid,
							SendId:     message.SendId,
							SendName:   message.SendName,
							SendAvatar: chatMessageReq.SendAvatar,
							ReceiveId:  message.ReceiveId,
							Type:       message.Type,
							Content:    message.Content,
							Url:        message.Url,
							FileSize:   message.FileSize,
							FileName:   message.FileName,
							FileType:   message.FileType,
							SessionSeq: message.SessionSeq,
							CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
						}
						//?将响应体序列化
						jsonMessage, err := json.Marshal(messageRsp)
						if err != nil {
							zlog.Error(err.Error())
						}
						logMessageDispatch("channel", "user", &message, len(jsonMessage))
						//? 构造messageBack, 放入client.SendBack准备发送给接收方
						var messageBack = &MessageBack{
							Message: jsonMessage, //这个就是[]byte,序列化后的响应体
							Uuid:    message.Uuid,
						}
						//? 如果对方在线的话就将messageback放进client.SendBack
						s.mutex.Lock()
						if receiveClient, ok := s.Clients[message.ReceiveId]; ok {
							receiveClient.enqueueBack(messageBack)
						}
						sendClient := s.Clients[message.SendId]
						sendClient.enqueueBack(messageBack)
						//WebSocket 连接本身不区分“别人发来的消息”和“自己消息的回显”，
						// 前端是通过消息体里的 SendId 与当前登录用户 userId 做比较来判断方向；SendId == 当前用户 就是自己的回显，否则就是别人发来的消息。
						// 以此来决定消息显示在左边还是右边
						s.mutex.Unlock()

						//? 如果双方的聊天记录缓存已存在，则顺手把新消息追加进去。
						// 尝试从 Redis 中获取当前双方（发送方与接收方）的私聊消息列表缓存。
						// 缓存键格式为：message_list_{SendId}_{ReceiveId}
						var rspString string
						rspString, err = myredis.GetKeyNilIsErr("message_list_" + message.SendId + "_" + message.ReceiveId)

						if err == nil {
							// 如果缓存存在（err == nil），则将其反序列化为消息响应体切片。
							var rsp []respond.GetMessageListRespond
							if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
								zlog.Error(err.Error()) // 反序列化失败，记录错误日志
							}

							rsp, _ = appendUniqueUserCacheMessage("message_list_"+message.SendId+"_"+message.ReceiveId, rsp, messageRsp, "channel")

							// 将更新后的消息列表重新序列化为 JSON 字符串。
							rspByte, err := json.Marshal(rsp)
							if err != nil {
								zlog.Error(err.Error()) // 序列化失败，记录错误日志
							}

							// 将更新后的消息列表写回 Redis，设置过期时间（REDIS_TIMEOUT 分钟），
							// 确保缓存不会无限增长，并在一段时间无交互后自动清理。
							if err := myredis.SetKeyEx("message_list_"+message.SendId+"_"+message.ReceiveId, string(rspByte), time.Minute*constants.REDIS_TIMEOUT); err != nil {
								zlog.Error(err.Error()) // 写入 Redis 失败，记录错误日志
							} else {
								logMessageCacheUpdate("channel", "message_list_"+message.SendId+"_"+message.ReceiveId, len(rsp))
							}
						} else {
							// 如果缓存不存在（即返回 redis.Nil），属于正常情况，无需处理。
							// 但如果返回的是其他非 Nil 错误（如网络、连接问题），则记录异常。
							if !errors.Is(err, redis.Nil) {
								zlog.Error(err.Error())
							}
						}
						completeIngressIdempotencyResult(message.SendId, message.Uuid, mustMarshalIngressReplayPayload(messageRsp))

					} else if message.ReceiveId[0] == 'G' { // 发送给Group
						// 群聊场景会查出群成员，并对在线成员逐个回推。
						//?构造响应体
						messageRsp := respond.GetGroupMessageListRespond{
							MessageId:  message.Uuid,                                    // 消息唯一标识，前端和缓存层都用它做幂等
							SendId:     message.SendId,                                  // 发送者 ID，前端据此判断是谁发的
							SendName:   message.SendName,                                // 发送者昵称，用于群聊气泡头部展示
							SendAvatar: chatMessageReq.SendAvatar,                       // 发送者头像，前端用于展示头像
							ReceiveId:  message.ReceiveId,                               // 接收群组 ID，标识这条消息属于哪个群
							Type:       message.Type,                                    // 消息类型，决定前端按文本/文件/通话等方式渲染
							Content:    message.Content,                                 // 文本内容，文本消息时主要展示这个字段
							Url:        message.Url,                                     // 资源地址，文件或媒体消息时使用
							FileSize:   message.FileSize,                                // 文件大小，前端用于文件消息说明
							FileName:   message.FileName,                                // 文件名，前端用于文件消息展示
							FileType:   message.FileType,                                // 文件类型，前端可据此决定图标或预览方式
							SessionSeq: message.SessionSeq,                              // 同一聊天流中的顺序号，用于前端稳定排序
							CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"), // 服务端生成的展示时间
						}
						//? 将响应体序列化
						jsonMessage, err := json.Marshal(messageRsp)
						if err != nil {
							zlog.Error(err.Error())
						}
						logMessageDispatch("channel", "group", &message, len(jsonMessage))
						//? 构造messageBack, 准备放入client.SendBack准备发送给接收方
						var messageBack = &MessageBack{
							Message: jsonMessage,
							Uuid:    message.Uuid,
						}
						//? 通过群的uuid将群成员取出来
						var group model.GroupInfo
						if res := dao.GormDB.Where("uuid = ?", message.ReceiveId).First(&group); res.Error != nil {
							zlog.Error(res.Error.Error())
						}
						var members []string
						if err := json.Unmarshal(group.Members, &members); err != nil {
							zlog.Error(err.Error())
						}
						//? 这里相当于对于每一个群组成员执行单聊中发送和回显的操作
						s.mutex.Lock()
						for _, member := range members {
							if member != message.SendId {
								if receiveClient, ok := s.Clients[member]; ok {
									receiveClient.enqueueBack(messageBack)
								}
							} else {
								sendClient := s.Clients[message.SendId]
								sendClient.enqueueBack(messageBack)
							}
						}
						s.mutex.Unlock()

						//? 群消息列表缓存命中时同步追加，避免下次查询回源数据库。
						var rspString string
						rspString, err = myredis.GetKeyNilIsErr("group_messagelist_" + message.ReceiveId)
						if err == nil {
							var rsp []respond.GetGroupMessageListRespond
							if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
								zlog.Error(err.Error())
							}
							rsp, _ = appendUniqueGroupCacheMessage("group_messagelist_"+message.ReceiveId, rsp, messageRsp, "channel")
							rspByte, err := json.Marshal(rsp)
							if err != nil {
								zlog.Error(err.Error())
							}
							if err := myredis.SetKeyEx("group_messagelist_"+message.ReceiveId, string(rspByte), time.Minute*constants.REDIS_TIMEOUT); err != nil {
								zlog.Error(err.Error())
							} else {
								logMessageCacheUpdate("channel", "group_messagelist_"+message.ReceiveId, len(rsp))
							}
						} else {
							if !errors.Is(err, redis.Nil) {
								zlog.Error(err.Error())
							}
						}
						completeIngressIdempotencyResult(message.SendId, message.Uuid, mustMarshalIngressReplayPayload(messageRsp))
					}
				} else if chatMessageReq.Type == message_type_enum.File {
					//? 文件消息与文本消息流程基本一致，只是展示字段换成文件信息。
					sessionSeq, err := nextMessageSessionSeq(chatMessageReq.SendId, chatMessageReq.ReceiveId)
					if err != nil {
						zlog.Error(err.Error())
						continue
					}
					message := model.Message{
						Uuid:       chatMessageReq.MessageId,
						SessionId:  chatMessageReq.SessionId,
						Type:       chatMessageReq.Type,
						Content:    "",
						Url:        chatMessageReq.Url,
						SendId:     chatMessageReq.SendId,
						SendName:   chatMessageReq.SendName,
						SendAvatar: chatMessageReq.SendAvatar,
						ReceiveId:  chatMessageReq.ReceiveId,
						ConversationKey: model.BuildConversationKey(
							chatMessageReq.SendId,
							chatMessageReq.ReceiveId,
						),
						FileSize:   chatMessageReq.FileSize,
						FileType:   chatMessageReq.FileType,
						FileName:   chatMessageReq.FileName,
						Status:     message_status_enum.Unsent,
						SessionSeq: sessionSeq,
						CreatedAt:  time.Now(),
						AVdata:     "",
					}
					//? 对SendAvatar去除前面/static之前的所有内容，防止ip前缀引入
					message.SendAvatar = normalizePath(message.SendAvatar)
					if res := dao.GormDB.Create(&message); res.Error != nil {
						zlog.Error(res.Error.Error())
						clearIngressIdempotencyPending(message.SendId, message.Uuid)
						continue
					}
					logMessagePersist("channel", &message)
					//? 进一步判断是否单聊
					if message.ReceiveId[0] == 'U' { // 发送给User
						//?构造响应体
						messageRsp := respond.GetMessageListRespond{
							MessageId:  message.Uuid,
							SendId:     message.SendId,
							SendName:   message.SendName,
							SendAvatar: chatMessageReq.SendAvatar,
							ReceiveId:  message.ReceiveId,
							Type:       message.Type,
							Content:    message.Content,
							Url:        message.Url,
							FileSize:   message.FileSize,
							FileName:   message.FileName,
							FileType:   message.FileType,
							SessionSeq: message.SessionSeq,
							CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
						}
						//? 将响应体序列化
						jsonMessage, err := json.Marshal(messageRsp)
						if err != nil {
							zlog.Error(err.Error())
						}
						logMessageDispatch("channel", "user", &message, len(jsonMessage))
						//? 构造messageBack, 准备放入client.SendBack准备发送给接收方
						var messageBack = &MessageBack{
							Message: jsonMessage,
							Uuid:    message.Uuid,
						}
						//? 通过单聊的uuid将单聊成员取出来发送和回显
						s.mutex.Lock()
						if receiveClient, ok := s.Clients[message.ReceiveId]; ok {
							receiveClient.enqueueBack(messageBack)
						}
						// 因为send_id肯定在线，所以这里在后端进行在线回显message，其实优化的话前端可以直接回显
						// 问题在于前后端的req和rsp结构不同，前端存储message的messageList不能存req，只能存rsp
						// 所以这里后端进行回显，前端不回显
						sendClient := s.Clients[message.SendId]
						sendClient.enqueueBack(messageBack)
						s.mutex.Unlock()

						//? 单聊文件消息同样会尝试刷新双方消息缓存。
						var rspString string
						rspString, err = myredis.GetKeyNilIsErr("message_list_" + message.SendId + "_" + message.ReceiveId)
						if err == nil {
							var rsp []respond.GetMessageListRespond
							if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
								zlog.Error(err.Error())
							}
							rsp, _ = appendUniqueUserCacheMessage("message_list_"+message.SendId+"_"+message.ReceiveId, rsp, messageRsp, "channel")
							rspByte, err := json.Marshal(rsp)
							if err != nil {
								zlog.Error(err.Error())
							}
							if err := myredis.SetKeyEx("message_list_"+message.SendId+"_"+message.ReceiveId, string(rspByte), time.Minute*constants.REDIS_TIMEOUT); err != nil {
								zlog.Error(err.Error())
							} else {
								logMessageCacheUpdate("channel", "message_list_"+message.SendId+"_"+message.ReceiveId, len(rsp))
							}
						} else {
							if !errors.Is(err, redis.Nil) {
								zlog.Error(err.Error())
							}
						}
						completeIngressIdempotencyResult(message.SendId, message.Uuid, mustMarshalIngressReplayPayload(messageRsp))
					} else {
						//? 先构造群聊文件消息的响应体，这个结构是前端 messageList 真正要消费的数据格式
						messageRsp := respond.GetGroupMessageListRespond{
							MessageId:  message.Uuid,
							SendId:     message.SendId,
							SendName:   message.SendName,
							SendAvatar: chatMessageReq.SendAvatar,
							ReceiveId:  message.ReceiveId,
							Type:       message.Type,
							Content:    message.Content,
							Url:        message.Url,
							FileSize:   message.FileSize,
							FileName:   message.FileName,
							FileType:   message.FileType,
							SessionSeq: message.SessionSeq,
							CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
						}

						//? 再把响应体序列化成 jsonMessage，后面无论是 websocket 推送还是封装到 MessageBack 都要用这份 []byte 数据
						jsonMessage, err := json.Marshal(messageRsp)
						if err != nil {
							zlog.Error(err.Error())
						}
						logMessageDispatch("channel", "group", &message, len(jsonMessage))

						//? 构造 messageBack，Message 字段里放的是准备通过 websocket 写回前端的序列化结果，
						//? Uuid 则对应数据库里的消息 uuid，后面写回成功后可以据此更新消息状态
						var messageBack = &MessageBack{
							Message: jsonMessage,
							Uuid:    message.Uuid,
						}

						//? 查出当前群的成员列表，后面服务端会遍历成员，把这条群消息推送给所有在线成员
						var group model.GroupInfo
						if res := dao.GormDB.Where("uuid = ?", message.ReceiveId).First(&group); res.Error != nil {
							zlog.Error(res.Error.Error())
						}
						var members []string
						if err := json.Unmarshal(group.Members, &members); err != nil {
							zlog.Error(err.Error())
						}

						//? 遍历群成员时，只有在线成员才会在 s.Clients 里找到对应连接；
						//? 找到了就将 messageBack 放进对方或自己的回写通道，等待各自的 Write() 协程写给前端
						s.mutex.Lock()
						for _, member := range members {
							if member != message.SendId {
								if receiveClient, ok := s.Clients[member]; ok {
									receiveClient.enqueueBack(messageBack)
								}
							} else {
								sendClient := s.Clients[message.SendId]
								sendClient.enqueueBack(messageBack)
							}
						}
						s.mutex.Unlock()

						//? 最后尝试刷新 Redis 里的群消息缓存：
						//? 如果这个群的消息列表缓存已经存在，就先取出来反序列化，再把新消息 append 进去并重新写回，
						//? 这样下次前端拉群消息列表时就可以优先命中缓存，减少回源数据库的开销
						var rspString string
						rspString, err = myredis.GetKeyNilIsErr("group_messagelist_" + message.ReceiveId)
						if err == nil {
							var rsp []respond.GetGroupMessageListRespond
							if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
								zlog.Error(err.Error())
							}
							rsp, _ = appendUniqueGroupCacheMessage("group_messagelist_"+message.ReceiveId, rsp, messageRsp, "channel")
							rspByte, err := json.Marshal(rsp)
							if err != nil {
								zlog.Error(err.Error())
							}
							if err := myredis.SetKeyEx("group_messagelist_"+message.ReceiveId, string(rspByte), time.Minute*constants.REDIS_TIMEOUT); err != nil {
								zlog.Error(err.Error())
							} else {
								logMessageCacheUpdate("channel", "group_messagelist_"+message.ReceiveId, len(rsp))
							}
						} else {
							if !errors.Is(err, redis.Nil) {
								zlog.Error(err.Error())
							}
						}
						completeIngressIdempotencyResult(message.SendId, message.Uuid, mustMarshalIngressReplayPayload(messageRsp))
					}
				} else if chatMessageReq.Type == message_type_enum.AudioOrVideo {
					// 音视频消息主要承载信令数据，只有部分事件需要持久化留痕。
					var avData request.AVData
					if err := json.Unmarshal([]byte(chatMessageReq.AVdata), &avData); err != nil {
						zlog.Error(err.Error())
					}
					sessionSeq, err := nextMessageSessionSeq(chatMessageReq.SendId, chatMessageReq.ReceiveId)
					if err != nil {
						zlog.Error(err.Error())
						continue
					}
					//log.Println(avData)
					message := model.Message{
						Uuid:       chatMessageReq.MessageId,
						SessionId:  chatMessageReq.SessionId,
						Type:       chatMessageReq.Type,
						Content:    "",
						Url:        "",
						SendId:     chatMessageReq.SendId,
						SendName:   chatMessageReq.SendName,
						SendAvatar: chatMessageReq.SendAvatar,
						ReceiveId:  chatMessageReq.ReceiveId,
						ConversationKey: model.BuildConversationKey(
							chatMessageReq.SendId,
							chatMessageReq.ReceiveId,
						),
						FileSize:   "",
						FileType:   "",
						FileName:   "",
						Status:     message_status_enum.Unsent,
						SessionSeq: sessionSeq,
						CreatedAt:  time.Now(),
						AVdata:     chatMessageReq.AVdata,
					}
					if avData.MessageId == "PROXY" && (avData.Type == "start_call" || avData.Type == "receive_call" || avData.Type == "reject_call") {
						// 仅把关键通话状态变化写库，避免 ICE candidate 这类高频信令刷爆消息表。
						message.SendAvatar = normalizePath(message.SendAvatar)
						if res := dao.GormDB.Create(&message); res.Error != nil {
							zlog.Error(res.Error.Error())
							clearIngressIdempotencyPending(message.SendId, message.Uuid)
							continue
						}
						logMessagePersist("channel", &message)
					}

					if chatMessageReq.ReceiveId[0] == 'U' { // 发送给User
						// 如果能找到ReceiveId，说明在线，可以发送，否则存表后跳过
						// 因为在线的时候是通过websocket更新消息记录的，离线后通过存表，登录时只调用一次数据库操作
						// 切换chat对象后，前端的messageList也会改变，获取messageList从第二次就是从redis中获取
						messageRsp := respond.AVMessageRespond{
							MessageId:  message.Uuid,
							SendId:     message.SendId,
							SendName:   message.SendName,
							SendAvatar: message.SendAvatar,
							ReceiveId:  message.ReceiveId,
							Type:       message.Type,
							Content:    message.Content,
							Url:        message.Url,
							FileSize:   message.FileSize,
							FileName:   message.FileName,
							FileType:   message.FileType,
							SessionSeq: message.SessionSeq,
							CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
							AVdata:     message.AVdata,
						}
						jsonMessage, err := json.Marshal(messageRsp)
						if err != nil {
							zlog.Error(err.Error())
						}
						logMessageDispatch("channel", "user", &message, len(jsonMessage))
						var messageBack = &MessageBack{
							Message: jsonMessage,
							Uuid:    message.Uuid,
						}
						s.mutex.Lock()
						if receiveClient, ok := s.Clients[message.ReceiveId]; ok {
							receiveClient.enqueueBack(messageBack)
						}
						// 通话信令不对发送方做回显，否则前端会重复收到 start_call 等事件。
						//sendClient := s.Clients[message.SendId]
						//sendClient.SendBack <- messageBack
						s.mutex.Unlock()
						completeIngressIdempotencyResult(message.SendId, message.Uuid, mustMarshalIngressReplayPayload(messageRsp))
					}
				}

			}
		}
	}
}

func (s *Server) IsShuttingDown() bool {
	return s.shuttingDown.Load()
}

func (s *Server) snapshotAndClearClients() []*Client {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	clients := make([]*Client, 0, len(s.Clients))
	for _, client := range s.Clients {
		clients = append(clients, client)
	}
	s.Clients = make(map[string]*Client)
	return clients
}

func (s *Server) closeClientsGracefully(message string) {
	clients := s.snapshotAndClearClients()
	for _, client := range clients {
		client.notifyClientCritical(message)
	}
	if len(clients) > 0 {
		time.Sleep(100 * time.Millisecond)
	}
	for _, client := range clients {
		client.Close()
	}
}

// Shutdown 负责优雅停止聊天服务：拒绝新消息、关闭在线连接，并等待主循环退出。
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownOnce.Do(func() {
		s.shuttingDown.Store(true) //这里是一个atomic.Bool类型,作为一个信号告诉系统里的其他代码：从现在开始，服务已经进入停机阶段，不要再接收新流量。
		close(s.done)
		s.closeClientsGracefully("服务器正在关闭，请稍后重连")
	})
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close 兼容旧调用方，内部退化为无超时的优雅关机。
func (s *Server) Close() {
	_ = s.Shutdown(context.Background())
}

func (s *Server) GetClient(uuid string) *Client {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.Clients[uuid]
}

func (s *Server) RemoveClient(client *Client) {
	if client == nil {
		return
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if existing, ok := s.Clients[client.Uuid]; ok && existing == client {
		delete(s.Clients, client.Uuid)
	}
}

// SendClientToLogin 把客户端加入登录通道，由服务主循环统一注册在线状态。
func (s *Server) SendClientToLogin(client *Client) bool {
	if s.IsShuttingDown() {
		return false
	}
	select {
	case <-s.done:
		return false
	case s.Login <- client:
		return true
	}
}

// SendClientToLogout 把客户端加入登出通道，由服务主循环统一执行清理。
func (s *Server) SendClientToLogout(client *Client) bool {
	if s.IsShuttingDown() {
		return false
	}
	select {
	case <-s.done:
		return false
	case s.Logout <- client:
		return true
	}
}

// SendMessageToTransmit 把消息投递到转发通道，交给主循环统一分发。
func (s *Server) SendMessageToTransmit(message []byte) bool {
	if s.IsShuttingDown() {
		return false
	}
	select {
	case <-s.done:
		return false
	case s.Transmit <- message:
		return true
	}
}
