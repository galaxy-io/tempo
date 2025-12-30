package update

// Version information - injected at build time via ldflags:
// go build -ldflags "-X github.com/atterpac/tempo/internal/update.Version=1.2.3 -X github.com/atterpac/tempo/internal/update.Commit=abc123 -X github.com/atterpac/tempo/internal/update.BuildDate=2024-01-01"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
