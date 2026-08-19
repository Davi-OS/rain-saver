package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port string

	DatabaseURL      string
	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	AuthServiceBaseURL string
}

// Load reads configuration from the environment once. Call it a single
// time in main() and pass the result explicitly to whatever needs it — no
// package-level global, so callers stay testable and dependencies stay
// explicit.
func Load() *Config {
	_ = godotenv.Load() // fine if .env doesn't exist — real env vars take precedence in production

	return &Config{
		Port: envOrDefault("PORT", "8081"),

		DatabaseURL:      os.Getenv("DATABASE_URL"),
		PostgresHost:     envOrDefault("POSTGRES_HOST", "localhost"),
		PostgresPort:     envOrDefault("POSTGRES_PORT", "5433"),
		PostgresUser:     envOrDefault("POSTGRES_USER", "rainsaver"),
		PostgresPassword: envOrDefault("POSTGRES_PASSWORD", "rainsaver"),
		PostgresDB:       envOrDefault("POSTGRES_DB", "rainsaver"),

		AuthServiceBaseURL: envOrDefault("AUTH_SERVICE_BASE_URL", "http://localhost:8080"),
	}
}

// DSN builds the Postgres connection string, preferring DatabaseURL
// verbatim when set.
func (c *Config) DSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.PostgresUser, c.PostgresPassword, c.PostgresHost, c.PostgresPort, c.PostgresDB)
}

// Addr is the address net/http.Server should listen on.
func (c *Config) Addr() string {
	return ":" + c.Port
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
