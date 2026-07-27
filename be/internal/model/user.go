package model

import "time"

type User struct {
	ID           int64      `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"displayName"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type UserPublic struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

func (u User) Public() UserPublic {
	return UserPublic{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
	}
}

type AuthSession struct {
	User         UserPublic `json:"user"`
	AccessToken  string     `json:"accessToken"`
	ExpiresIn    int        `json:"expiresIn"`
	BrokerLinked bool       `json:"brokerLinked"`
}

type BrokerCredential struct {
	ID               int64
	UserID           int64
	BrokerCode       string
	ClientIDEnc      []byte
	ClientSecretEnc  []byte
	ClientIDHint     string
	IsActive         bool
	LinkedAt         time.Time
	UpdatedAt        time.Time
}

type BrokerAccount struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"userId"`
	BrokerCode  string    `json:"brokerCode"`
	AccountName string    `json:"accountName"`
	AccountSeq  int64     `json:"accountSeq"`
	AccountNo   string    `json:"accountNo,omitempty"`
	AccountType string    `json:"accountType,omitempty"`
	IsPrimary   bool      `json:"isPrimary"`
	SyncedAt    time.Time `json:"syncedAt,omitempty"`
}

type BrokerLinkStatus struct {
	Linked       bool            `json:"linked"`
	BrokerCode   string          `json:"brokerCode"`
	ClientIDHint string          `json:"clientIdHint,omitempty"`
	Accounts     []BrokerAccount `json:"accounts,omitempty"`
	LinkedAt     string          `json:"linkedAt,omitempty"`
}
