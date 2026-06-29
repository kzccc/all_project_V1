package v1

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"echo_chat_server/internal/auth"
	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/model"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/enum/contact/contact_status_enum"
	"echo_chat_server/pkg/zlog"
)

func CurrentUser(c *gin.Context) *model.UserInfo {
	value, ok := c.Get(constants.CURRENT_USER_CONTEXT_KEY)
	if !ok {
		return nil
	}
	user, ok := value.(*model.UserInfo)
	if !ok {
		return nil
	}
	return user
}

func CurrentClaims(c *gin.Context) *auth.Claims {
	value, ok := c.Get(constants.CURRENT_TOKEN_CLAIMS_CONTEXT_KEY)
	if !ok {
		return nil
	}
	claims, ok := value.(*auth.Claims)
	if !ok {
		return nil
	}
	return claims
}

func currentActorID(c *gin.Context) string {
	if user := CurrentUser(c); user != nil {
		return user.Uuid
	}
	return ActorID(c)
}

func abortForbidden(c *gin.Context, api string, message string) bool {
	zlog.Error(
		"http.request.forbidden",
		RequestFields(
			c,
			zap.String("event", "http.request.forbidden"),
			zap.String("module", api),
			zap.String("reason", message),
		)...,
	)
	c.JSON(http.StatusOK, gin.H{
		"code":    403,
		"message": message,
	})
	c.Abort()
	return false
}

func ensureActorMatches(c *gin.Context, candidate string, api string) bool {
	actorID := currentActorID(c)
	candidate = strings.TrimSpace(candidate)
	if actorID == "" || candidate == "" {
		return abortForbidden(c, api, "请求身份校验失败")
	}
	if actorID != candidate {
		return abortForbidden(c, api, "无权操作其他用户数据")
	}
	return true
}

func ensureSelfOrAdmin(c *gin.Context, userID string, api string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return abortForbidden(c, api, "请求身份校验失败")
	}
	currentUser := CurrentUser(c)
	if currentUser == nil {
		return abortForbidden(c, api, "登录已失效，请重新登录")
	}
	if currentUser.IsAdmin == 1 || currentUser.Uuid == userID {
		return true
	}
	return abortForbidden(c, api, "无权访问该用户数据")
}

func ensureMessageParticipant(c *gin.Context, userOneID string, userTwoID string, api string) (string, bool) {
	actorID := currentActorID(c)
	switch actorID {
	case strings.TrimSpace(userOneID):
		return strings.TrimSpace(userTwoID), true
	case strings.TrimSpace(userTwoID):
		return strings.TrimSpace(userOneID), true
	default:
		_ = abortForbidden(c, api, "无权查看该会话消息")
		return "", false
	}
}

func ensureGroupOwner(c *gin.Context, groupID string, api string) bool {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return abortForbidden(c, api, "群组不存在")
	}
	currentUser := CurrentUser(c)
	if currentUser == nil {
		return abortForbidden(c, api, "登录已失效，请重新登录")
	}
	if currentUser.IsAdmin == 1 {
		return true
	}
	var group model.GroupInfo
	if err := dao.GormDB.Where("uuid = ?", groupID).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return abortForbidden(c, api, "群组不存在")
		}
		zlog.Error("group.owner.lookup_failed", RequestFields(c, zap.String("event", "group.owner.lookup_failed"), zap.String("module", api), zap.String("group_id", groupID), zap.String("error", err.Error()))...)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		c.Abort()
		return false
	}
	if group.OwnerId != currentUser.Uuid {
		return abortForbidden(c, api, "仅群主可执行该操作")
	}
	return true
}

func ensureGroupMember(c *gin.Context, groupID string, api string) bool {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return abortForbidden(c, api, "群组不存在")
	}
	currentUser := CurrentUser(c)
	if currentUser == nil {
		return abortForbidden(c, api, "登录已失效，请重新登录")
	}
	if currentUser.IsAdmin == 1 {
		return true
	}
	var group model.GroupInfo
	if err := dao.GormDB.Where("uuid = ?", groupID).First(&group).Error; err == nil && group.OwnerId == currentUser.Uuid {
		return true
	}
	var contact model.UserContact
	if err := dao.GormDB.
		Where("user_id = ? AND contact_id = ? AND status NOT IN ?", currentUser.Uuid, groupID, []int8{contact_status_enum.QUIT_GROUP, contact_status_enum.KICK_OUT_GROUP}).
		First(&contact).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return abortForbidden(c, api, "仅群成员可访问该群数据")
		}
		zlog.Error("group.member.lookup_failed", RequestFields(c, zap.String("event", "group.member.lookup_failed"), zap.String("module", api), zap.String("group_id", groupID), zap.String("error", err.Error()))...)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		c.Abort()
		return false
	}
	return true
}

func ensureApplyOwnerAuthorized(c *gin.Context, ownerID string, api string) bool {
	ownerID = strings.TrimSpace(ownerID)
	switch {
	case ownerID == "":
		return abortForbidden(c, api, "请求身份校验失败")
	case strings.HasPrefix(ownerID, "U"):
		return ensureSelfOrAdmin(c, ownerID, api)
	case strings.HasPrefix(ownerID, "G"):
		return ensureGroupOwner(c, ownerID, api)
	default:
		return abortForbidden(c, api, "请求身份校验失败")
	}
}
