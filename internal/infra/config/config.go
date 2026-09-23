package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port  string
	DBDSN string
}

func Load() *Config {
	// Load .env for local development. Existing environment variables take
	// precedence over values from the file.
	_ = godotenv.Load()

	return &Config{
		Port:  getEnv("PORT", "8080"),
		DBDSN: buildDBDSN(),
	}
}

// buildDBDSN uses the same database variables as docker-compose.yml so the API
// and PostgreSQL container connect to the same Bint database.
func buildDBDSN() string {
	host := getEnv("DATABASE_HOST", "localhost")
	port := getEnv("DATABASE_PORT", "5432")
	user := getEnv("DATABASE_USERNAME", "postgres")
	password := getEnv("DATABASE_PASSWORD", "postgres")
	name := getEnv("DATABASE_NAME", "bint") + os.Getenv("APP_ENV")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, name)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
