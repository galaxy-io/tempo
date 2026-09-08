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

func TestLoadTemporalProfileTOMLImportsCurrentConnectionSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "temporal.toml")
	contents := []byte(`[profile.production]
address = "production.example.com:7233"
namespace = "production"
api_key = "secret"
authority = "temporal.internal"
grpc_meta = { x_team = "payments", Trace_ID = "1234" }

[profile.production.tls]
disabled = false
client_cert_data = "certificate data"
client_key_data = "key data"
server_ca_cert_path = "/tmp/temporal-ca.pem"
server_name = "temporal.example.com"
disable_host_verification = true
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write Temporal CLI profile: %v", err)
	}

	profiles, err := loadTemporalProfileTOML(path)
	if err != nil {
		t.Fatalf("load Temporal CLI profile: %v", err)
	}

	profile := profiles["production"]
	if !profile.TLS.Enabled {
		t.Fatal("TLS table presence was not imported")
	}
	if got, want := profile.TLS.CertData, "certificate data"; got != want {
		t.Fatalf("TLS cert data = %q, want %q", got, want)
	}
	if got, want := profile.TLS.KeyData, "key data"; got != want {
		t.Fatalf("TLS key data = %q, want %q", got, want)
	}
	if got, want := profile.TLS.CA, "/tmp/temporal-ca.pem"; got != want {
		t.Fatalf("TLS CA path = %q, want %q", got, want)
	}
	if !profile.TLS.SkipVerify {
		t.Fatal("TLS host verification setting was not imported")
	}
	if got, want := profile.Authority, "temporal.internal"; got != want {
		t.Fatalf("authority = %q, want %q", got, want)
	}
	if got, want := profile.GRPCMeta["x-team"], "payments"; got != want {
		t.Fatalf("normalized gRPC metadata = %q, want %q", got, want)
	}
	if got, want := profile.GRPCMeta["trace-id"], "1234"; got != want {
		t.Fatalf("normalized gRPC metadata = %q, want %q", got, want)
	}
}

func TestLoadTemporalProfileTOMLPreservesExplicitBaseTLS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "temporal.toml")
	contents := []byte(`[profile.secure]
address = "secure.example.com:7233"

[profile.secure.tls]
disabled = false
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write Temporal CLI profile: %v", err)
	}

	profiles, err := loadTemporalProfileTOML(path)
	if err != nil {
		t.Fatalf("load Temporal CLI profile: %v", err)
	}

	if !profiles["secure"].TLS.Enabled {
		t.Fatal("explicit TLS table without certificate settings was not preserved")
	}
}

func TestLoadTemporalProfileTOMLImportsInlineServerCAAndTLSDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "temporal.toml")
	contents := []byte(`[profile.local]
address = "localhost:7233"

[profile.local.tls]
disabled = true
server_ca_cert_data = "CA data"
`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write Temporal CLI profile: %v", err)
	}

	profiles, err := loadTemporalProfileTOML(path)
	if err != nil {
		t.Fatalf("load Temporal CLI profile: %v", err)
	}

	profile := profiles["local"]
	if !profile.TLS.Disabled {
		t.Fatal("explicit TLS disablement was not imported")
	}
	if got, want := profile.TLS.CAData, "CA data"; got != want {
		t.Fatalf("TLS CA data = %q, want %q", got, want)
	}
}

func TestLoadTemporalCLIProfilesUsesConfigFileOverride(t *testing.T) {
	// A direct config-file override must not depend on resolving a home
	// directory, which is only needed for the legacy YAML environment file.
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
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
