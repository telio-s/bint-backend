package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/telio-s/bint-backend.git/internal/domain/apperror"
	"github.com/telio-s/bint-backend.git/internal/domain/model"
	"github.com/telio-s/bint-backend.git/internal/port"
)

const (
	minimumPasswordLength = 8
	maximumPasswordLength = 72
)

type AuthConfig struct {
	RefreshTokenTTL time.Duration
}

type authService struct {
	logger     *slog.Logger
	repository port.AuthRepository
	hasher     port.PasswordHasher
	tokens     port.TokenManager
	google     port.GoogleOAuth
	config     AuthConfig
	now        func() time.Time
}

func NewAuthService(
	logger *slog.Logger,
	repository port.AuthRepository,
	hasher port.PasswordHasher,
	tokens port.TokenManager,
	google port.GoogleOAuth,
	config AuthConfig,
) port.AuthService {
	return &authService{
		logger:     logger.With("component", "auth_service"),
		repository: repository,
		hasher:     hasher,
		tokens:     tokens,
		google:     google,
		config:     config,
		now:        time.Now,
	}
}

func (s *authService) RegisterEmail(
	ctx context.Context,
	email, password, displayName string,
) (model.AuthResult, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return model.AuthResult{}, err
	}
	if err := validatePassword(password); err != nil {
		return model.AuthResult{}, err
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	now := s.now().UTC()
	user, err := s.repository.CreateEmailUser(ctx, model.User{
		ID:          uuid.NewString(),
		Email:       email,
		DisplayName: strings.TrimSpace(displayName),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, passwordHash)
	if err != nil {
		s.logger.WarnContext(ctx, "email registration failed", "error", err)
		return model.AuthResult{}, fmt.Errorf("create email user: %w", err)
	}

	return s.createAuthResult(ctx, user)
}

func (s *authService) LoginEmail(
	ctx context.Context,
	email, password string,
) (model.AuthResult, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return model.AuthResult{}, apperror.ErrInvalidCredentials
	}

	user, passwordHash, err := s.repository.GetUserWithPasswordByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			s.logger.WarnContext(ctx, "email login rejected", "reason", "user_not_found")
			return model.AuthResult{}, apperror.ErrInvalidCredentials
		}
		return model.AuthResult{}, fmt.Errorf("find email user: %w", err)
	}
	if err := s.hasher.Compare(passwordHash, password); err != nil {
		s.logger.WarnContext(ctx, "email login rejected", "reason", "password_mismatch")
		return model.AuthResult{}, apperror.ErrInvalidCredentials
	}

	return s.createAuthResult(ctx, user)
}

func (s *authService) GoogleAuthorizationURL(state, nonce, codeChallenge string) string {
	return s.google.AuthorizationURL(state, nonce, codeChallenge)
}

func (s *authService) LoginGoogle(
	ctx context.Context,
	code, codeVerifier, expectedNonce string,
) (model.AuthResult, error) {
	identity, err := s.google.Exchange(ctx, code, codeVerifier, expectedNonce)
	if err != nil {
		s.logger.WarnContext(ctx, "Google authentication failed", "error", err)
		return model.AuthResult{}, fmt.Errorf("verify Google identity: %w", err)
	}
	if !identity.EmailVerified {
		return model.AuthResult{}, fmt.Errorf(
			"Google email is not verified: %w",
			apperror.ErrInvalidCredentials,
		)
	}

	now := s.now().UTC()
	user, err := s.repository.FindOrCreateGoogleUser(ctx, model.User{
		ID:            uuid.NewString(),
		Email:         strings.ToLower(identity.Email),
		DisplayName:   identity.DisplayName,
		AvatarURL:     identity.AvatarURL,
		EmailVerified: true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, identity)
	if err != nil {
		s.logger.ErrorContext(ctx, "Google user persistence failed", "error", err)
		return model.AuthResult{}, fmt.Errorf("find or create Google user: %w", err)
	}

	return s.createAuthResult(ctx, user)
}

func (s *authService) Refresh(
	ctx context.Context,
	rawRefreshToken string,
) (model.AuthResult, error) {
	if rawRefreshToken == "" {
		return model.AuthResult{}, apperror.ErrInvalidToken
	}

	replacementRaw, replacement, err := s.newRefreshToken("")
	if err != nil {
		return model.AuthResult{}, err
	}
	user, err := s.repository.RotateRefreshToken(ctx, hashToken(rawRefreshToken), replacement)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			s.logger.WarnContext(ctx, "refresh token rejected")
			return model.AuthResult{}, apperror.ErrInvalidToken
		}
		return model.AuthResult{}, fmt.Errorf("rotate refresh token: %w", err)
	}

	accessToken, accessExpiry, err := s.tokens.IssueAccessToken(user.ID, s.now().UTC())
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("issue access token: %w", err)
	}
	return model.AuthResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: replacementRaw,
		AccessExpiry: accessExpiry,
	}, nil
}

func (s *authService) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}
	if err := s.repository.RevokeRefreshToken(
		ctx,
		hashToken(rawRefreshToken),
	); err != nil &&
		!errors.Is(err, apperror.ErrNotFound) {
		s.logger.ErrorContext(ctx, "refresh token revocation failed", "error", err)
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (s *authService) CurrentUser(ctx context.Context, userID string) (model.User, error) {
	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return model.User{}, fmt.Errorf("get current user: %w", err)
	}
	return user, nil
}

func (s *authService) createAuthResult(
	ctx context.Context,
	user model.User,
) (model.AuthResult, error) {
	accessToken, accessExpiry, err := s.tokens.IssueAccessToken(user.ID, s.now().UTC())
	if err != nil {
		return model.AuthResult{}, fmt.Errorf("issue access token: %w", err)
	}

	refreshRaw, refresh, err := s.newRefreshToken(user.ID)
	if err != nil {
		return model.AuthResult{}, err
	}
	if err := s.repository.StoreRefreshToken(ctx, refresh); err != nil {
		return model.AuthResult{}, fmt.Errorf("store refresh token: %w", err)
	}

	return model.AuthResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshRaw,
		AccessExpiry: accessExpiry,
	}, nil
}

func (s *authService) newRefreshToken(userID string) (string, model.RefreshToken, error) {
	raw, err := randomToken(32)
	if err != nil {
		return "", model.RefreshToken{}, fmt.Errorf("generate refresh token: %w", err)
	}
	now := s.now().UTC()
	return raw, model.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: hashToken(raw),
		ExpiresAt: now.Add(s.config.RefreshTokenTTL),
		CreatedAt: now,
	}, nil
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return "", fmt.Errorf("invalid email: %w", apperror.ErrInvalidInput)
	}
	return value, nil
}

func validatePassword(password string) error {
	if len(password) < minimumPasswordLength || len(password) > maximumPasswordLength {
		return fmt.Errorf(
			"password must be between %d and %d bytes: %w",
			minimumPasswordLength,
			maximumPasswordLength,
			apperror.ErrInvalidInput,
		)
	}
	return nil
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
