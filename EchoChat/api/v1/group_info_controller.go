package v1

// 本文件实现 group info controller 相关的 HTTP 控制器入口，负责参数绑定、调用 service 并返回统一响应。

import (
	"github.com/gin-gonic/gin"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/service/gorm"
)

// CreateGroup 创建群聊
func CreateGroup(c *gin.Context) {
	var createGroupReq request.CreateGroupRequest
	if !BindJSONOrError(c, &createGroupReq, "api.group.create") {
		return
	}
	createGroupReq.OwnerId = currentActorID(c)
	message, ret := gorm.GroupInfoService.CreateGroup(createGroupReq)
	JsonBack(c, message, ret, nil)
}

// LoadMyGroup 获取我创建的群聊
func LoadMyGroup(c *gin.Context) {
	var loadMyGroupReq request.OwnlistRequest
	if !BindJSONOrError(c, &loadMyGroupReq, "api.group.load_my") {
		return
	}
	message, groupList, ret := gorm.GroupInfoService.LoadMyGroup(currentActorID(c))
	JsonBack(c, message, ret, groupList)
}

// CheckGroupAddMode 检查群聊加群方式
func CheckGroupAddMode(c *gin.Context) {
	var req request.CheckGroupAddModeRequest
	if !BindJSONOrError(c, &req, "api.group.check_add_mode") {
		return
	}
	message, addMode, ret := gorm.GroupInfoService.CheckGroupAddMode(req.GroupId)
	JsonBack(c, message, ret, addMode)
}

// EnterGroupDirectly 直接进群
func EnterGroupDirectly(c *gin.Context) {
	var req request.EnterGroupDirectlyRequest
	if !BindJSONOrError(c, &req, "api.group.enter_directly") {
		return
	}
	if !ensureSelfOrAdmin(c, req.ContactId, "api.group.enter_directly") {
		return
	}
	message, ret := gorm.GroupInfoService.EnterGroupDirectly(req.OwnerId, req.ContactId)
	JsonBack(c, message, ret, nil)
}

// LeaveGroup 退群
func LeaveGroup(c *gin.Context) {
	var req request.LeaveGroupRequest
	if !BindJSONOrError(c, &req, "api.group.leave") {
		return
	}
	message, ret := gorm.GroupInfoService.LeaveGroup(currentActorID(c), req.GroupId)
	JsonBack(c, message, ret, nil)
}

// DismissGroup 解散群聊
func DismissGroup(c *gin.Context) {
	var req request.DismissGroupRequest
	if !BindJSONOrError(c, &req, "api.group.dismiss") {
		return
	}
	if !ensureGroupOwner(c, req.GroupId, "api.group.dismiss") {
		return
	}
	message, ret := gorm.GroupInfoService.DismissGroup(currentActorID(c), req.GroupId)
	JsonBack(c, message, ret, nil)
}

// GetGroupInfo 获取群聊详情
func GetGroupInfo(c *gin.Context) {
	var req request.GetGroupInfoRequest
	if !BindJSONOrError(c, &req, "api.group.get_info") {
		return
	}
	message, groupInfo, ret := gorm.GroupInfoService.GetGroupInfo(req.GroupId)
	JsonBack(c, message, ret, groupInfo)
}

// GetGroupInfoList 获取群聊列表 - 管理员
func GetGroupInfoList(c *gin.Context) {
	message, groupList, ret := gorm.GroupInfoService.GetGroupInfoList()
	JsonBack(c, message, ret, groupList)
}

// DeleteGroups 删除列表中群聊 - 管理员
func DeleteGroups(c *gin.Context) {
	var req request.DeleteGroupsRequest
	if !BindJSONOrError(c, &req, "api.group.delete") {
		return
	}
	message, ret := gorm.GroupInfoService.DeleteGroups(req.UuidList)
	JsonBack(c, message, ret, nil)
}

// SetGroupsStatus 设置群聊是否启用
func SetGroupsStatus(c *gin.Context) {
	var req request.SetGroupsStatusRequest
	if !BindJSONOrError(c, &req, "api.group.set_status") {
		return
	}
	message, ret := gorm.GroupInfoService.SetGroupsStatus(req.UuidList, req.Status)
	JsonBack(c, message, ret, nil)
}

// UpdateGroupInfo 更新群聊消息
func UpdateGroupInfo(c *gin.Context) {
	var req request.UpdateGroupInfoRequest
	if !BindJSONOrError(c, &req, "api.group.update") {
		return
	}
	if !ensureGroupOwner(c, req.Uuid, "api.group.update") {
		return
	}
	req.OwnerId = currentActorID(c)
	message, ret := gorm.GroupInfoService.UpdateGroupInfo(req)
	JsonBack(c, message, ret, nil)
}

// GetGroupMemberList 获取群聊成员列表
func GetGroupMemberList(c *gin.Context) {
	var req request.GetGroupMemberListRequest
	if !BindJSONOrError(c, &req, "api.group.member_list") {
		return
	}
	if !ensureGroupMember(c, req.GroupId, "api.group.member_list") {
		return
	}
	message, groupMemberList, ret := gorm.GroupInfoService.GetGroupMemberList(req.GroupId)
	JsonBack(c, message, ret, groupMemberList)
}

// RemoveGroupMembers 移除群聊成员
func RemoveGroupMembers(c *gin.Context) {
	var req request.RemoveGroupMembersRequest
	if !BindJSONOrError(c, &req, "api.group.remove_members") {
		return
	}
	if !ensureGroupOwner(c, req.GroupId, "api.group.remove_members") {
		return
	}
	req.OwnerId = currentActorID(c)
	message, ret := gorm.GroupInfoService.RemoveGroupMembers(req)
	JsonBack(c, message, ret, nil)
}
