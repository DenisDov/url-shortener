package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the final exported configuration used by your application.
type Config struct {
	AppPort string
	AppEnv  string
	DBUrl   string
}

// rawEnv is an internal struct to handle parsing and default values via tags.
type rawEnv struct {
	AppPort    string `env:"APP_PORT" envDefault:"8080"`
	AppEnv     string `env:"APP_ENV" envDefault:"development"`
	DBHost     string `env:"DB_HOST" envDefault:"localhost"`
	DBPort     string `env:"DB_PORT" envDefault:"5432"`
	DBUser     string `env:"DB_USER" envDefault:"postgres"`
	DBPassword string `env:"DB_PASSWORD"`
	DBName     string `env:"DB_NAME" envDefault:"postgres"`
	DBSSLMode  string `env:"DB_SSLMODE" envDefault:"disable"`
}

// Load reads environment variables and populates the Config struct.
func Load() (Config, error) {
	// 1. Load .env file if it exists.
	// We ignore the error because in production, variables are usually injected
	// by the environment (e.g., Docker/K8s), so a .env file might not exist.
	_ = godotenv.Load()

	// 2. Parse environment variables into our raw struct using generics
	parsedEnv, err := env.ParseAs[rawEnv]()
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// 3. Validate critical variables
	if parsedEnv.AppEnv == "production" && parsedEnv.DBPassword == "" {
		return Config{}, fmt.Errorf("DB_PASSWORD is required in production environment")
	}

	// 4. Construct the DB string
	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		parsedEnv.DBUser, parsedEnv.DBPassword, parsedEnv.DBHost, parsedEnv.DBPort, parsedEnv.DBName, parsedEnv.DBSSLMode,
	)

	// 5. Create and return the final struct
	cfg := Config{
		AppPort: parsedEnv.AppPort,
		AppEnv:  parsedEnv.AppEnv,
		DBUrl:   dbUrl,
	}

	return cfg, nil
}
