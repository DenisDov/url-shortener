package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	const validDSN = "postgres://user:supersecretpassword@localhost:5432/urlshortener?sslmode=disable"

	tests := []struct {
		name        string
		envs        map[string]string // Key-value pairs to set in the environment
		wantErr     bool
		expectedErr string
	}{
		{
			name: "Succeeds with only DATABASE_URL set",
			envs: map[string]string{
				"DATABASE_URL": validDSN,
			},
			wantErr: false,
		},
		{
			name: "Fails when DATABASE_URL is missing",
			envs: map[string]string{
				"DATABASE_URL": "", // Deliberately empty
			},
			wantErr:     true,
			expectedErr: "DATABASE_URL is required",
		},
		{
			name: "Fails on a non-positive code length",
			envs: map[string]string{
				"DATABASE_URL": validDSN,
				"CODE_LENGTH":  "0",
			},
			wantErr:     true,
			expectedErr: "CODE_LENGTH must be positive, got 0",
		},
		{
			name: "Fails on an unparseable duration",
			envs: map[string]string{
				"DATABASE_URL": validDSN,
				"CACHE_TTL":    "not-a-duration",
			},
			wantErr:     true,
			expectedErr: "failed to parse environment variables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Inject the environment variables for this specific test
			for key, val := range tt.envs {
				t.Setenv(key, val)
			}

			cfg, err := Load()

			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("Load() error message = %q, want it to contain %q", err, tt.expectedErr)
				}
				return
			}

			if cfg.DatabaseURL != tt.envs["DATABASE_URL"] {
				t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, tt.envs["DATABASE_URL"])
			}
		})
	}
}

// Defaults matter here: docker compose only passes a handful of variables, so
// everything else the server needs to boot has to come from the envDefault tags.
func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	checks := []struct {
		field string
		got   any
		want  any
	}{
		{"AppEnv", cfg.AppEnv, "development"},
		{"HTTPPort", cfg.HTTPPort, "8080"},
		{"RedisAddr", cfg.RedisAddr, "localhost:6379"},
		{"RedisDB", cfg.RedisDB, 0},
		{"BaseURL", cfg.BaseURL, "http://localhost:8080"},
		{"CodeLength", cfg.CodeLength, 7},
		{"CacheTTL", cfg.CacheTTL, time.Hour},
		{"ReadTimeout", cfg.ReadTimeout, 5 * time.Second},
		{"WriteTimeout", cfg.WriteTimeout, 10 * time.Second},
		{"IdleTimeout", cfg.IdleTimeout, 120 * time.Second},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.field, c.got, c.want)
		}
	}
}
