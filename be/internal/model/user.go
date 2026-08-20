package model

import "time"

type User struct {
	ID          int64      `json:"id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"displayName"`
	Role        string     `json:"role"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type UserPublic struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	IsAdmin     bool   `json:"isAdmin"`
}

func (u User) Public() UserPublic {
	role := u.Role
	if role == "" {
		role = RoleUser
	}
	return UserPublic{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        role,
		IsAdmin:     IsAdminUser(u.Email, role),
	}
}

type AuthSession struct {
	User        UserPublic `json:"user"`
	AccessToken string     `json:"accessToken"`
	ExpiresIn   int        `json:"expiresIn"`
}
