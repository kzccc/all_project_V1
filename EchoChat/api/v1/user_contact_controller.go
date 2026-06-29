package v1

// 本文件实现 user contact controller 相关的 HTTP 控制器入口，负责参数绑定、调用 service 并返回统一响应。

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/service/gorm"
	"echo_chat_server/pkg/zlog"
)

// GetUserList 获取联系人列表
func GetUserList(c *gin.Context) {
	var myUserListReq request.OwnlistRequest
	if !BindJSONOrError(c, &myUserListReq, "api.contact.user_list") {
		return
	}
	message, userList, ret := gorm.UserContactService.GetUserList(currentActorID(c))
	JsonBack(c, message, ret, userList)
}

// LoadMyJoinedGroup 获取我加入的群聊
func LoadMyJoinedGroup(c *gin.Context) {
	var loadMyJoinedGroupReq request.OwnlistRequest
	if !BindJSONOrError(c, &loadMyJoinedGroupReq, "api.contact.joined_group") {
		return
	}
	message, groupList, ret := gorm.UserContactService.LoadMyJoinedGroup(currentActorID(c))
	JsonBack(c, message, ret, groupList)
}

// GetContactInfo 获取联系人信息
func GetContactInfo(c *gin.Context) {
	var getContactInfoReq request.GetContactInfoRequest
	if !BindJSONOrError(c, &getContactInfoReq, "api.contact.info") {
		return
	}
	zlog.Info("contact.info.request", RequestFields(c, zap.String("contact_id", getContactInfoReq.ContactId))...)
	message, contactInfo, ret := gorm.UserContactService.GetContactInfo(getContactInfoReq.ContactId)
	JsonBack(c, message, ret, contactInfo)
}

// DeleteContact 删除联系人
func DeleteContact(c *gin.Context) {
	var deleteContactReq request.DeleteContactRequest
	if !BindJSONOrError(c, &deleteContactReq, "api.contact.delete") {
		return
	}
	message, ret := gorm.UserContactService.DeleteContact(currentActorID(c), deleteContactReq.ContactId)
	JsonBack(c, message, ret, nil)
}

// ApplyContact 申请添加联系人
func ApplyContact(c *gin.Context) {
	var applyContactReq request.ApplyContactRequest
	if !BindJSONOrError(c, &applyContactReq, "api.contact.apply") {
		return
	}
	applyContactReq.OwnerId = currentActorID(c)
	message, ret := gorm.UserContactService.ApplyContact(applyContactReq)
	JsonBack(c, message, ret, nil)
}

// GetNewContactList 获取新的联系人申请列表
func GetNewContactList(c *gin.Context) {
	var req request.OwnlistRequest
	if !BindJSONOrError(c, &req, "api.contact.new_list") {
		return
	}
	message, data, ret := gorm.UserContactService.GetNewContactList(currentActorID(c))
	JsonBack(c, message, ret, data)
}

// PassContactApply 通过联系人申请
func PassContactApply(c *gin.Context) {
	var passContactApplyReq request.PassContactApplyRequest
	if !BindJSONOrError(c, &passContactApplyReq, "api.contact.pass_apply") {
		return
	}
	if !ensureApplyOwnerAuthorized(c, passContactApplyReq.OwnerId, "api.contact.pass_apply") {
		return
	}
	message, ret := gorm.UserContactService.PassContactApply(passContactApplyReq.OwnerId, passContactApplyReq.ContactId)
	JsonBack(c, message, ret, nil)
}

// RefuseContactApply 拒绝联系人申请
func RefuseContactApply(c *gin.Context) {
	var passContactApplyReq request.PassContactApplyRequest
	if !BindJSONOrError(c, &passContactApplyReq, "api.contact.refuse_apply") {
		return
	}
	if !ensureApplyOwnerAuthorized(c, passContactApplyReq.OwnerId, "api.contact.refuse_apply") {
		return
	}
	message, ret := gorm.UserContactService.RefuseContactApply(passContactApplyReq.OwnerId, passContactApplyReq.ContactId)
	JsonBack(c, message, ret, nil)
}

// BlackContact 拉黑联系人
func BlackContact(c *gin.Context) {
	var req request.BlackContactRequest
	if !BindJSONOrError(c, &req, "api.contact.black") {
		return
	}
	message, ret := gorm.UserContactService.BlackContact(currentActorID(c), req.ContactId)
	JsonBack(c, message, ret, nil)
}

// CancelBlackContact 解除拉黑联系人
func CancelBlackContact(c *gin.Context) {
	var req request.BlackContactRequest
	if !BindJSONOrError(c, &req, "api.contact.cancel_black") {
		return
	}
	message, ret := gorm.UserContactService.CancelBlackContact(currentActorID(c), req.ContactId)
	JsonBack(c, message, ret, nil)
}

// GetAddGroupList 获取新的群聊申请列表
func GetAddGroupList(c *gin.Context) {
	var req request.AddGroupListRequest
	if !BindJSONOrError(c, &req, "api.contact.add_group_list") {
		return
	}
	if !ensureGroupOwner(c, req.GroupId, "api.contact.add_group_list") {
		return
	}
	message, data, ret := gorm.UserContactService.GetAddGroupList(req.GroupId)
	JsonBack(c, message, ret, data)
}

// BlackApply 拉黑申请
func BlackApply(c *gin.Context) {
	var req request.BlackApplyRequest
	if !BindJSONOrError(c, &req, "api.contact.black_apply") {
		return
	}
	if !ensureApplyOwnerAuthorized(c, req.OwnerId, "api.contact.black_apply") {
		return
	}
	message, ret := gorm.UserContactService.BlackApply(req.OwnerId, req.ContactId)
	JsonBack(c, message, ret, nil)
}
