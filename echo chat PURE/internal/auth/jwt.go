package auth

import (
	"errors"
	"fmt"
	"echo_chat_server/internal/config"
	"echo_chat_server/internal/model"
	"echo_chat_server/pkg/util/random"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	AccessTokenType  TokenType = "access"
	RefreshTokenType TokenType = "refresh"
)

var (
	ErrInvalidToken   = errors.New("invalid token")
	ErrInvalidType    = errors.New("invalid token type")
	ErrEmptyToken     = errors.New("empty token")
	ErrEmptySessionID = errors.New("empty session id")
)

type Claims struct {
	UserID    int64     `json:"user_id"`
	UserUUID  string    `json:"user_uuid"`
	Telephone string    `json:"telephone"`
	IsAdmin   int8      `json:"is_admin"`
	SessionID string    `json:"session_id"`
	TokenType TokenType `json:"token_type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	SessionID    string
}

func signingKey() ([]byte, error) {
	key := config.GetConfig().JwtConfig.Key
	if key == "" {
		return nil, errors.New("jwt signing key is empty")
	}
	return []byte(key), nil
}

func newSessionID() string {
	//?返回的字符串就是TS+8(年月日)+11位随机数
	return "TS" + random.GetNowAndLenRandomString(11)
}

func buildClaims(user model.UserInfo, sessionID string, tokenType TokenType, expireAt time.Time) *Claims { // 定义一个函数：根据用户信息、会话ID、token类型和过期时间，构建JWT载荷并返回
	conf := config.GetConfig().JwtConfig // 读取项目配置中的 JWT 配置，后面要用 issuer、subject
	now := time.Now()                    // 记录当前时间，作为 token 的签发时间 iat
	return &Claims{                      // 返回一个 Claims 结构体指针
		UserID:    user.Id,        // 把用户数据库主键写入 claims，后续鉴权时可直接识别用户
		UserUUID:  user.Uuid,      // 把用户业务 UUID 写入 claims
		Telephone: user.Telephone, // 把手机号写入 claims，便于业务侧直接使用
		IsAdmin:   user.IsAdmin,   // 把管理员标记写入 claims，后续可用于权限判断
		SessionID: sessionID,      // 写入当前登录会话 ID，用于 access/refresh token 绑定同一会话
		TokenType: tokenType,      // 写入 token 类型，标记这是 access token 还是 refresh token
		RegisteredClaims: jwt.RegisteredClaims{ // JWT 标准字段区域
			ExpiresAt: jwt.NewNumericDate(expireAt), // exp：token 过期时间
			IssuedAt:  jwt.NewNumericDate(now),      // iat：token 签发时间
			Issuer:    conf.Issuer,                  // iss：签发者，来自配置
			Subject:   conf.Subject,                 // sub：主题，来自配置，一般表示这类 token 的用途
		},
	}
}

func signClaims(claims *Claims) (string, error) {
	key, err := signingKey() //?去配置拿到jwt密钥
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims) //? 使用 HS256 签名算法和 claims 构造一个 JWT 对象，这一步还只是内存里的 token
	return token.SignedString(key)                             //? 用密钥 key 对 JWT 对象做签名，返回最终可发给前端的 token 字符串
}

func GenerateAccessToken(user model.UserInfo, sessionID string) (string, error) {
	if sessionID == "" {
		return "", ErrEmptySessionID
	}
	//? 计算 access token 的过期时间, 用“当前时间 + 这个时间跨度”，得到最终过期时间
	expireAt := time.Now().Add(time.Duration(config.GetConfig().JwtConfig.AccessExpireMinutes) * time.Minute)
	//?buildClaims(...)返回 *Claims也就是一个 JWT 载荷对象
	//?返回的载荷对象被交给signClaims签名
	return signClaims(buildClaims(user, sessionID, AccessTokenType, expireAt))
}

func GenerateRefreshToken(user model.UserInfo, sessionID string) (string, error) {
	if sessionID == "" {
		return "", ErrEmptySessionID
	}
	//? //? 计算 refresh token 的过期时间, 用“当前时间 + 这个时间跨度”，得到最终过期时间
	expireAt := time.Now().Add(time.Duration(config.GetConfig().JwtConfig.RefreshExpireHours) * time.Hour)
	return signClaims(buildClaims(user, sessionID, RefreshTokenType, expireAt))
}

func GenerateTokenPair(user model.UserInfo) (*TokenPair, error) {
	//?这里再包装一层,多传入一个sessionID,通过newSessionID()获取
	return GenerateTokenPairWithSession(user, newSessionID())
}

func GenerateTokenPairWithSession(user model.UserInfo, sessionID string) (*TokenPair, error) {

	//?传入一个user模型和会话创建accessToken
	accessToken, err := GenerateAccessToken(user, sessionID)
	if err != nil {
		return nil, err
	}
	//?传入一个user模型和会话创建RefreshToken
	refreshToken, err := GenerateRefreshToken(user, sessionID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		SessionID:    sessionID,
	}, nil
}

func ParseToken(tokenString string) (*Claims, error) {
	if tokenString == "" { //? 如果传进来的 token 字符串为空，直接返回“空 token”错误
		return nil, ErrEmptyToken
	}
	key, err := signingKey() //? 读取 JWT 签名密钥，后面校验签名时要用
	if err != nil {          //? 如果拿不到密钥，说明配置有问题，直接返回错误
		return nil, err
	}
	claims := new(Claims) //? 先创建一个空的 Claims 对象，用来接收解析出来的 JWT 载荷

	// 定义验签规则：先检查算法，再把密钥交给 jwt 库
	keyFunc := func(token *jwt.Token) (interface{}, error) { 
		if token.Method != jwt.SigningMethodHS256 { // 额外校验签名算法，当前项目只接受 HS256
		return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return key, nil //? 把签名密钥返回给 jwt 库，用它来校验这个 token 的签名是否正确
	}

	//? 解析 token 字符串，并把载荷反序列化到 claims 里
	token, err := jwt.ParseWithClaims(
		tokenString,//主要就是对传入的未知tokenString,尝试解析他,验证他的合法性,返回一个标准的jwt库的token,其次是将负载内容赋值到空claims
		claims,
		keyFunc,//这个是交key给jwt库去校验的方法,把key交给库去验证合法性,但是这个方法在传入key前要验证是否是相对应的签名算法
	)
	if err != nil { //? 只要解析过程出错，比如签名不对、格式错误、过期等，就直接返回错误
		return nil, err//?负责接住“解析失败 / 签名不对 / 已过期 / 算法不对”等明确错误
	}
	//? 如果 token 对象为空，或者校验后被判定为无效 token，就返回无效 token 错误,这里更像是兜底,防止解析结果对象本身无效
	if token == nil || !token.Valid {
		return nil, ErrInvalidToken//?负责兜底“虽然没走到前面的错误分支，但这个 token 最终仍然不合法”的情况
	}
	//? 这个项目要求 token 里必须带 session_id，没有就认为 token 非法
	if claims.SessionID == "" {
		return nil, ErrEmptySessionID
	}
	return claims, nil //? 解析和校验都通过，返回解析后的 claims
}

func ParseTokenAs(tokenString string, expectedType TokenType) (*Claims, error) {
	//核心解析负载数据的部分
	claims, err := ParseToken(tokenString)
	if err != nil {
		return nil, err
	}
	//看看解析出来的数据是否是期待的token类型,不是的话要重新
	if claims.TokenType != expectedType {
		return nil, ErrInvalidType
	}
	return claims, nil
}
