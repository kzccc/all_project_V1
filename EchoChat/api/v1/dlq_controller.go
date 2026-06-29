package v1

import (
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"echo_chat_server/internal/dto/request"
	"echo_chat_server/internal/service/dlq"
)

func DLQList(c *gin.Context) {
	var req request.DLQListRequest
	if !BindJSONOrError(c, &req, "api.dlq.list") {
		return
	}
	data, err := dlq.Service.List(req)
	if err != nil {
		JsonBack(c, err.Error(), -1, nil)
		return
	}
	JsonBack(c, "获取成功", 0, data)
}

func DLQDetail(c *gin.Context) {
	var req request.DLQIDRequest
	if !BindJSONOrError(c, &req, "api.dlq.detail") {
		return
	}
	data, err := dlq.Service.Detail(req.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			JsonBack(c, "记录不存在", -2, nil)
			return
		}
		JsonBack(c, err.Error(), -1, nil)
		return
	}
	JsonBack(c, "获取成功", 0, data)
}

func DLQLogs(c *gin.Context) {
	var req request.DLQIDRequest
	if !BindJSONOrError(c, &req, "api.dlq.logs") {
		return
	}
	data, err := dlq.Service.Logs(req.ID)
	if err != nil {
		JsonBack(c, err.Error(), -1, nil)
		return
	}
	JsonBack(c, "获取成功", 0, data)
}

func DLQStats(c *gin.Context) {
	data, err := dlq.Service.Stats()
	if err != nil {
		JsonBack(c, err.Error(), -1, nil)
		return
	}
	JsonBack(c, "获取成功", 0, data)
}

func DLQClaim(c *gin.Context) {
	var req request.DLQClaimRequest
	if !BindJSONOrError(c, &req, "api.dlq.claim") {
		return
	}
	if err := dlq.Service.Claim(req.ID, currentActorID(c)); err != nil {
		JsonBack(c, err.Error(), -1, nil)
		return
	}
	JsonBack(c, "操作成功", 0, nil)
}

func DLQReopen(c *gin.Context) {
	var req request.DLQReopenRequest
	if !BindJSONOrError(c, &req, "api.dlq.reopen") {
		return
	}
	if err := dlq.Service.Reopen(req.ID, currentActorID(c)); err != nil {
		JsonBack(c, err.Error(), -1, nil)
		return
	}
	JsonBack(c, "操作成功", 0, nil)
}

func DLQClose(c *gin.Context) {
	var req request.DLQCloseRequest
	if !BindJSONOrError(c, &req, "api.dlq.close") {
		return
	}
	if err := dlq.Service.Close(req.ID, currentActorID(c), req.CloseReason, req.Remark); err != nil {
		JsonBack(c, err.Error(), -1, nil)
		return
	}
	JsonBack(c, "操作成功", 0, nil)
}

func DLQRemark(c *gin.Context) {
	var req request.DLQRemarkRequest
	if !BindJSONOrError(c, &req, "api.dlq.remark") {
		return
	}
	if err := dlq.Service.UpdateRemark(req.ID, currentActorID(c), req.Remark); err != nil {
		JsonBack(c, err.Error(), -1, nil)
		return
	}
	JsonBack(c, "操作成功", 0, nil)
}
