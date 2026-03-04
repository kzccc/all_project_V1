package v1

import (
	"github.com/gin-gonic/gin"
	"kama_chat_server/internal/dto/request"
	"kama_chat_server/internal/service/gorm"
	"kama_chat_server/pkg/constants"
	"kama_chat_server/pkg/zlog"
	"log"
	"net/http"
)

// GetUserList 获取联系人列表
// @Summary 获取联系人列表
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.OwnlistRequest true "用户ID"
// @Success 200 {object} SwaggerResponse
// @Router /contact/getUserList [post]
func GetUserList(c *gin.Context) {
	var myUserListReq request.OwnlistRequest
	if err := c.BindJSON(&myUserListReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
	}
	message, userList, ret := gorm.UserContactService.GetUserList(myUserListReq.OwnerId)
	JsonBack(c, message, ret, userList)
}

// LoadMyJoinedGroup 获取我加入的群聊
// @Summary 获取我加入的群聊
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.OwnlistRequest true "用户ID"
// @Success 200 {object} SwaggerResponse
// @Router /contact/loadMyJoinedGroup [post]
func LoadMyJoinedGroup(c *gin.Context) {
	var loadMyJoinedGroupReq request.OwnlistRequest
	if err := c.BindJSON(&loadMyJoinedGroupReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, groupList, ret := gorm.UserContactService.LoadMyJoinedGroup(loadMyJoinedGroupReq.OwnerId)
	JsonBack(c, message, ret, groupList)
}

// GetContactInfo 获取联系人信息
// @Summary 获取联系人信息
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.GetContactInfoRequest true "联系人ID"
// @Success 200 {object} SwaggerResponse
// @Router /contact/getContactInfo [post]
func GetContactInfo(c *gin.Context) {
	var getContactInfoReq request.GetContactInfoRequest
	if err := c.BindJSON(&getContactInfoReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	log.Println(getContactInfoReq)
	message, contactInfo, ret := gorm.UserContactService.GetContactInfo(getContactInfoReq.ContactId)
	JsonBack(c, message, ret, contactInfo)
}

// DeleteContact 删除联系人
// @Summary 删除联系人
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.DeleteContactRequest true "删除联系人请求"
// @Success 200 {object} SwaggerResponse
// @Router /contact/deleteContact [post]
func DeleteContact(c *gin.Context) {
	var deleteContactReq request.DeleteContactRequest
	if err := c.BindJSON(&deleteContactReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserContactService.DeleteContact(deleteContactReq.OwnerId, deleteContactReq.ContactId)
	JsonBack(c, message, ret, nil)
}

// ApplyContact 申请添加联系人
// @Summary 申请添加联系人
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.ApplyContactRequest true "申请联系人请求"
// @Success 200 {object} SwaggerResponse
// @Router /contact/applyContact [post]
func ApplyContact(c *gin.Context) {
	var applyContactReq request.ApplyContactRequest
	if err := c.BindJSON(&applyContactReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserContactService.ApplyContact(applyContactReq)
	JsonBack(c, message, ret, nil)
}

// GetNewContactList 获取新的联系人申请列表
// @Summary 获取新的联系人申请列表
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.OwnlistRequest true "用户ID"
// @Success 200 {object} SwaggerResponse
// @Router /contact/getNewContactList [post]
func GetNewContactList(c *gin.Context) {
	var req request.OwnlistRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, data, ret := gorm.UserContactService.GetNewContactList(req.OwnerId)
	JsonBack(c, message, ret, data)
}

// PassContactApply 通过联系人申请
// @Summary 通过联系人申请
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.PassContactApplyRequest true "通过联系人申请请求"
// @Success 200 {object} SwaggerResponse
// @Router /contact/passContactApply [post]
func PassContactApply(c *gin.Context) {
	var passContactApplyReq request.PassContactApplyRequest
	if err := c.BindJSON(&passContactApplyReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserContactService.PassContactApply(passContactApplyReq.OwnerId, passContactApplyReq.ContactId)
	JsonBack(c, message, ret, nil)
}

// RefuseContactApply 拒绝联系人申请
// @Summary 拒绝联系人申请
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.PassContactApplyRequest true "拒绝联系人申请请求"
// @Success 200 {object} SwaggerResponse
// @Router /contact/refuseContactApply [post]
func RefuseContactApply(c *gin.Context) {
	var passContactApplyReq request.PassContactApplyRequest
	if err := c.BindJSON(&passContactApplyReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserContactService.RefuseContactApply(passContactApplyReq.OwnerId, passContactApplyReq.ContactId)
	JsonBack(c, message, ret, nil)
}

// BlackContact 拉黑联系人
// @Summary 拉黑联系人
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.BlackContactRequest true "拉黑联系人请求"
// @Success 200 {object} SwaggerResponse
// @Router /contact/blackContact [post]
func BlackContact(c *gin.Context) {
	var req request.BlackContactRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserContactService.BlackContact(req.OwnerId, req.ContactId)
	JsonBack(c, message, ret, nil)
}

// CancelBlackContact 解除拉黑联系人
// @Summary 解除拉黑联系人
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.BlackContactRequest true "解除拉黑联系人请求"
// @Success 200 {object} SwaggerResponse
// @Router /contact/cancelBlackContact [post]
func CancelBlackContact(c *gin.Context) {
	var req request.BlackContactRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserContactService.CancelBlackContact(req.OwnerId, req.ContactId)
	JsonBack(c, message, ret, nil)
}

// GetAddGroupList 获取新的群聊申请列表
// @Summary 获取新的群聊申请列表
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.AddGroupListRequest true "群聊ID"
// @Success 200 {object} SwaggerResponse
// @Router /contact/getAddGroupList [post]
func GetAddGroupList(c *gin.Context) {
	var req request.AddGroupListRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, data, ret := gorm.UserContactService.GetAddGroupList(req.GroupId)
	JsonBack(c, message, ret, data)
}

// BlackApply 拉黑申请
// @Summary 拉黑申请
// @Tags contact
// @Accept json
// @Produce json
// @Param body body request.BlackApplyRequest true "拉黑申请请求"
// @Success 200 {object} SwaggerResponse
// @Router /contact/blackApply [post]
func BlackApply(c *gin.Context) {
	var req request.BlackApplyRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserContactService.BlackApply(req.OwnerId, req.ContactId)
	JsonBack(c, message, ret, nil)
}
