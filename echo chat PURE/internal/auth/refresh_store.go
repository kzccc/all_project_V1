package auth

import (
	"errors"
	"echo_chat_server/internal/config"
	myredis "echo_chat_server/internal/service/redis"
	"time"

	redis "github.com/go-redis/redis/v8"
)

var ErrRefreshTokenRevoked = errors.New("refresh token revoked")

// refreshTokenKey 根据 sessionID 生成 refresh token 在 Redis 中使用的 key。
//
// 当前项目使用 sessionID 作为一个登录会话的唯一标识，
// 每个会话在 Redis 中只保留一份当前有效的 refresh token。
func refreshTokenKey(sessionID string) string {
	return "refresh_token:" + sessionID
}

// SaveRefreshToken 将当前会话对应的 refresh token 写入 Redis，并设置过期时间。
//
// 这里会使用 sessionID 作为 Redis key 的一部分，
// 这样服务端后续就可以根据 sessionID 找到该会话当前有效的 refresh token。
func SaveRefreshToken(sessionID string, refreshToken string) error {
	// sessionID 不能为空，否则无法建立会话与 refresh token 的映射关系。
	if sessionID == "" {
		return ErrEmptySessionID
	}

	// Redis 中 refresh token 的过期时间与 JWT 配置中的 refresh token 有效期保持一致。
	timeout := time.Duration(getRefreshExpireHours()) * time.Hour

	// 将 refresh token 保存到 Redis。
	return myredis.SetKeyEx(refreshTokenKey(sessionID), refreshToken, timeout)
}

// ValidateRefreshToken 校验某个 sessionID 下传入的 refresh token 是否仍然有效。
//
// 这个校验不是检查 JWT 的签名和过期时间，
// 而是检查“当前会话在服务端记录的 refresh token”是否与传入值一致。
// 这样可以支持 refresh token 轮换、服务端撤销和主动登出。
func ValidateRefreshToken(sessionID string, refreshToken string) error {
	// sessionID 不能为空，否则无法定位到对应的会话记录。
	if sessionID == "" {
		return ErrEmptySessionID
	}

	// 从 Redis 中读取该 sessionID 当前保存的 refresh token。
	storedToken, err := myredis.GetKeyNilIsErr(refreshTokenKey(sessionID))
	if err != nil {
		// 如果 Redis 中不存在这条记录，说明该 refresh token 已被删除、已过期
		// 或者该会话已经被服务端撤销，此时统一视为 refresh token 已失效。
		if errors.Is(err, redis.Nil) {
			return ErrRefreshTokenRevoked
		}

		// 其他错误属于 Redis 或系统异常，交由上层继续处理。
		return err
	}

	// 如果 Redis 中保存的 token 与当前传入的 token 不一致，
	// 说明这个 refresh token 已经不是该会话当前认可的最新 token，
	// 常见于 refresh token 已轮换、旧 token 被替换的场景。
	if storedToken != refreshToken {
		return ErrRefreshTokenRevoked
	}

	// 读取成功且值完全一致，说明这个 refresh token 仍然有效。
	return nil
}

// DeleteRefreshToken 删除某个 sessionID 对应的 refresh token 记录。
//
// 这个函数通常用于主动退出登录、账号状态异常或服务端撤销会话时，
// 删除后该会话的 refresh token 将无法再用于刷新 access token。
func DeleteRefreshToken(sessionID string) error {
	// sessionID 为空时没有可删除的目标，直接返回即可。
	if sessionID == "" {
		return nil
	}

	// 删除 Redis 中该会话对应的 refresh token 记录。
	return myredis.DelKeyIfExists(refreshTokenKey(sessionID))
}

// getRefreshExpireHours 获取 refresh token 的过期时长，单位为小时。
//
// 如果配置值小于等于 0，则回退到默认值 168 小时，也就是 7 天。
func getRefreshExpireHours() int {
	hours := config.GetConfig().JwtConfig.RefreshExpireHours
	if hours <= 0 {
		return 168
	}
	return hours
}
