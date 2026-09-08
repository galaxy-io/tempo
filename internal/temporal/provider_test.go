package temporal

import (
	"testing"

	"github.com/galaxy-io/tempo/internal/config"
)

func TestConnectionConfigFromProfileIncludesCodecSettings(t *testing.T) {
	got := ConnectionConfigFromProfile(config.ConnectionConfig{
		Address:   "production.example.com:7233",
		Namespace: "production",
		TLS: config.TLSConfig{
			Enabled:    true,
			Disabled:   true,
			CertData:   "certificate data",
			KeyData:    "key data",
			CAData:     "CA data",
			SkipVerify: true,
		},
		Codec: config.CodecConfig{
			Endpoint: "https://codec.example.com/{namespace}",
			Auth:     "Bearer secret-token",
			Headers:  map[string]string{"X-Codec-Tenant": "payments"},
		},
		Authority: "temporal.internal",
	})

	if want := "https://codec.example.com/{namespace}"; got.CodecEndpoint != want {
		t.Fatalf("codec endpoint = %q, want %q", got.CodecEndpoint, want)
	}
	if want := "Bearer secret-token"; got.CodecAuth != want {
		t.Fatalf("codec auth = %q, want %q", got.CodecAuth, want)
	}
	if !got.TLSEnabled || !got.TLSDisabled || !got.TLSSkipVerify {
		t.Fatalf("TLS booleans were not translated: %#v", got)
	}
	if got.TLSCertData != "certificate data" || got.TLSKeyData != "key data" || got.TLSCAData != "CA data" {
		t.Fatalf("inline TLS data was not translated: %#v", got)
	}
	if want := "temporal.internal"; got.Authority != want {
		t.Fatalf("authority = %q, want %q", got.Authority, want)
	}
	if want := "payments"; got.CodecHeaders["X-Codec-Tenant"] != want {
		t.Fatalf("codec header = %q, want %q", got.CodecHeaders["X-Codec-Tenant"], want)
	}
}

func TestBuildClientOptionsEnablesBaseTLS(t *testing.T) {
	opts, err := buildClientOptions(ConnectionConfig{
		Address:    "production.example.com:7233",
		Namespace:  "production",
		TLSEnabled: true,
	})
	if err != nil {
		t.Fatalf("build client options: %v", err)
	}
	if opts.ConnectionOptions.TLS == nil {
		t.Fatal("explicit base TLS configuration was not enabled")
	}
}
