package config

import "time"

type Config struct {
	Application ApplicationConfig
	Server      ServerConfig
	Database    DatabaseConfig
	Auth        AuthConfig
}

type ApplicationConfig struct {
	Name        string
	Environment string
	LogLevel    string
}

type ServerConfig struct {
	Port              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type DatabaseConfig struct {
	URL      string
	Host     string
	Port     string
	Username string
	Password string
	Name     string
	DSN      string
}

type AuthConfig struct {
	JWT                     JWTConfig
	Google                  GoogleConfig
	Cookie                  CookieConfig
	SuccessRedirectURL      string
	FailureRedirectURL      string
	OAuthTemporaryCookieTTL time.Duration
}

type JWTConfig struct {
	Secret          string
	Issuer          string
	Audience        string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type CookieConfig struct {
	Domain string
	Secure bool
}
