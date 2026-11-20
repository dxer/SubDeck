package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAndEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
server:
  host: "127.0.0.1"
  port: 9090
  trusted_proxies:
    - "127.0.0.1"
database:
  path: "/tmp/subdeck.db"
auth:
  admin_username: "opsadmin"
  admin_password: "file-password"
  jwt_secret: "file-secret"
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	t.Setenv("SUBDECK_PORT", "18080")
	t.Setenv("SUBDECK_ADMIN_USER", "envadmin")

	cfg, err := Load(cfgPath, true)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("expected host 127.0.0.1, got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 18080 {
		t.Fatalf("expected env override port 18080, got %d", cfg.Server.Port)
	}
	if cfg.Database.Path != "/tmp/subdeck.db" {
		t.Fatalf("expected db path /tmp/subdeck.db, got %q", cfg.Database.Path)
	}
	if cfg.Auth.AdminUsername != "envadmin" {
		t.Fatalf("expected env override user envadmin, got %q", cfg.Auth.AdminUsername)
	}
	if cfg.Auth.AdminPassword != "file-password" || cfg.Auth.JWTSecret != "file-secret" {
		t.Fatalf("unexpected auth config: %+v", cfg.Auth)
	}
}

func TestLoadExplicitMissingFileErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml"), true); err == nil {
		t.Fatalf("expected error for explicit missing config file")
	}

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"), false)
	if err != nil {
		t.Fatalf("expected defaults when implicit config file is absent, got %v", err)
	}
	if cfg.Server.Port != 8080 || cfg.Auth.AdminUsername != "admin" {
		t.Fatalf("unexpected default config: %+v", cfg)
	}
}
