package dto

import (
	"time"

	"github.com/telio-s/bint-backend.git/internal/domain/model"
)

type RegisterEmailRequest struct {
	Email       string `json:"email"       binding:"required,email"`
	Password    string `json:"password"    binding:"required,min=8,max=72"`
	DisplayName string `json:"displayName" binding:"required,max=100"`
}

type LoginEmailRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"displayName"`
	AvatarURL     string    `json:"avatarUrl,omitempty"`
	EmailVerified bool      `json:"emailVerified"`
	CreatedAt     time.Time `json:"createdAt"`
}

type AuthResponse struct {
	AccessToken string       `json:"accessToken"`
	TokenType   string       `json:"tokenType"`
	ExpiresAt   time.Time    `json:"expiresAt"`
	User        UserResponse `json:"user"`
}

type SessionResponse struct {
	Authenticated bool         `json:"authenticated"`
	User          UserResponse `json:"user"`
}

func AuthResponseFromModel(result model.AuthResult) AuthResponse {
	return AuthResponse{
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   result.AccessExpiry,
		User:        UserResponseFromModel(result.User),
	}
}

func UserResponseFromModel(user model.User) UserResponse {
	return UserResponse{
		ID: user.ID, Email: user.Email, DisplayName: user.DisplayName,
		AvatarURL: user.AvatarURL, EmailVerified: user.EmailVerified, CreatedAt: user.CreatedAt,
	}
}
