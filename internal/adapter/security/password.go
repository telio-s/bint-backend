package security

import (
	"log/slog"

	"golang.org/x/crypto/bcrypt"
)

type PasswordHasher struct {
	logger *slog.Logger
	cost   int
}

func NewPasswordHasher(logger *slog.Logger) *PasswordHasher {
	return &PasswordHasher{
		logger: logger.With("component", "password_hasher"),
		cost:   bcrypt.DefaultCost,
	}
}

func (h *PasswordHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		h.logger.Error("password hashing failed", "error", err)
	}
	return string(hash), err
}

func (h *PasswordHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
