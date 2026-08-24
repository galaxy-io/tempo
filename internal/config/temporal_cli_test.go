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

func TestLoadTemporalCLIProfilesUsesConfigFileOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(t.TempDir(), "custom-temporal.toml")
	t.Setenv("TEMPORAL_CONFIG_FILE", path)
	contents := []byte(`[profile.production]
address = "production.example.com:7233"
namespace = "production"
codec = { endpoint = "https://codec.example.com", auth = "Bearer secret-token" }
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write Temporal CLI profile: %v", err)
	}

	profiles := LoadTemporalCLIProfiles()
	profile, ok := profiles["production"]
	if !ok {
		t.Fatal("production profile was not imported from TEMPORAL_CONFIG_FILE")
	}
	if got, want := profile.Codec.Endpoint, "https://codec.example.com"; got != want {
		t.Fatalf("codec endpoint = %q, want %q", got, want)
	}
}

func TestLoadTemporalCLIProfilesUsesPlatformConfigDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	t.Setenv("APPDATA", filepath.Join(home, "app-data"))
	t.Setenv("TEMPORAL_CONFIG_FILE", "")

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("resolve platform config directory: %v", err)
	}
	path := filepath.Join(configDir, "temporalio", "temporal.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create Temporal CLI config directory: %v", err)
	}
	contents := []byte(`[profile.platform]
address = "platform.example.com:7233"
namespace = "platform"
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write Temporal CLI profile: %v", err)
	}

	profiles := LoadTemporalCLIProfiles()
	if _, ok := profiles["platform"]; !ok {
		t.Fatalf("platform profile was not imported from %s", path)
	}
}
