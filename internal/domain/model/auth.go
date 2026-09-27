package model

import "time"

type User struct {
	ID            string
	Email         string
	DisplayName   string
	AvatarURL     string
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type GoogleIdentity struct {
	Subject       string
	Email         string
	DisplayName   string
	AvatarURL     string
	EmailVerified bool
}

type RefreshToken struct {
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type AuthResult struct {
	User         User
	AccessToken  string
	RefreshToken string
	AccessExpiry time.Time
}

type AccessClaims struct {
	UserID string
	Expiry time.Time
}
