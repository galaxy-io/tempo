package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemporalEnvYAMLImportsCodecSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "temporal.yaml")
	contents := []byte(`env:
  production:
    address: production.example.com:7233
    namespace: production
    codec-endpoint: https://codec.example.com/{namespace}
    codec-auth: Bearer secret-token
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write Temporal CLI environment: %v", err)
	}

	profiles, err := loadTemporalEnvYAML(path)
	if err != nil {
		t.Fatalf("load Temporal CLI environment: %v", err)
	}

	profile := profiles["production"]
	if got, want := profile.Codec.Endpoint, "https://codec.example.com/{namespace}"; got != want {
		t.Fatalf("codec endpoint = %q, want %q", got, want)
	}
	if got, want := profile.Codec.Auth, "Bearer secret-token"; got != want {
		t.Fatalf("codec auth = %q, want %q", got, want)
	}
}

func TestLoadTemporalProfileTOMLImportsCodecSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "temporal.toml")
	contents := []byte(`[profile.production]
address = "production.example.com:7233"
namespace = "production"

[profile.production.codec]
endpoint = "https://codec.example.com/{namespace}"
auth = "Bearer secret-token"
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write Temporal CLI profile: %v", err)
	}

	profiles, err := loadTemporalProfileTOML(path)
	if err != nil {
		t.Fatalf("load Temporal CLI profile: %v", err)
	}

	profile := profiles["production"]
	if got, want := profile.Codec.Endpoint, "https://codec.example.com/{namespace}"; got != want {
		t.Fatalf("codec endpoint = %q, want %q", got, want)
	}
	if got, want := profile.Codec.Auth, "Bearer secret-token"; got != want {
		t.Fatalf("codec auth = %q, want %q", got, want)
	}
}
