package main

import (
	"testing"

	"github.com/galaxy-io/tempo/internal/temporal"
)

func TestApplyCodecOverridesReplacesProfileSettings(t *testing.T) {
	connection := temporal.ConnectionConfig{
		CodecEndpoint: "https://profile-codec.example.com",
		CodecAuth:     "profile auth",
	}

	applyCodecOverrides(
		&connection,
		"https://flag-codec.example.com",
		"flag auth",
		map[string]string{"X-Codec-Tenant": "payments"},
	)

	if got, want := connection.CodecEndpoint, "https://flag-codec.example.com"; got != want {
		t.Fatalf("codec endpoint = %q, want %q", got, want)
	}
	if got, want := connection.CodecAuth, "flag auth"; got != want {
		t.Fatalf("codec auth = %q, want %q", got, want)
	}
	if got, want := connection.CodecHeaders["X-Codec-Tenant"], "payments"; got != want {
		t.Fatalf("codec header = %q, want %q", got, want)
	}
}

func TestCodecHeaderFlags(t *testing.T) {
	var headers codecHeaderFlags
	if err := headers.Set("X-Codec-Tenant=payments=west"); err != nil {
		t.Fatalf("set codec header: %v", err)
	}
	if got, want := headers["X-Codec-Tenant"], "payments=west"; got != want {
		t.Fatalf("codec header = %q, want %q", got, want)
	}
	if err := headers.Set("invalid"); err == nil {
		t.Fatal("invalid codec header should fail")
	}
}
