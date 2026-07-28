package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	GinMode     string
	DatabaseURL string
}

func Load() (*Config, error) {
	// Attempt to load .env file if present, ignore error if missing (e.g. in container envs)
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	ginMode := getEnv("GIN_MODE", "debug")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		host := getEnv("DB_HOST", "localhost")
		dbPort := getEnv("DB_PORT", "5432")
		user := getEnv("DB_USER", "postgres")
		pass := getEnv("DB_PASSWORD", "postgres")
		name := getEnv("DB_NAME", "restock_priority")
		ssl := getEnv("DB_SSLMODE", "disable")

		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, dbPort, name, ssl)
	}

	return &Config{
		Port:        port,
		GinMode:     ginMode,
		DatabaseURL: dbURL,
	}, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
