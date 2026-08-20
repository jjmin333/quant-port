package model

import "strings"

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	DefaultAdminEmail = "(관리자 이메일)"
)

func IsAdminUser(email, role string) bool {
	if role == RoleAdmin {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(email), DefaultAdminEmail)
}
