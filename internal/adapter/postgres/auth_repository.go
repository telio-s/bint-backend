package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/telio-s/bint-backend.git/internal/adapter/postgres/sqlc"
	"github.com/telio-s/bint-backend.git/internal/domain/apperror"
	"github.com/telio-s/bint-backend.git/internal/domain/model"
	"github.com/telio-s/bint-backend.git/internal/port"
)

type AuthRepository struct {
	logger  *slog.Logger
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewAuthRepository(logger *slog.Logger, pool *pgxpool.Pool) port.AuthRepository {
	return &AuthRepository{
		logger: logger.With("component", "auth_repository"), pool: pool, queries: sqlc.New(pool),
	}
}

func (r *AuthRepository) CreateEmailUser(
	ctx context.Context,
	user model.User,
	passwordHash string,
) (model.User, error) {
	row, err := r.queries.CreateEmailUser(ctx, sqlc.CreateEmailUserParams{
		Email: user.Email, DisplayName: user.DisplayName, AvatarUrl: user.AvatarURL,
		EmailVerified: user.EmailVerified, PasswordHash: &passwordHash,
		CreatedAt: timestamp(user.CreatedAt), UpdatedAt: timestamp(user.UpdatedAt),
	})
	if err != nil {
		r.logger.ErrorContext(ctx, "email user insert failed", "error", err)
		return model.User{}, mapDatabaseError(err)
	}
	return model.User{
		ID:            row.ID,
		Email:         row.Email,
		DisplayName:   row.DisplayName,
		AvatarURL:     row.AvatarUrl,
		EmailVerified: row.EmailVerified,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

func (r *AuthRepository) GetUserWithPasswordByEmail(
	ctx context.Context,
	email string,
) (model.User, string, error) {
	row, err := r.queries.GetUserWithPasswordByEmail(ctx, email)
	if err != nil {
		r.logger.ErrorContext(ctx, "Google user transaction failed to start", "error", err)
		return model.User{}, "", mapDatabaseError(err)
	}
	if row.PasswordHash == nil {
		return model.User{}, "", apperror.ErrNotFound
	}
	return userFromSQLC(
		row.ID,
		row.Email,
		row.DisplayName,
		row.AvatarUrl,
		row.EmailVerified,
		row.CreatedAt,
		row.UpdatedAt,
	), *row.PasswordHash, nil
}

func (r *AuthRepository) GetUserByID(ctx context.Context, userID string) (model.User, error) {
	row, err := r.queries.GetUserByID(ctx, userID)
	if err != nil {
		r.logger.ErrorContext(ctx, "refresh token transaction failed to start", "error", err)
		return model.User{}, mapDatabaseError(err)
	}
	return userFromSQLC(
		row.ID,
		row.Email,
		row.DisplayName,
		row.AvatarUrl,
		row.EmailVerified,
		row.CreatedAt,
		row.UpdatedAt,
	), nil
}

func (r *AuthRepository) FindOrCreateGoogleUser(
	ctx context.Context,
	candidate model.User,
	identity model.GoogleIdentity,
) (model.User, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.User{}, fmt.Errorf("begin Google user transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)

	googleUser, err := queries.GetUserByGoogleSubject(ctx, identity.Subject)
	if err == nil {
		return userFromSQLC(
			googleUser.ID,
			googleUser.Email,
			googleUser.DisplayName,
			googleUser.AvatarUrl,
			googleUser.EmailVerified,
			googleUser.CreatedAt,
			googleUser.UpdatedAt,
		), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, fmt.Errorf("find Google identity: %w", err)
	}

	var user model.User
	existing, err := queries.GetUserByEmail(ctx, candidate.Email)
	switch {
	case err == nil:
		updated, updateErr := queries.UpdateUserFromGoogle(ctx, sqlc.UpdateUserFromGoogleParams{
			ID: existing.ID, DisplayName: candidate.DisplayName, AvatarUrl: candidate.AvatarURL,
			UpdatedAt: timestamp(candidate.UpdatedAt),
		})
		if updateErr != nil {
			return model.User{}, fmt.Errorf("mark Google email verified: %w", updateErr)
		}
		user = userFromSQLC(
			updated.ID,
			updated.Email,
			updated.DisplayName,
			updated.AvatarUrl,
			updated.EmailVerified,
			updated.CreatedAt,
			updated.UpdatedAt,
		)
	case errors.Is(err, pgx.ErrNoRows):
		created, createErr := queries.CreateGoogleUser(ctx, sqlc.CreateGoogleUserParams{
			Email: candidate.Email, DisplayName: candidate.DisplayName,
			AvatarUrl: candidate.AvatarURL, EmailVerified: candidate.EmailVerified,
			CreatedAt: timestamp(candidate.CreatedAt), UpdatedAt: timestamp(candidate.UpdatedAt),
		})
		if createErr != nil {
			return model.User{}, mapDatabaseError(createErr)
		}
		user = userFromSQLC(
			created.ID,
			created.Email,
			created.DisplayName,
			created.AvatarUrl,
			created.EmailVerified,
			created.CreatedAt,
			created.UpdatedAt,
		)
	default:
		return model.User{}, fmt.Errorf("find user by Google email: %w", err)
	}

	if err := queries.CreateGoogleIdentity(ctx, sqlc.CreateGoogleIdentityParams{
		UserID:          user.ID,
		ProviderSubject: identity.Subject,
		CreatedAt:       timestamp(candidate.CreatedAt),
	}); err != nil {
		return model.User{}, mapDatabaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.User{}, fmt.Errorf("commit Google user transaction: %w", err)
	}
	return user, nil
}

func (r *AuthRepository) StoreRefreshToken(ctx context.Context, token model.RefreshToken) error {
	err := r.queries.StoreRefreshToken(ctx, sqlc.StoreRefreshTokenParams{
		UserID: token.UserID, TokenHash: token.TokenHash,
		ExpiresAt: timestamp(token.ExpiresAt), CreatedAt: timestamp(token.CreatedAt),
	})
	if err != nil {
		return mapDatabaseError(err)
	}
	return nil
}

func (r *AuthRepository) RotateRefreshToken(
	ctx context.Context,
	currentTokenHash string,
	replacement model.RefreshToken,
) (model.User, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.User{}, fmt.Errorf("begin refresh token transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)

	row, err := queries.GetRefreshTokenForUpdate(ctx, currentTokenHash)
	if err != nil {
		return model.User{}, mapDatabaseError(err)
	}
	if _, err := queries.RevokeRefreshToken(ctx, currentTokenHash); err != nil {
		return model.User{}, fmt.Errorf("revoke rotated refresh token: %w", err)
	}
	replacement.UserID = row.ID
	if err := queries.StoreRefreshToken(ctx, sqlc.StoreRefreshTokenParams{
		UserID: replacement.UserID, TokenHash: replacement.TokenHash,
		ExpiresAt: timestamp(replacement.ExpiresAt), CreatedAt: timestamp(replacement.CreatedAt),
	}); err != nil {
		return model.User{}, fmt.Errorf("store rotated refresh token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.User{}, fmt.Errorf("commit refresh token transaction: %w", err)
	}
	return userFromSQLC(
		row.ID,
		row.Email,
		row.DisplayName,
		row.AvatarUrl,
		row.EmailVerified,
		row.CreatedAt,
		row.UpdatedAt,
	), nil
}

func (r *AuthRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	rows, err := r.queries.RevokeRefreshToken(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	if rows == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func userFromSQLC(
	id, email, name, avatar string,
	verified bool,
	createdAt, updatedAt pgtype.Timestamptz,
) model.User {
	return model.User{
		ID: id, Email: email, DisplayName: name, AvatarURL: avatar, EmailVerified: verified,
		CreatedAt: createdAt.Time, UpdatedAt: updatedAt.Time,
	}
}

func mapDatabaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return apperror.ErrConflict
	}
	return err
}
