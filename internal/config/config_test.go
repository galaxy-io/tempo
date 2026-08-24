package config

import "testing"

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
