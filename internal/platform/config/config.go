package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"

	"github.com/joho/godotenv"
)

// ginModes are the only values gin.SetMode accepts. They are restated here rather
// than imported so that configuration stays independent of the web framework, and
// they are checked at all because gin panics on anything else: without this, a typo
// like GIN_MODE=production takes the process down with a stack trace instead of a
// configuration error.
var ginModes = []string{"debug", "release", "test"}

type Config struct {
	Port        string
	GinMode     string
	DatabaseURL string
}

func Load() (*Config, error) {
	// Attempt to load .env file if present, ignore error if missing (e.g. in container envs)
	_ = godotenv.Load()

	ginMode := getEnv("GIN_MODE", "debug")
	if !slices.Contains(ginModes, ginMode) {
		return nil, fmt.Errorf("GIN_MODE must be one of %v, got %q", ginModes, ginMode)
	}

	return &Config{
		Port:        getEnv("PORT", "8080"),
		GinMode:     ginMode,
		DatabaseURL: databaseURL(),
	}, nil
}

// databaseURL prefers an explicit DATABASE_URL and otherwise assembles one from the
// discrete DB_* variables.
//
// The fallback is built with net/url rather than fmt.Sprintf because a credential is
// not a format argument: a password containing @, /, ? or # would corrupt the URL, and
// an @ in particular would silently point the service at a different host. JoinHostPort
// brackets IPv6 literals for the same reason.
func databaseURL() string {
	if explicit := os.Getenv("DATABASE_URL"); explicit != "" {
		return explicit
	}

	dsn := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			getEnv("DB_USER", "postgres"),
			getEnv("DB_PASSWORD", "postgres"),
		),
		Host: net.JoinHostPort(
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_PORT", "5432"),
		),
		Path:     "/" + getEnv("DB_NAME", "restock_priority"),
		RawQuery: url.Values{"sslmode": {getEnv("DB_SSLMODE", "disable")}}.Encode(),
	}

	return dsn.String()
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
