package temporal

import (
	"testing"

	"github.com/galaxy-io/tempo/internal/config"
)

func TestConnectionConfigFromProfileIncludesCodecSettings(t *testing.T) {
	got := ConnectionConfigFromProfile(config.ConnectionConfig{
		Address:   "production.example.com:7233",
		Namespace: "production",
		Codec: config.CodecConfig{
			Endpoint: "https://codec.example.com/{namespace}",
			Auth:     "Bearer secret-token",
			Headers:  map[string]string{"X-Codec-Tenant": "payments"},
		},
	})

	if want := "https://codec.example.com/{namespace}"; got.CodecEndpoint != want {
		t.Fatalf("codec endpoint = %q, want %q", got.CodecEndpoint, want)
	}
	if want := "Bearer secret-token"; got.CodecAuth != want {
		t.Fatalf("codec auth = %q, want %q", got.CodecAuth, want)
	}
	if want := "payments"; got.CodecHeaders["X-Codec-Tenant"] != want {
		t.Fatalf("codec header = %q, want %q", got.CodecHeaders["X-Codec-Tenant"], want)
	}
}
