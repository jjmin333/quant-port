package model

import "strings"

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	DefaultAdminEmail = "jjmin321" // 로그인 아이디 (2026-09-28 이메일에서 변경)
)

func IsAdminUser(email, role string) bool {
	if role == RoleAdmin {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(email), DefaultAdminEmail)
}
