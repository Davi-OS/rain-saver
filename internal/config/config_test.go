package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	cfg := Load()

	if cfg.Port != "8081" {
		t.Errorf("Port = %q, want 8081", cfg.Port)
	}
	if cfg.PostgresHost != "localhost" {
		t.Errorf("PostgresHost = %q, want localhost", cfg.PostgresHost)
	}
	if cfg.PostgresPort != "5433" {
		t.Errorf("PostgresPort = %q, want 5433", cfg.PostgresPort)
	}
	if cfg.PostgresUser != "rainsaver" {
		t.Errorf("PostgresUser = %q, want rainsaver", cfg.PostgresUser)
	}
	if cfg.PostgresPassword != "rainsaver" {
		t.Errorf("PostgresPassword = %q, want rainsaver", cfg.PostgresPassword)
	}
	if cfg.PostgresDB != "rainsaver" {
		t.Errorf("PostgresDB = %q, want rainsaver", cfg.PostgresDB)
	}
	if cfg.AuthServiceBaseURL != "http://localhost:8080" {
		t.Errorf("AuthServiceBaseURL = %q, want http://localhost:8080", cfg.AuthServiceBaseURL)
	}
	if cfg.AuthIssuer != "" {
		t.Errorf("AuthIssuer = %q, want empty (SDK default)", cfg.AuthIssuer)
	}
}

func TestLoadEnvOverridesDefaults(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("POSTGRES_HOST", "db.internal")
	t.Setenv("AUTH_SERVICE_BASE_URL", "https://auth.internal")
	t.Setenv("AUTH_ISSUER", "https://auth.internal")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
	if cfg.PostgresHost != "db.internal" {
		t.Errorf("PostgresHost = %q, want db.internal", cfg.PostgresHost)
	}
	if cfg.AuthServiceBaseURL != "https://auth.internal" {
		t.Errorf("AuthServiceBaseURL = %q, want https://auth.internal", cfg.AuthServiceBaseURL)
	}
	if cfg.AuthIssuer != "https://auth.internal" {
		t.Errorf("AuthIssuer = %q, want https://auth.internal", cfg.AuthIssuer)
	}
}

func TestDSNPrefersDatabaseURL(t *testing.T) {
	cfg := &Config{DatabaseURL: "postgres://custom-dsn"}
	if got := cfg.DSN(); got != "postgres://custom-dsn" {
		t.Errorf("DSN() = %q, want postgres://custom-dsn", got)
	}
}

func TestDSNBuildsFromFields(t *testing.T) {
	cfg := &Config{
		PostgresUser:     "u",
		PostgresPassword: "p",
		PostgresHost:     "h",
		PostgresPort:     "5433",
		PostgresDB:       "d",
	}
	want := "postgres://u:p@h:5433/d?sslmode=disable"
	if got := cfg.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

func TestAddr(t *testing.T) {
	cfg := &Config{Port: "8081"}
	if got := cfg.Addr(); got != ":8081" {
		t.Errorf("Addr() = %q, want :8081", got)
	}
}

func TestCORSAllowedOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", " http://localhost:5173 , ,https://davi-os.github.io")
	got := Load().CORSAllowedOrigins
	if len(got) != 2 || got[0] != "http://localhost:5173" || got[1] != "https://davi-os.github.io" {
		t.Errorf("CORSAllowedOrigins = %q", got)
	}

	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	if got := Load().CORSAllowedOrigins; len(got) != 0 {
		t.Errorf("empty env: CORSAllowedOrigins = %q, want none", got)
	}
}
