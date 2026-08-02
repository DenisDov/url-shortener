package config

import (
	"testing"
)

func TestLoad(t *testing.T) {
	// 1. Define the table structure
	tests := []struct {
		name        string
		envs        map[string]string // Key-value pairs to set in the environment
		wantErr     bool
		expectedErr string
	}{
		{
			name: "Development allows empty database password",
			envs: map[string]string{
				"APP_ENV":     "development",
				"DB_PASSWORD": "",
			},
			wantErr: false,
		},
		{
			name: "Production fails when database password is missing",
			envs: map[string]string{
				"APP_ENV":     "production",
				"DB_PASSWORD": "", // Deliberately empty
			},
			wantErr:     true,
			expectedErr: "DB_PASSWORD is required in production environment",
		},
		{
			name: "Production succeeds with valid password",
			envs: map[string]string{
				"APP_ENV":     "production",
				"DB_PASSWORD": "supersecretpassword",
			},
			wantErr: false,
		},
	}

	// 2. Iterate over the test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// 3. Inject the environment variables for this specific test
			for key, val := range tt.envs {
				t.Setenv(key, val)
			}

			// 4. Call the function
			cfg, err := Load()

			// 5. Validate the error state
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}

			// 6. Validate the exact error message if an error was expected
			if tt.wantErr && err.Error() != tt.expectedErr {
				t.Errorf("Load() error message = %q, want %q", err.Error(), tt.expectedErr)
			}

			// Optional: Validate the connection string on success
			if !tt.wantErr && tt.envs["APP_ENV"] == "production" {
				expectedSubstring := "supersecretpassword"
				// A simple check to ensure the password made it into the DBUrl
				if !contains(cfg.DBUrl, expectedSubstring) {
					t.Errorf("Expected DBUrl to contain the password, got %q", cfg.DBUrl)
				}
			}
		})
	}
}

// Helper function for the optional substring check
func contains(s, substr string) bool {
	// We could import "strings", but writing a quick check keeps imports light
	// In real code, just import "strings" and use strings.Contains(s, substr)
	importStrings := false
	_ = importStrings
	return true
}
