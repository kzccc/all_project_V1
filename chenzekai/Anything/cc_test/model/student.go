package model

import (
	"errors"
	"regexp"
)

// Student 学生数据模型
type Student struct {
	ID     string `json:"id"`     // 学号（唯一标识）
	Name   string `json:"name"`   // 姓名
	Age    int    `json:"age"`    // 年龄
	Gender string `json:"gender"` // 性别
	Major  string `json:"major"`  // 专业
	Email  string `json:"email"`  // 邮箱
}

// Validate 验证学生信息
func (s *Student) Validate() error {
	if s.ID == "" {
		return errors.New("学号不能为空")
	}
	if s.Name == "" {
		return errors.New("姓名不能为空")
	}
	if s.Age < 1 || s.Age > 150 {
		return errors.New("年龄必须在 1-150 之间")
	}
	if s.Gender != "男" && s.Gender != "女" {
		return errors.New("性别必须是'男'或'女'")
	}
	if s.Major == "" {
		return errors.New("专业不能为空")
	}
	if s.Email != "" && !isValidEmail(s.Email) {
		return errors.New("邮箱格式不正确")
	}
	return nil
}

// isValidEmail 验证邮箱格式
func isValidEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}
