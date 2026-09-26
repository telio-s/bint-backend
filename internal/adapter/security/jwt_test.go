package security

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/telio-s/bint-backend.git/internal/domain/apperror"
)

func TestJWTManagerRoundTrip(t *testing.T) {
	manager := NewJWTManager(testLogger(), JWTConfig{
		Secret: "a-test-secret-that-is-at-least-32-characters", Issuer: "test-issuer",
		Audience: "test-audience", TTL: 15 * time.Minute,
	})
	token, expiry, err := manager.IssueAccessToken("user-123", time.Now().UTC())
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}
	claims, err := manager.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken() error = %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %q, want user-123", claims.UserID)
	}
	if difference := claims.Expiry.Sub(
		expiry,
	); difference <= -time.Second ||
		difference >= time.Second {
		t.Errorf("Expiry = %v, want %v", claims.Expiry, expiry)
	}
}

func TestJWTManagerRejectsInvalidToken(t *testing.T) {
	manager := NewJWTManager(testLogger(), JWTConfig{
		Secret: "a-test-secret-that-is-at-least-32-characters", Issuer: "test-issuer",
		Audience: "test-audience", TTL: 15 * time.Minute,
	})
	_, err := manager.VerifyAccessToken("not-a-jwt")
	if !errors.Is(err, apperror.ErrInvalidToken) {
		t.Fatalf("VerifyAccessToken() error = %v, want ErrInvalidToken", err)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
