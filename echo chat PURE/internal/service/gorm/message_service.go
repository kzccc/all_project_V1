package gorm

// 本文件实现 message service 相关的业务服务，负责组织数据库、缓存和业务规则。

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"io"
	"echo_chat_server/internal/config"
	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/dto/respond"
	"echo_chat_server/internal/model"
	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/zlog"
	"os"
	"path/filepath"
)

type messageService struct {
}

// MessageService 提供消息列表查询和文件上传能力。
var MessageService = new(messageService)

// GetMessageList 获取聊天记录
func (m *messageService) GetMessageList(userOneId, userTwoId string) (string, []respond.GetMessageListRespond, int) {
	conversationKey := model.BuildConversationKey(userOneId, userTwoId)
	zlog.Info(
		"message.query.start",
		zap.String("event", "message.query.start"),
		zap.String("module", "message.service"),
		zap.String("scope", "user"),
		zap.String("user_one_id", userOneId),
		zap.String("user_two_id", userTwoId),
		zap.String("order_by", "session_seq ASC"),
	)
	rspString, err := myredis.GetKeyNilIsErr("message_list_" + userOneId + "_" + userTwoId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Info(
				"message.query.cache_miss",
				zap.String("event", "message.query.cache_miss"),
				zap.String("module", "message.service"),
				zap.String("scope", "user"),
				zap.String("user_one_id", userOneId),
				zap.String("user_two_id", userTwoId),
			)
			// 单聊消息需要同时查双方互发的记录，并按时间正序返回给前端。
			var messageList []model.Message
			if res := dao.GormDB.Where("conversation_key = ?", conversationKey).Order("session_seq ASC").Find(&messageList); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, nil, -1
			}
			var rspList []respond.GetMessageListRespond
			for _, message := range messageList {
				rspList = append(rspList, respond.GetMessageListRespond{
					MessageId:  message.Uuid,
					SendId:     message.SendId,
					SendName:   message.SendName,
					SendAvatar: message.SendAvatar,
					ReceiveId:  message.ReceiveId,
					Content:    message.Content,
					Url:        message.Url,
					Type:       message.Type,
					FileType:   message.FileType,
					FileName:   message.FileName,
					FileSize:   message.FileSize,
					SessionSeq: message.SessionSeq,
					CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}
			// 这里原本有回写 Redis 的逻辑，目前注释掉，说明消息列表主要依赖实时推送增量维护。
			//rspString, err := json.Marshal(rspList)
			//if err != nil {
			//	zlog.Error(err.Error())
			//}
			//if err := myredis.SetKeyEx("message_list_"+userOneId+"_"+userTwoId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
			//	zlog.Error(err.Error())
			//}
			zlog.Info(
				"message.query.finish",
				zap.String("event", "message.query.finish"),
				zap.String("module", "message.service"),
				zap.String("scope", "user"),
				zap.String("source", "db"),
				zap.String("user_one_id", userOneId),
				zap.String("user_two_id", userTwoId),
				zap.Int("result_count", len(rspList)),
				zap.String("order_by", "session_seq ASC"),
			)
			return "获取聊天记录成功", rspList, 0
		} else {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	var rsp []respond.GetMessageListRespond
	// 缓存命中时直接反序列化返回，减少聊天页频繁刷新带来的数据库压力。
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	zlog.Info(
		"message.query.finish",
		zap.String("event", "message.query.finish"),
		zap.String("module", "message.service"),
		zap.String("scope", "user"),
		zap.String("source", "cache"),
		zap.String("user_one_id", userOneId),
		zap.String("user_two_id", userTwoId),
		zap.Int("result_count", len(rsp)),
		zap.String("order_by", "session_seq ASC"),
	)
	return "获取群聊记录成功", rsp, 0
}

// GetGroupMessageList 获取群聊消息记录
func (m *messageService) GetGroupMessageList(groupId string) (string, []respond.GetGroupMessageListRespond, int) {
	conversationKey := model.BuildConversationKey("", groupId)
	zlog.Info(
		"message.query.start",
		zap.String("event", "message.query.start"),
		zap.String("module", "message.service"),
		zap.String("scope", "group"),
		zap.String("group_id", groupId),
		zap.String("order_by", "session_seq ASC"),
	)
	rspString, err := myredis.GetKeyNilIsErr("group_messagelist_" + groupId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Info(
				"message.query.cache_miss",
				zap.String("event", "message.query.cache_miss"),
				zap.String("module", "message.service"),
				zap.String("scope", "group"),
				zap.String("group_id", groupId),
			)
			// 群消息列表只需要按 receive_id 查询该群的全部消息即可。
			var messageList []model.Message
			if res := dao.GormDB.Where("conversation_key = ?", conversationKey).Order("session_seq ASC").Find(&messageList); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, nil, -1
			}
			var rspList []respond.GetGroupMessageListRespond
			for _, message := range messageList {
				rsp := respond.GetGroupMessageListRespond{
					MessageId:  message.Uuid,
					SendId:     message.SendId,
					SendName:   message.SendName,
					SendAvatar: message.SendAvatar,
					ReceiveId:  message.ReceiveId,
					Content:    message.Content,
					Url:        message.Url,
					Type:       message.Type,
					FileType:   message.FileType,
					FileName:   message.FileName,
					FileSize:   message.FileSize,
					SessionSeq: message.SessionSeq,
					CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
				}
				rspList = append(rspList, rsp)
			}
			//rspString, err := json.Marshal(rspList)
			//if err != nil {
			//	zlog.Error(err.Error())
			//}
			//if err := myredis.SetKeyEx("group_messagelist_"+groupId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
			//	zlog.Error(err.Error())
			//}
			zlog.Info(
				"message.query.finish",
				zap.String("event", "message.query.finish"),
				zap.String("module", "message.service"),
				zap.String("scope", "group"),
				zap.String("source", "db"),
				zap.String("group_id", groupId),
				zap.Int("result_count", len(rspList)),
				zap.String("order_by", "session_seq ASC"),
			)
			return "获取聊天记录成功", rspList, 0
		} else {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	var rsp []respond.GetGroupMessageListRespond
	// 群消息缓存命中后直接返回，避免在高频刷新场景下重复扫消息表。
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	zlog.Info(
		"message.query.finish",
		zap.String("event", "message.query.finish"),
		zap.String("module", "message.service"),
		zap.String("scope", "group"),
		zap.String("source", "cache"),
		zap.String("group_id", groupId),
		zap.Int("result_count", len(rsp)),
		zap.String("order_by", "session_seq ASC"),
	)
	return "获取聊天记录成功", rsp, 0
}

// UploadAvatar 上传头像
func (m *messageService) UploadAvatar(c *gin.Context) (string, int) {
	if err := c.Request.ParseMultipartForm(constants.FILE_MAX_SIZE); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	mForm := c.Request.MultipartForm
	for key, _ := range mForm.File {
		// 当前接口允许一次上传多个字段，但实际前端通常只传一个头像文件。
		file, fileHeader, err := c.Request.FormFile(key)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		defer file.Close()
		zlog.Info(fmt.Sprintf("文件名：%s，文件大小：%d", fileHeader.Filename, fileHeader.Size))
		// 原来Filename应该是213451545.xxx，将Filename修改为avatar_ownerId.xxx
		ext := filepath.Ext(fileHeader.Filename)
		zlog.Info(ext)
		// 落盘前确保头像目录已存在，兼容首次启动或空目录场景。
		if err := os.MkdirAll(config.GetConfig().StaticAvatarPath, 0755); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		localFileName := config.GetConfig().StaticAvatarPath + "/" + fileHeader.Filename
		out, err := os.Create(localFileName)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		// 上传接口只负责保存文件，不在这里直接改用户资料表。
		zlog.Info("完成文件上传")
	}
	return "上传成功", 0
}

// UploadFile 上传文件
func (m *messageService) UploadFile(c *gin.Context) (string, int) {
	if err := c.Request.ParseMultipartForm(constants.FILE_MAX_SIZE); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	mForm := c.Request.MultipartForm
	for key, _ := range mForm.File {
		// 文件消息先通过 HTTP 接口落盘，再由聊天消息体携带 URL 通知接收方。
		file, fileHeader, err := c.Request.FormFile(key)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		defer file.Close()
		zlog.Info(fmt.Sprintf("文件名：%s，文件大小：%d", fileHeader.Filename, fileHeader.Size))
		// 原来Filename应该是213451545.xxx，将Filename修改为avatar_ownerId.xxx
		ext := filepath.Ext(fileHeader.Filename)
		zlog.Info(ext)
		// 落盘前同样保证目标目录存在，防止部署时目录缺失导致上传失败。
		if err := os.MkdirAll(config.GetConfig().StaticFilePath, 0755); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		localFileName := config.GetConfig().StaticFilePath + "/" + fileHeader.Filename
		out, err := os.Create(localFileName)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		zlog.Info("完成文件上传")
	}
	return "上传成功", 0
}
