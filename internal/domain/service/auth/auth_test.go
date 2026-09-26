package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/telio-s/bint-backend.git/internal/domain/apperror"
	"github.com/telio-s/bint-backend.git/internal/domain/model"
)

type fakeAuthRepository struct {
	usersByEmail map[string]model.User
	passwords    map[string]string
	refreshUsers map[string]model.User
}

func newFakeAuthRepository() *fakeAuthRepository {
	return &fakeAuthRepository{
		usersByEmail: make(map[string]model.User),
		passwords:    make(map[string]string),
		refreshUsers: make(map[string]model.User),
	}
}

func (r *fakeAuthRepository) CreateEmailUser(
	_ context.Context,
	user model.User,
	hash string,
) (model.User, error) {
	if _, exists := r.usersByEmail[user.Email]; exists {
		return model.User{}, apperror.ErrConflict
	}
	r.usersByEmail[user.Email] = user
	r.passwords[user.Email] = hash
	return user, nil
}

func (r *fakeAuthRepository) GetUserWithPasswordByEmail(
	_ context.Context,
	email string,
) (model.User, string, error) {
	user, exists := r.usersByEmail[email]
	if !exists || r.passwords[email] == "" {
		return model.User{}, "", apperror.ErrNotFound
	}
	return user, r.passwords[email], nil
}

func (r *fakeAuthRepository) FindOrCreateGoogleUser(
	_ context.Context,
	user model.User,
	_ model.GoogleIdentity,
) (model.User, error) {
	if existing, exists := r.usersByEmail[user.Email]; exists {
		return existing, nil
	}
	r.usersByEmail[user.Email] = user
	return user, nil
}

func (r *fakeAuthRepository) GetUserByID(_ context.Context, userID string) (model.User, error) {
	for _, user := range r.usersByEmail {
		if user.ID == userID {
			return user, nil
		}
	}
	return model.User{}, apperror.ErrNotFound
}

func (r *fakeAuthRepository) StoreRefreshToken(_ context.Context, token model.RefreshToken) error {
	user, err := r.GetUserByID(context.Background(), token.UserID)
	if err != nil {
		return err
	}
	r.refreshUsers[token.TokenHash] = user
	return nil
}

func (r *fakeAuthRepository) RotateRefreshToken(
	_ context.Context,
	currentHash string,
	replacement model.RefreshToken,
) (model.User, error) {
	user, exists := r.refreshUsers[currentHash]
	if !exists {
		return model.User{}, apperror.ErrNotFound
	}
	delete(r.refreshUsers, currentHash)
	r.refreshUsers[replacement.TokenHash] = user
	return user, nil
}

func (r *fakeAuthRepository) RevokeRefreshToken(_ context.Context, tokenHash string) error {
	if _, exists := r.refreshUsers[tokenHash]; !exists {
		return apperror.ErrNotFound
	}
	delete(r.refreshUsers, tokenHash)
	return nil
}

type fakePasswordHasher struct{}

func (fakePasswordHasher) Hash(password string) (string, error) { return "hashed:" + password, nil }
func (fakePasswordHasher) Compare(hash, password string) error {
	if hash != "hashed:"+password {
		return errors.New("password mismatch")
	}
	return nil
}

type fakeTokenManager struct{ issued int }

func (m *fakeTokenManager) IssueAccessToken(_ string, now time.Time) (string, time.Time, error) {
	m.issued++
	return "access-token", now.Add(15 * time.Minute), nil
}

func (*fakeTokenManager) VerifyAccessToken(string) (model.AccessClaims, error) {
	return model.AccessClaims{}, nil
}

type fakeGoogleOAuth struct{ identity model.GoogleIdentity }

func (f fakeGoogleOAuth) AuthorizationURL(_, _, _ string) string {
	return "https://accounts.google.com"
}

func (f fakeGoogleOAuth) Exchange(
	context.Context,
	string,
	string,
	string,
) (model.GoogleIdentity, error) {
	return f.identity, nil
}

func TestAuthMethodsIssueTheSameBintTokenType(t *testing.T) {
	repository := newFakeAuthRepository()
	tokens := &fakeTokenManager{}
	google := fakeGoogleOAuth{identity: model.GoogleIdentity{
		Subject:       "google-subject",
		Email:         "google@example.com",
		DisplayName:   "Google User",
		EmailVerified: true,
	}}
	service := NewAuthService(
		testLogger(),
		repository,
		fakePasswordHasher{},
		tokens,
		google,
		AuthConfig{RefreshTokenTTL: 24 * time.Hour},
	)

	emailResult, err := service.RegisterEmail(
		context.Background(),
		"EMAIL@example.com",
		"password123",
		"Email User",
	)
	if err != nil {
		t.Fatalf("RegisterEmail() error = %v", err)
	}
	googleResult, err := service.LoginGoogle(context.Background(), "code", "verifier", "nonce")
	if err != nil {
		t.Fatalf("LoginGoogle() error = %v", err)
	}

	for name, result := range map[string]model.AuthResult{"email": emailResult, "google": googleResult} {
		t.Run(name, func(t *testing.T) {
			if result.AccessToken != "access-token" {
				t.Errorf("AccessToken = %q, want Bint access token", result.AccessToken)
			}
			if result.RefreshToken == "" {
				t.Error("RefreshToken is empty")
			}
		})
	}
	if tokens.issued != 2 {
		t.Errorf("issued access tokens = %d, want 2", tokens.issued)
	}
}

func TestRefreshRotatesOpaqueToken(t *testing.T) {
	repository := newFakeAuthRepository()
	tokens := &fakeTokenManager{}
	service := NewAuthService(
		testLogger(),
		repository,
		fakePasswordHasher{},
		tokens,
		fakeGoogleOAuth{},
		AuthConfig{RefreshTokenTTL: 24 * time.Hour},
	)

	registered, err := service.RegisterEmail(
		context.Background(),
		"person@example.com",
		"password123",
		"Person",
	)
	if err != nil {
		t.Fatalf("RegisterEmail() error = %v", err)
	}
	refreshed, err := service.Refresh(context.Background(), registered.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.RefreshToken == registered.RefreshToken {
		t.Error("refresh token was not rotated")
	}
	if _, err := service.Refresh(
		context.Background(),
		registered.RefreshToken,
	); !errors.Is(
		err,
		apperror.ErrInvalidToken,
	) {
		t.Errorf("reusing rotated refresh token error = %v, want ErrInvalidToken", err)
	}
}

func TestLoginEmailHidesCredentialFailureReason(t *testing.T) {
	service := NewAuthService(
		testLogger(),
		newFakeAuthRepository(),
		fakePasswordHasher{},
		&fakeTokenManager{},
		fakeGoogleOAuth{},
		AuthConfig{RefreshTokenTTL: time.Hour},
	)

	_, err := service.LoginEmail(context.Background(), "missing@example.com", "wrong-password")
	if !errors.Is(err, apperror.ErrInvalidCredentials) {
		t.Fatalf("LoginEmail() error = %v, want ErrInvalidCredentials", err)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
