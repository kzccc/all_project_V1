package https_server

// 本文件负责初始化 Gin 引擎、中间件与全部 HTTP / WebSocket 路由。

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"io"
	v1 "echo_chat_server/api/v1"
	"echo_chat_server/internal/config"
	"echo_chat_server/pkg/constants"
)

var GE *gin.Engine

// init 在服务启动时完成 Gin 实例、中间件和全部路由注册。
func init() {
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	gin.DebugPrintRouteFunc = func(string, string, string, int) {}
	GE = gin.New()
	GE.Use(gin.Recovery())
	GE.Use(RequestLogMiddleware())
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", constants.REQUEST_ID_HEADER, constants.ACTOR_ID_HEADER}
	corsConfig.ExposeHeaders = []string{constants.REQUEST_ID_HEADER}
	GE.Use(cors.New(corsConfig))

	// 静态目录同时承担头像和聊天文件下载能力。
	GE.Static("/static/avatars", config.GetConfig().StaticAvatarPath)
	GE.Static("/static/files", config.GetConfig().StaticFilePath)

	// 公开接口。
	GE.POST("/login", v1.Login)
	GE.POST("/register", v1.Register)
	GE.POST("/auth/refresh", v1.RefreshToken) //前端业务请求带着 access_token , 如果后端返回 401, axios 响应拦截器会调用 refreshAccessToken(store)
	GE.POST("/user/sendSmsCode", v1.SendSmsCode)
	GE.POST("/user/smsLogin", v1.SmsLogin)
	registerObservabilityRoutes(GE)

	authed := GE.Group("/")    //创建一个根路由组，所有接口都需要鉴权
	authed.Use(AuthRequired()) // // 为该组的所有路由添加认证中间件
	{
		// 已登录用户也需要能拉自己的资料和主动登出。
		authed.POST("/user/getUserInfo", v1.GetUserInfo)
		authed.POST("/user/wsLogout", v1.WsLogout)

		active := authed.Group("/")
		active.Use(RequireActiveUser())
		{
			// 用户与认证相关接口。
			active.POST("/user/updateUserInfo", v1.UpdateUserInfo)

			// 群组管理相关接口。
			active.POST("/group/createGroup", v1.CreateGroup)
			active.POST("/group/loadMyGroup", v1.LoadMyGroup)
			active.POST("/group/checkGroupAddMode", v1.CheckGroupAddMode)
			active.POST("/group/enterGroupDirectly", v1.EnterGroupDirectly)
			active.POST("/group/leaveGroup", v1.LeaveGroup)
			active.POST("/group/dismissGroup", v1.DismissGroup)
			active.POST("/group/getGroupInfo", v1.GetGroupInfo)
			active.POST("/group/updateGroupInfo", v1.UpdateGroupInfo)
			active.POST("/group/getGroupMemberList", v1.GetGroupMemberList)
			active.POST("/group/removeGroupMembers", v1.RemoveGroupMembers)

			// 会话列表相关接口。
			active.POST("/session/openSession", v1.OpenSession)
			active.POST("/session/getUserSessionList", v1.GetUserSessionList)
			active.POST("/session/getGroupSessionList", v1.GetGroupSessionList)
			active.POST("/session/deleteSession", v1.DeleteSession)
			active.POST("/session/checkOpenSessionAllowed", v1.CheckOpenSessionAllowed)

			// 联系人与申请流转相关接口。
			active.POST("/contact/getUserList", v1.GetUserList)
			active.POST("/contact/loadMyJoinedGroup", v1.LoadMyJoinedGroup)
			active.POST("/contact/getContactInfo", v1.GetContactInfo)
			active.POST("/contact/deleteContact", v1.DeleteContact)
			active.POST("/contact/applyContact", v1.ApplyContact)
			active.POST("/contact/getNewContactList", v1.GetNewContactList)
			active.POST("/contact/passContactApply", v1.PassContactApply)
			active.POST("/contact/blackContact", v1.BlackContact)
			active.POST("/contact/cancelBlackContact", v1.CancelBlackContact)
			active.POST("/contact/getAddGroupList", v1.GetAddGroupList)
			active.POST("/contact/refuseContactApply", v1.RefuseContactApply)
			active.POST("/contact/blackApply", v1.BlackApply)

			// 消息与附件相关接口。
			active.POST("/message/getMessageList", v1.GetMessageList)
			active.POST("/message/getGroupMessageList", v1.GetGroupMessageList)
			active.POST("/message/uploadAvatar", v1.UploadAvatar)
			active.POST("/message/uploadFile", v1.UploadFile)

			// 聊天室与实时连接相关接口。
			active.POST("/chatroom/getCurContactListInChatRoom", v1.GetCurContactListInChatRoom)
			active.GET("/wss", v1.WsLogin)
		}

		admin := authed.Group("/")
		admin.Use(RequireAdmin())
		{
			admin.POST("/user/getUserInfoList", v1.GetUserInfoList)
			admin.POST("/user/ableUsers", v1.AbleUsers)
			admin.POST("/user/disableUsers", v1.DisableUsers)
			admin.POST("/user/deleteUsers", v1.DeleteUsers)
			admin.POST("/user/setAdmin", v1.SetAdmin)

			admin.POST("/group/getGroupInfoList", v1.GetGroupInfoList)
			admin.POST("/group/deleteGroups", v1.DeleteGroups)
			admin.POST("/group/setGroupsStatus", v1.SetGroupsStatus)
		}
	}
}
