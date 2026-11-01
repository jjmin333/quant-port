package model

import "testing"

// 예전에는 아이디 jjmin321이면 role과 무관하게 관리자였다 — 이제 role만 본다.
func TestPublicIsAdminFollowsRoleOnly(t *testing.T) {
	if (User{Email: "jjmin321", Role: RoleUser}).Public().IsAdmin {
		t.Fatal("login id alone must not grant admin")
	}
	if !(User{Email: "anyone", Role: RoleAdmin}).Public().IsAdmin {
		t.Fatal("role=admin must grant admin")
	}
}
