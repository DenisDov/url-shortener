package config

import (
	"fmt"
	"os"
)

type Config struct {
	AppPort string
	AppEnv  string
	DBUrl   string
}

// Load reads environment variables and populates the Config struct.
func Load() (Config, error) {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "postgres")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")

	// 1. Validate critical variables before building the connection string
	// (Example: requiring a password in production)
	appEnv := getEnv("APP_ENV", "development")
	if appEnv == "production" && dbPass == "" {
		return Config{}, fmt.Errorf("DB_PASSWORD is required in production environment")
	}

	// 2. Construct the DB string
	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPass, dbHost, dbPort, dbName, dbSSLMode,
	)

	// 3. Create and return the struct by value
	cfg := Config{
		AppPort: getEnv("APP_PORT", "8080"),
		AppEnv:  appEnv,
		DBUrl:   dbUrl,
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
