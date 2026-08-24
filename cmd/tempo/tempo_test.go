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
	)

	if got, want := connection.CodecEndpoint, "https://flag-codec.example.com"; got != want {
		t.Fatalf("codec endpoint = %q, want %q", got, want)
	}
	if got, want := connection.CodecAuth, "flag auth"; got != want {
		t.Fatalf("codec auth = %q, want %q", got, want)
	}
}
