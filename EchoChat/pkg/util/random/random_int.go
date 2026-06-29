package random

// 本文件提供随机数与随机字符串的生成工具，供 ID 和验证码场景使用。

import (
	"math"
	"math/rand"
	"strconv"
	"time"
)

// GetRandomInt 生成指定长度的正整数，常用于短信验证码或随机后缀。
func GetRandomInt(len int) int {
	return rand.Intn(9*int(math.Pow(10, float64(len-1)))) + int(math.Pow(10, float64(len-1)))
}

// GetNowAndLenRandomString 把当前日期与随机数拼接成字符串，适合生成业务主键后缀。
func GetNowAndLenRandomString(len int) string {
	return time.Now().Format("20060102") + strconv.Itoa(GetRandomInt(len))
}
