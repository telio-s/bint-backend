package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	httpadapter "github.com/telio-s/bint-backend.git/internal/adapter/http"
	"github.com/telio-s/bint-backend.git/internal/adapter/http/handler"
	googleoauth "github.com/telio-s/bint-backend.git/internal/adapter/oauth/google"
	"github.com/telio-s/bint-backend.git/internal/adapter/postgres"
	"github.com/telio-s/bint-backend.git/internal/adapter/security"
	authservice "github.com/telio-s/bint-backend.git/internal/domain/service/auth"
	"github.com/telio-s/bint-backend.git/internal/infra/config"
	"github.com/telio-s/bint-backend.git/internal/port"
	"github.com/telio-s/bint-backend.git/pkg/httpjson"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(
			config.Load,
			provideLogger,
			httpjson.NewResponder,
			providePool,
			postgres.NewAuthRepository,
			providePasswordHasher,
			provideJWTManager,
			provideGoogleOAuth,
			provideAuthService,
			provideAuthHandler,
			httpadapter.NewRouter,
			provideHTTPServer,
		),
		fx.Invoke(registerLifecycle),
	).Run()
}

func provideLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Application.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With(
		"application", cfg.Application.Name,
		"environment", cfg.Application.Environment,
	)
}

func providePool(
	lifecycle fx.Lifecycle,
	logger *slog.Logger,
	cfg *config.Config,
) (*pgxpool.Pool, error) {
	logger = logger.With("component", "postgres_lifecycle")
	pool, err := postgres.NewPool(logger, cfg.Database.DSN)
	if err != nil {
		return nil, err
	}
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := pool.Ping(ctx); err != nil {
				logger.ErrorContext(ctx, "PostgreSQL ping failed", "error", err)
				return fmt.Errorf("ping PostgreSQL: %w", err)
			}
			logger.InfoContext(ctx, "PostgreSQL connection established")
			return nil
		},
		OnStop: func(context.Context) error {
			logger.Info("closing PostgreSQL connection pool")
			pool.Close()
			return nil
		},
	})
	return pool, nil
}

func providePasswordHasher(logger *slog.Logger) port.PasswordHasher {
	return security.NewPasswordHasher(logger)
}

func provideJWTManager(logger *slog.Logger, cfg *config.Config) port.TokenManager {
	return security.NewJWTManager(logger, security.JWTConfig{
		Secret: cfg.Auth.JWT.Secret, Issuer: cfg.Auth.JWT.Issuer,
		Audience: cfg.Auth.JWT.Audience, TTL: cfg.Auth.JWT.AccessTokenTTL,
	})
}

func provideGoogleOAuth(logger *slog.Logger, cfg *config.Config) port.GoogleOAuth {
	return googleoauth.New(logger, googleoauth.Config{
		ClientID:     cfg.Auth.Google.ClientID,
		ClientSecret: cfg.Auth.Google.ClientSecret,
		RedirectURL:  cfg.Auth.Google.RedirectURL,
	})
}

func provideAuthService(
	logger *slog.Logger,
	repository port.AuthRepository,
	hasher port.PasswordHasher,
	tokens port.TokenManager,
	google port.GoogleOAuth,
	cfg *config.Config,
) port.AuthService {
	return authservice.NewAuthService(
		logger,
		repository,
		hasher,
		tokens,
		google,
		authservice.AuthConfig{RefreshTokenTTL: cfg.Auth.JWT.RefreshTokenTTL},
	)
}

func provideAuthHandler(
	logger *slog.Logger,
	responder *httpjson.Responder,
	auth port.AuthService,
	tokens port.TokenManager,
	cfg *config.Config,
) *handler.AuthHandler {
	return handler.NewAuthHandler(logger, responder, auth, tokens, handler.AuthConfig{
		CookieDomain:            cfg.Auth.Cookie.Domain,
		CookieSecure:            cfg.Auth.Cookie.Secure,
		RefreshTokenTTL:         cfg.Auth.JWT.RefreshTokenTTL,
		OAuthTemporaryCookieTTL: cfg.Auth.OAuthTemporaryCookieTTL,
		SuccessRedirectURL:      cfg.Auth.SuccessRedirectURL,
		FailureRedirectURL:      cfg.Auth.FailureRedirectURL,
	})
}

func provideHTTPServer(logger *slog.Logger, router *gin.Engine, cfg *config.Config) *http.Server {
	logger.With("component", "http_server").Debug("HTTP server configured", "port", cfg.Server.Port)
	return &http.Server{
		Addr: ":" + cfg.Server.Port, Handler: router,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout, ReadTimeout: cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout, IdleTimeout: cfg.Server.IdleTimeout,
	}
}

func registerLifecycle(
	lifecycle fx.Lifecycle,
	shutdowner fx.Shutdowner,
	logger *slog.Logger,
	server *http.Server,
) {
	logger = logger.With("component", "http_server")
	var listener net.Listener
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			var err error
			listener, err = net.Listen("tcp", server.Addr)
			if err != nil {
				logger.Error("HTTP listener failed", "address", server.Addr, "error", err)
				return fmt.Errorf("listen on %s: %w", server.Addr, err)
			}
			logger.Info("HTTP server started", "address", server.Addr)
			go func() {
				if serveErr := server.Serve(
					listener,
				); serveErr != nil &&
					!errors.Is(serveErr, http.ErrServerClosed) {
					logger.Error("HTTP server failed", "error", serveErr)
					if err := shutdowner.Shutdown(); err != nil {
						logger.Error("application shutdown failed", "error", err)
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.InfoContext(ctx, "HTTP server stopping")
			return server.Shutdown(ctx)
		},
	})
}
