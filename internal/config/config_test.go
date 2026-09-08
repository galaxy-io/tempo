package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConnectionConfigExpandEnvExpandsCodecCredentials(t *testing.T) {
	t.Setenv("TEMPO_CODEC_ENDPOINT", "https://codec.example.com")
	t.Setenv("TEMPO_CODEC_AUTH", "Bearer secret-token")

	expanded := (ConnectionConfig{
		Codec: CodecConfig{
			Endpoint: "$TEMPO_CODEC_ENDPOINT/{namespace}",
			Auth:     "$TEMPO_CODEC_AUTH",
		},
	}).ExpandEnv()

	if got, want := expanded.Codec.Endpoint, "https://codec.example.com/{namespace}"; got != want {
		t.Fatalf("codec endpoint = %q, want %q", got, want)
	}
	if got, want := expanded.Codec.Auth, "Bearer secret-token"; got != want {
		t.Fatalf("codec auth = %q, want %q", got, want)
	}
}

func TestLoadPreservesImportedActiveProfile(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, "config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	configPath := filepath.Join(configHome, "tempo", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("create Tempo config directory: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`active_profile: import:production
profiles:
  default:
    address: localhost:7233
    namespace: default
`), 0o600); err != nil {
		t.Fatalf("write Tempo config: %v", err)
	}

	temporalConfigPath := filepath.Join(home, "temporal.toml")
	t.Setenv("TEMPORAL_CONFIG_FILE", temporalConfigPath)
	if err := os.WriteFile(temporalConfigPath, []byte(`[profile.production]
address = "production.example.com:7233"
namespace = "production"
`), 0o600); err != nil {
		t.Fatalf("write Temporal config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got, want := cfg.ActiveProfile, "import:production"; got != want {
		t.Fatalf("active profile = %q, want %q", got, want)
	}
	if _, ok := cfg.GetProfile(cfg.ActiveProfile); !ok {
		t.Fatalf("active imported profile %q was not loaded", cfg.ActiveProfile)
	}
}
