package update

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/creativeprojects/go-selfupdate"
)

const (
	repoOwner = "atterpac"
	repoName  = "tempo"
)

// IsHomebrewInstall returns true if the binary was installed via Homebrew.
func IsHomebrewInstall() bool {
	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return false
	}
	// Homebrew installs to /opt/homebrew/Cellar or /usr/local/Cellar
	return strings.Contains(exe, "homebrew") || strings.Contains(exe, "Cellar")
}

// UpdateInfo contains information about an available update.
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseURL     string
	ReleaseNotes   string
	NeedsUpdate    bool
}

// Updater handles checking for and applying updates.
type Updater struct {
	source selfupdate.Source
	repo   selfupdate.RepositorySlug
}

// NewUpdater creates a new Updater instance.
func NewUpdater() *Updater {
	source, _ := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	return &Updater{
		source: source,
		repo:   selfupdate.ParseSlug(repoOwner + "/" + repoName),
	}
}

// CheckForUpdate checks if a newer version is available.
func (u *Updater) CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    u.source,
		Validator: nil, // Could add checksum validation here
	})
	if err != nil {
		return nil, fmt.Errorf("creating updater: %w", err)
	}

	latest, found, err := updater.DetectLatest(ctx, u.repo)
	if err != nil {
		return nil, fmt.Errorf("detecting latest version: %w", err)
	}

	if !found {
		return &UpdateInfo{
			CurrentVersion: Version,
			NeedsUpdate:    false,
		}, nil
	}

	info := &UpdateInfo{
		CurrentVersion: Version,
		LatestVersion:  latest.Version(),
		ReleaseURL:     latest.URL,
		ReleaseNotes:   latest.ReleaseNotes,
		NeedsUpdate:    false,
	}

	// Check if we need an update
	// For "dev" version, always consider it needs update if a release exists
	if Version == "dev" {
		info.NeedsUpdate = true
		return info, nil
	}

	// Compare versions
	if latest.GreaterThan(Version) {
		info.NeedsUpdate = true
	}

	return info, nil
}

// ApplyUpdate downloads and applies the update.
func (u *Updater) ApplyUpdate(ctx context.Context, info *UpdateInfo) error {
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    u.source,
		Validator: nil,
	})
	if err != nil {
		return fmt.Errorf("creating updater: %w", err)
	}

	latest, found, err := updater.DetectLatest(ctx, u.repo)
	if err != nil {
		return fmt.Errorf("detecting latest version: %w", err)
	}

	if !found {
		return fmt.Errorf("no release found")
	}

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	if err := updater.UpdateTo(ctx, latest, exe); err != nil {
		return fmt.Errorf("updating binary: %w", err)
	}

	return nil
}

// GetCurrentVersion returns the current version string.
func GetCurrentVersion() string {
	return Version
}

// GetVersionInfo returns formatted version information.
func GetVersionInfo() string {
	return fmt.Sprintf("tempo %s (%s/%s)\nCommit: %s\nBuilt: %s",
		Version, runtime.GOOS, runtime.GOARCH, Commit, BuildDate)
}
