package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIntegration1COptionalAuth(t *testing.T) {
	tests := []struct {
		name        string
		integration string
		wantAuth    *Integration1CAuthConfig
	}{
		{
			name:        "omitted",
			integration: `{"url":"http://127.0.0.1:3001"}`,
		},
		{
			name:        "null",
			integration: `{"url":"http://127.0.0.1:3001","auth":null}`,
		},
		{
			name:        "configured",
			integration: `{"url":"http://127.0.0.1:3001","auth":{"user_name":"gateway-user","password":"secret:пароль"}}`,
			wantAuth: &Integration1CAuthConfig{
				UserName: "gateway-user",
				Password: "secret:пароль",
			},
		},
		{
			name:        "empty password",
			integration: `{"url":"http://127.0.0.1:3001","auth":{"user_name":"gateway-user","password":""}}`,
			wantAuth:    &Integration1CAuthConfig{UserName: "gateway-user"},
		},
		{
			name:        "empty object",
			integration: `{"url":"http://127.0.0.1:3001","auth":{}}`,
			wantAuth:    &Integration1CAuthConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			body := `{"database":{"primary":"postgres://localhost/meatshop"},"integration_1c":` + tt.integration + `}`
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load(): %v", err)
			}
			got := cfg.Integration1C.Auth
			if tt.wantAuth == nil {
				if got != nil {
					t.Fatal("expected authentication to be disabled")
				}
				return
			}
			if got == nil {
				t.Fatal("expected authentication to be configured")
			}
			if *got != *tt.wantAuth {
				t.Fatal("loaded credentials differ from configured credentials")
			}
		})
	}
}
