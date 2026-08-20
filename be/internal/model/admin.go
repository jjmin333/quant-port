package model

import "strings"

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	DefaultAdminEmail = "jjmin321@naver.com"
)

func IsAdminUser(email, role string) bool {
	if role == RoleAdmin {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(email), DefaultAdminEmail)
}
