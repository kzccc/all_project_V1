package respond

// 本文件实现 get contactinfo respond 相关逻辑。

import "encoding/json"

// GetContactInfoRespond 描述该接口返回给前端的数据结构。
type GetContactInfoRespond struct {
	// ContactId 保存响应中的 `contact_id` 字段。
	ContactId string `json:"contact_id"`
	// ContactName 保存响应中的 `contact_name` 字段。
	ContactName string `json:"contact_name"`
	// ContactAvatar 保存响应中的 `contact_avatar` 字段。
	ContactAvatar string `json:"contact_avatar"`
	// ContactPhone 保存响应中的 `contact_phone` 字段。
	ContactPhone string `json:"contact_phone"`
	// ContactEmail 保存响应中的 `contact_email` 字段。
	ContactEmail string `json:"contact_email"`
	// ContactGender 保存响应中的 `contact_gender` 字段。
	ContactGender int8 `json:"contact_gender"`
	// ContactSignature 保存响应中的 `contact_signature` 字段。
	ContactSignature string `json:"contact_signature"`
	// ContactBirthday 保存响应中的 `contact_birthday` 字段。
	ContactBirthday string `json:"contact_birthday"`
	// ContactNotice 保存响应中的 `contact_notice` 字段。
	ContactNotice string `json:"contact_notice"`
	// ContactMembers 保存响应中的 `contact_members` 字段。
	ContactMembers json.RawMessage `json:"contact_members"`
	// ContactMemberCnt 保存响应中的 `contact_member_cnt` 字段。
	ContactMemberCnt int `json:"contact_member_cnt"`
	// ContactOwnerId 保存响应中的 `contact_owner_id` 字段。
	ContactOwnerId string `json:"contact_owner_id"`
	// ContactAddMode 保存响应中的 `contact_add_mode` 字段。
	ContactAddMode int8 `json:"contact_add_mode"`
}
