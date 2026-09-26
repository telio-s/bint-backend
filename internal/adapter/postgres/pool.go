package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(logger *slog.Logger, dsn string) (*pgxpool.Pool, error) {
	logger = logger.With("component", "postgres_pool")
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		logger.Error("PostgreSQL pool creation failed", "error", err)
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	logger.Debug("PostgreSQL pool configured")
	return pool, nil
}
