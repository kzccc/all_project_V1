package v1

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"kama_chat_server/internal/dto/request"
	"kama_chat_server/internal/service/gorm"
	"kama_chat_server/pkg/constants"
	"kama_chat_server/pkg/zlog"
	"net/http"
)

// Register 注册
// @Summary 用户注册
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.RegisterRequest true "注册信息"
// @Success 200 {object} SwaggerResponse
// @Router /register [post]
func Register(c *gin.Context) {
	var registerReq request.RegisterRequest
	if err := c.BindJSON(&registerReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	fmt.Println(registerReq)
	message, userInfo, ret := gorm.UserInfoService.Register(registerReq)
	JsonBack(c, message, ret, userInfo)
}

// Login 登录
// @Summary 用户登录
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.LoginRequest true "登录信息"
// @Success 200 {object} SwaggerResponse
// @Router /login [post]
func Login(c *gin.Context) {
	var loginReq request.LoginRequest
	if err := c.BindJSON(&loginReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, userInfo, ret := gorm.UserInfoService.Login(loginReq)
	JsonBack(c, message, ret, userInfo)
}

// SmsLogin 验证码登录
// @Summary 验证码登录
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.SmsLoginRequest true "验证码登录信息"
// @Success 200 {object} SwaggerResponse
// @Router /user/smsLogin [post]
func SmsLogin(c *gin.Context) {
	var req request.SmsLoginRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, userInfo, ret := gorm.UserInfoService.SmsLogin(req)
	JsonBack(c, message, ret, userInfo)
}

// UpdateUserInfo 修改用户信息
// @Summary 修改用户信息
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.UpdateUserInfoRequest true "修改用户信息请求"
// @Success 200 {object} SwaggerResponse
// @Router /user/updateUserInfo [post]
func UpdateUserInfo(c *gin.Context) {
	var req request.UpdateUserInfoRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserInfoService.UpdateUserInfo(req)
	JsonBack(c, message, ret, nil)
}

// GetUserInfoList 获取用户列表
// @Summary 获取用户列表
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.GetUserInfoListRequest true "查询用户列表请求"
// @Success 200 {object} SwaggerResponse
// @Router /user/getUserInfoList [post]
func GetUserInfoList(c *gin.Context) {
	var req request.GetUserInfoListRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, userList, ret := gorm.UserInfoService.GetUserInfoList(req.OwnerId)
	JsonBack(c, message, ret, userList)
}

// AbleUsers 启用用户
// @Summary 启用用户
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.AbleUsersRequest true "用户ID列表"
// @Success 200 {object} SwaggerResponse
// @Router /user/ableUsers [post]
func AbleUsers(c *gin.Context) {
	var req request.AbleUsersRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserInfoService.AbleUsers(req.UuidList)
	JsonBack(c, message, ret, nil)
}

// DisableUsers 禁用用户
// @Summary 禁用用户
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.AbleUsersRequest true "用户ID列表"
// @Success 200 {object} SwaggerResponse
// @Router /user/disableUsers [post]
func DisableUsers(c *gin.Context) {
	var req request.AbleUsersRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserInfoService.DisableUsers(req.UuidList)
	JsonBack(c, message, ret, nil)
}

// GetUserInfo 获取用户信息
// @Summary 获取用户信息
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.GetUserInfoRequest true "用户ID"
// @Success 200 {object} SwaggerResponse
// @Router /user/getUserInfo [post]
func GetUserInfo(c *gin.Context) {
	var req request.GetUserInfoRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, userInfo, ret := gorm.UserInfoService.GetUserInfo(req.Uuid)
	JsonBack(c, message, ret, userInfo)
}

// DeleteUsers 删除用户
// @Summary 删除用户
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.AbleUsersRequest true "用户ID列表"
// @Success 200 {object} SwaggerResponse
// @Router /user/deleteUsers [post]
func DeleteUsers(c *gin.Context) {
	var req request.AbleUsersRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserInfoService.DeleteUsers(req.UuidList)
	JsonBack(c, message, ret, nil)
}

// SetAdmin 设置管理员
// @Summary 设置管理员
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.AbleUsersRequest true "用户ID列表"
// @Success 200 {object} SwaggerResponse
// @Router /user/setAdmin [post]
func SetAdmin(c *gin.Context) {
	var req request.AbleUsersRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserInfoService.SetAdmin(req.UuidList, req.IsAdmin)
	JsonBack(c, message, ret, nil)
}

// SendSmsCode 发送短信验证码
// @Summary 发送短信验证码
// @Tags user
// @Accept json
// @Produce json
// @Param body body request.SendSmsCodeRequest true "手机号"
// @Success 200 {object} SwaggerResponse
// @Router /user/sendSmsCode [post]
func SendSmsCode(c *gin.Context) {
	var req request.SendSmsCodeRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserInfoService.SendSmsCode(req.Telephone)
	JsonBack(c, message, ret, nil)
}
