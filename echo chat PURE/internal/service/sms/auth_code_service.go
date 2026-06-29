package sms

// 本文件封装短信验证码相关逻辑，包括验证码生成、缓存和短信发送。

import (
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi20170525 "github.com/alibabacloud-go/dysmsapi-20170525/v4/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"go.uber.org/zap"
	"echo_chat_server/internal/config"
	"echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/constants"
	"echo_chat_server/pkg/util/random"
	"echo_chat_server/pkg/zlog"
	"strconv"
	"time"
)

// smsClient 缓存阿里云短信客户端，避免每次发短信都重复建连。
var smsClient *dysmsapi20170525.Client

// createClient 使用 AK/SK 初始化短信客户端，并在首次调用后复用实例。
func createClient() (result *dysmsapi20170525.Client, err error) {
	// 工程代码泄露可能会导致 AccessKey 泄露，并威胁账号下所有资源的安全性。以下代码示例仅供参考。
	// 建议使用更安全的 STS 方式，更多鉴权访问方式请参见：https://help.aliyun.com/document_detail/378661.html。
	accessKeyID := config.GetConfig().AccessKeyID
	accessKeySecret := config.GetConfig().AccessKeySecret
	if smsClient == nil {
		config := &openapi.Config{
			// 必填，请确保代码运行环境设置了环境变量 ALIBABA_CLOUD_ACCESS_KEY_ID。
			AccessKeyId: tea.String(accessKeyID),
			// 必填，请确保代码运行环境设置了环境变量 ALIBABA_CLOUD_ACCESS_KEY_SECRET。
			AccessKeySecret: tea.String(accessKeySecret),
		}
		// Endpoint 请参考 https://api.aliyun.com/product/Dysmsapi
		config.Endpoint = tea.String("dysmsapi.aliyuncs.com")
		smsClient, err = dysmsapi20170525.NewClient(config)
	}
	return smsClient, err
}

// VerificationCode 负责生成验证码、写入 Redis，并在可用时调用短信服务发送。
func VerificationCode(telephone string) (string, int) {
	key := "auth_code_" + telephone
	code, err := redis.GetKey(key)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}

	if code != "" {
		// 已存在未过期验证码时直接提示复用，避免短时间内频繁发送。
		message := "目前还不能发送验证码，请输入已发送的验证码"
		zlog.Info(message)
		return message, -2
	}
	// 验证码不存在或已过期时重新生成六位随机码。
	code = strconv.Itoa(random.GetRandomInt(6))
	zlog.Info(
		"sms.code.generated",
		zap.String("event", "sms.code.generated"),
		zap.String("telephone", telephone),
		zap.Int("code_len", len(code)),
	)
	err = redis.SetKeyEx(key, code, time.Minute) // 1分钟有效
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if config.GetConfig().AccessKeyID == "" || config.GetConfig().AccessKeySecret == "" {
		// 本地开发场景下允许直接把验证码透出给前端，便于联调。
		zlog.Warn("sms credentials are empty, using local development verification code")
		return "验证码已生成，当前开发环境验证码为: " + code, 0
	}
	client, err := createClient()
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	sendSmsRequest := &dysmsapi20170525.SendSmsRequest{
		SignName:      tea.String("阿里云短信测试"),
		TemplateCode:  tea.String("SMS_154950909"), // 短信模板
		PhoneNumbers:  tea.String(telephone),
		TemplateParam: tea.String("{\"code\":\"" + code + "\"}"),
	}

	runtime := &util.RuntimeOptions{}
	// 当前仓库默认走测试签名和模板；线上部署时通常需要替换为正式配置。
	rsp, err := client.SendSmsWithOptions(sendSmsRequest, runtime)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	zlog.Info(*util.ToJSONString(rsp))
	return "验证码发送成功，请及时在对应电话查收短信", 0
}
