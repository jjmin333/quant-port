package model

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// IsAdminRole — 관리자는 users.role 하나로만 정한다. 예전에는 특정 로그인 아이디도
// 관리자로 봤는데(그 아이디로 가입하면 role=admin), 그 계정이 사라지면 누구나 그
// 아이디로 가입해 관리자가 될 수 있었다(2026-11-02 제거). 관리자 지정은 DB에서
// `UPDATE users SET role='admin' WHERE id=?`.
func IsAdminRole(role string) bool {
	return role == RoleAdmin
}
