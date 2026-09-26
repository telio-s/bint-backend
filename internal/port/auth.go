package port

import (
	"context"
	"time"

	"github.com/telio-s/bint-backend.git/internal/domain/model"
)

type AuthService interface {
	RegisterEmail(
		ctx context.Context,
		email, password, displayName string,
	) (model.AuthResult, error)
	LoginEmail(ctx context.Context, email, password string) (model.AuthResult, error)
	GoogleAuthorizationURL(state, nonce, codeChallenge string) string
	LoginGoogle(
		ctx context.Context,
		code, codeVerifier, expectedNonce string,
	) (model.AuthResult, error)
	Refresh(ctx context.Context, refreshToken string) (model.AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
	CurrentUser(ctx context.Context, userID string) (model.User, error)
}

type AuthRepository interface {
	CreateEmailUser(ctx context.Context, user model.User, passwordHash string) (model.User, error)
	GetUserWithPasswordByEmail(ctx context.Context, email string) (model.User, string, error)
	FindOrCreateGoogleUser(
		ctx context.Context,
		user model.User,
		identity model.GoogleIdentity,
	) (model.User, error)
	GetUserByID(ctx context.Context, userID string) (model.User, error)
	StoreRefreshToken(ctx context.Context, token model.RefreshToken) error
	RotateRefreshToken(
		ctx context.Context,
		currentTokenHash string,
		replacement model.RefreshToken,
	) (model.User, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenManager interface {
	IssueAccessToken(userID string, now time.Time) (string, time.Time, error)
	VerifyAccessToken(token string) (model.AccessClaims, error)
}

type GoogleOAuth interface {
	AuthorizationURL(state, nonce, codeChallenge string) string
	Exchange(
		ctx context.Context,
		code, codeVerifier, expectedNonce string,
	) (model.GoogleIdentity, error)
}
