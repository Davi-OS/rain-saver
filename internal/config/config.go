package config

import (
	"fmt"
	"os"
	"strings"

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
	// AuthIssuer must equal the auth service's AUTH_ISSUER (the "iss" of
	// its tokens). Empty uses the SDK default, "auth-service".
	AuthIssuer string

	// GeocoderURL is the Nominatim instance used to locate addresses sent
	// without coordinates; GeocoderUserAgent identifies this app to it, as
	// Nominatim's usage policy requires.
	GeocoderURL       string
	GeocoderUserAgent string

	// CORSAllowedOrigins are the browser origins (e.g. http://localhost:5173)
	// allowed to call the API. Empty disables CORS.
	CORSAllowedOrigins []string
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
		AuthIssuer:         os.Getenv("AUTH_ISSUER"),

		GeocoderURL:       envOrDefault("GEOCODER_URL", "https://nominatim.openstreetmap.org"),
		GeocoderUserAgent: envOrDefault("GEOCODER_USER_AGENT", "rain-saver (+https://github.com/Davi-OS/rain-saver)"),

		CORSAllowedOrigins: envList("CORS_ALLOWED_ORIGINS"),
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

// envList parses a comma-separated env var, dropping blanks.
func envList(key string) []string {
	var out []string
	for _, v := range strings.Split(os.Getenv(key), ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
