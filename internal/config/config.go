package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the final exported configuration used by your application.
//
// DATABASE_URL and REDIS_ADDR arrive fully formed rather than being assembled
// from host/port/user/password parts: docker compose builds the DSN from the
// POSTGRES_* variables, and outside compose the Makefile passes DB_DSN through.
// One value, one source, no chance of the two drifting apart.
type Config struct {
	AppEnv      string        `env:"APP_ENV"       envDefault:"development"`
	HTTPPort    string        `env:"HTTP_PORT"     envDefault:"8080"`
	DatabaseURL string        `env:"DATABASE_URL"`
	RedisAddr   string        `env:"REDIS_ADDR"    envDefault:"localhost:6379"`
	RedisDB     int           `env:"REDIS_DB"      envDefault:"0"`
	BaseURL     string        `env:"BASE_URL"      envDefault:"http://localhost:8080"`
	CodeLength  int           `env:"CODE_LENGTH"   envDefault:"7"`
	CacheTTL    time.Duration `env:"CACHE_TTL"     envDefault:"1h"`

	ReadTimeout  time.Duration `env:"READ_TIMEOUT"  envDefault:"5s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s"`
}

// Load reads environment variables and populates the Config struct.
func Load() (Config, error) {
	// 1. Load .env file if it exists.
	// We ignore the error because in production, variables are usually injected
	// by the environment (e.g., Docker/K8s), so a .env file might not exist.
	_ = godotenv.Load()

	// 2. Parse environment variables into the config struct using generics
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// 3. Validate critical variables
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.CodeLength <= 0 {
		return fmt.Errorf("CODE_LENGTH must be positive, got %d", c.CodeLength)
	}
	return nil
}
