package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

func Load() (*Config, error) {
	// Existing environment variables take precedence over local .env values.
	_ = godotenv.Load()

	server, err := loadServerConfig()
	if err != nil {
		return nil, err
	}
	auth, err := loadAuthConfig()
	if err != nil {
		return nil, err
	}
	database := loadDatabaseConfig()

	config := &Config{
		Application: ApplicationConfig{
			Name:        getEnv("APP_NAME", "bint-backend"),
			Environment: getEnv("APP_ENV", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
		},
		Server:   server,
		Database: database,
		Auth:     auth,
	}

	if len(config.Auth.JWT.Secret) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 characters")
	}
	if config.Auth.Google.ClientID == "" || config.Auth.Google.ClientSecret == "" {
		return nil, errors.New("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are required")
	}
	return config, nil
}

func loadServerConfig() (ServerConfig, error) {
	readHeaderTimeout, err := durationEnv("SERVER_READ_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return ServerConfig{}, err
	}
	readTimeout, err := durationEnv("SERVER_READ_TIMEOUT", 15*time.Second)
	if err != nil {
		return ServerConfig{}, err
	}
	writeTimeout, err := durationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second)
	if err != nil {
		return ServerConfig{}, err
	}
	idleTimeout, err := durationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return ServerConfig{}, err
	}
	return ServerConfig{
		Port: getEnv("PORT", "8080"), ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout: readTimeout, WriteTimeout: writeTimeout, IdleTimeout: idleTimeout,
	}, nil
}

func loadDatabaseConfig() DatabaseConfig {
	database := DatabaseConfig{
		URL:      os.Getenv("DATABASE_URL"),
		Host:     getEnv("DATABASE_HOST", "localhost"),
		Port:     getEnv("DATABASE_PORT", "5432"),
		Username: getEnv("DATABASE_USERNAME", "postgres"),
		Password: getEnv("DATABASE_PASSWORD", "postgres"),
		Name:     getEnv("DATABASE_NAME", "bint") + os.Getenv("APP_ENV"),
	}
	if database.URL != "" {
		database.DSN = database.URL
	} else {
		database.DSN = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable",
			database.Username, database.Password, database.Host, database.Port, database.Name,
		)
	}
	return database
}

func loadAuthConfig() (AuthConfig, error) {
	accessTTL, err := durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return AuthConfig{}, err
	}
	refreshTTL, err := durationEnv("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return AuthConfig{}, err
	}
	oauthCookieTTL, err := durationEnv("OAUTH_TEMPORARY_COOKIE_TTL", 10*time.Minute)
	if err != nil {
		return AuthConfig{}, err
	}
	cookieSecure, err := strconv.ParseBool(getEnv("COOKIE_SECURE", "false"))
	if err != nil {
		return AuthConfig{}, fmt.Errorf("parse COOKIE_SECURE: %w", err)
	}
	return AuthConfig{
		JWT: JWTConfig{
			Secret: os.Getenv("JWT_SECRET"),
			Issuer: getEnv("JWT_ISSUER", "bint-backend"),
			Audience: getEnv(
				"JWT_AUDIENCE",
				"bint-api",
			),
			AccessTokenTTL:  accessTTL,
			RefreshTokenTTL: refreshTTL,
		},
		Google: GoogleConfig{
			ClientID: os.Getenv(
				"GOOGLE_CLIENT_ID",
			),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL: getEnv(
				"GOOGLE_REDIRECT_URL",
				"http://localhost:8080/api/v1/auth/google/callback",
			),
		},
		Cookie: CookieConfig{
			Domain: os.Getenv("COOKIE_DOMAIN"),
			Secure: cookieSecure,
		},
		SuccessRedirectURL: getEnv(
			"AUTH_SUCCESS_REDIRECT_URL",
			"http://localhost:3000/auth/complete",
		),
		FailureRedirectURL: getEnv(
			"AUTH_FAILURE_REDIRECT_URL",
			"http://localhost:3000/auth/error",
		),
		OAuthTemporaryCookieTTL: oauthCookieTTL,
	}, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return duration, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
