package v1

// 本文件实现 user info controller 相关的 HTTP 控制器入口，负责参数绑定、调用 service 并返回统一响应。

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/service/gorm"
	"echo_chat_server/pkg/zlog"
)

// Register 注册
func Register(c *gin.Context) {
	var registerReq request.RegisterRequest
	if !BindJSONOrError(c, &registerReq, "api.user.register") {
		return
	}
	zlog.Info(
		"user.register.request",
		RequestFields(c, zap.String("telephone", registerReq.Telephone), zap.String("nickname", registerReq.Nickname))...,
	)
	message, userInfo, ret := gorm.UserInfoService.Register(registerReq)
	JsonBack(c, message, ret, userInfo)
}

// Login 登录
func Login(c *gin.Context) {
	var loginReq request.LoginRequest
	if !BindJSONOrError(c, &loginReq, "api.user.login") {
		return
	}
	message, userInfo, ret := gorm.UserInfoService.Login(loginReq)
	JsonBack(c, message, ret, userInfo)
}

// SmsLogin 验证码登录
func SmsLogin(c *gin.Context) {
	var req request.SmsLoginRequest
	if !BindJSONOrError(c, &req, "api.user.sms_login") {
		return
	}
	message, userInfo, ret := gorm.UserInfoService.SmsLogin(req)
	JsonBack(c, message, ret, userInfo)
}

// UpdateUserInfo 修改用户信息
func UpdateUserInfo(c *gin.Context) {
	var req request.UpdateUserInfoRequest
	if !BindJSONOrError(c, &req, "api.user.update") {
		return
	}
	req.Uuid = currentActorID(c)
	message, ret := gorm.UserInfoService.UpdateUserInfo(req)
	JsonBack(c, message, ret, nil)
}

// GetUserInfoList 获取用户列表
func GetUserInfoList(c *gin.Context) {
	var req request.GetUserInfoListRequest
	if !BindJSONOrError(c, &req, "api.user.list") {
		return
	}
	message, userList, ret := gorm.UserInfoService.GetUserInfoList(currentActorID(c))
	JsonBack(c, message, ret, userList)
}

// AbleUsers 启用用户
func AbleUsers(c *gin.Context) {
	var req request.AbleUsersRequest
	if !BindJSONOrError(c, &req, "api.user.able") {
		return
	}
	message, ret := gorm.UserInfoService.AbleUsers(req.UuidList)
	JsonBack(c, message, ret, nil)
}

// DisableUsers 禁用用户
func DisableUsers(c *gin.Context) {
	var req request.AbleUsersRequest
	if !BindJSONOrError(c, &req, "api.user.disable") {
		return
	}
	message, ret := gorm.UserInfoService.DisableUsers(req.UuidList)
	JsonBack(c, message, ret, nil)
}

// GetUserInfo 获取用户信息
func GetUserInfo(c *gin.Context) {
	var req request.GetUserInfoRequest
	if !BindJSONOrError(c, &req, "api.user.get") {
		return
	}
	if req.Uuid == "" {
		req.Uuid = currentActorID(c)
	}
	if !ensureSelfOrAdmin(c, req.Uuid, "api.user.get") {
		return
	}
	message, userInfo, ret := gorm.UserInfoService.GetUserInfo(req.Uuid)
	JsonBack(c, message, ret, userInfo)
}

// DeleteUsers 删除用户
func DeleteUsers(c *gin.Context) {
	var req request.AbleUsersRequest
	if !BindJSONOrError(c, &req, "api.user.delete") {
		return
	}
	message, ret := gorm.UserInfoService.DeleteUsers(req.UuidList)
	JsonBack(c, message, ret, nil)
}

// SetAdmin 设置管理员
func SetAdmin(c *gin.Context) {
	var req request.AbleUsersRequest
	if !BindJSONOrError(c, &req, "api.user.set_admin") {
		return
	}
	message, ret := gorm.UserInfoService.SetAdmin(req.UuidList, req.IsAdmin)
	JsonBack(c, message, ret, nil)
}

// SendSmsCode 发送短信验证码
func SendSmsCode(c *gin.Context) {
	var req request.SendSmsCodeRequest
	if !BindJSONOrError(c, &req, "api.user.send_sms") {
		return
	}
	message, ret := gorm.UserInfoService.SendSmsCode(req.Telephone)
	JsonBack(c, message, ret, nil)
}
