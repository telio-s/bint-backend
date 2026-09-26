package security

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/telio-s/bint-backend.git/internal/domain/apperror"
	"github.com/telio-s/bint-backend.git/internal/domain/model"
)

type JWTConfig struct {
	Secret   string
	Issuer   string
	Audience string
	TTL      time.Duration
}

type JWTManager struct {
	logger   *slog.Logger
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
}

type accessTokenClaims struct {
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

func NewJWTManager(logger *slog.Logger, config JWTConfig) *JWTManager {
	return &JWTManager{
		logger: logger.With("component", "jwt_manager"), secret: []byte(config.Secret),
		issuer: config.Issuer, audience: config.Audience, ttl: config.TTL,
	}
}

func (m *JWTManager) IssueAccessToken(userID string, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(m.ttl)
	claims := accessTokenClaims{
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: m.issuer, Subject: userID, Audience: jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt), IssuedAt: jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now), ID: uuid.NewString(),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		m.logger.Error("access token signing failed", "error", err)
		return "", time.Time{}, fmt.Errorf("sign JWT: %w", err)
	}
	return token, expiresAt, nil
}

func (m *JWTManager) VerifyAccessToken(rawToken string) (model.AccessClaims, error) {
	claims := &accessTokenClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(m.issuer), jwt.WithAudience(m.audience), jwt.WithExpirationRequired())
	if err != nil || !token.Valid || claims.TokenType != "access" || claims.Subject == "" ||
		claims.ExpiresAt == nil {
		m.logger.Debug("access token verification failed", "error", err)
		return model.AccessClaims{}, fmt.Errorf(
			"verify access JWT: %w",
			errors.Join(err, apperror.ErrInvalidToken),
		)
	}
	return model.AccessClaims{UserID: claims.Subject, Expiry: claims.ExpiresAt.Time}, nil
}
