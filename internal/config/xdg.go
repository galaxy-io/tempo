package config

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the configuration directory path following XDG spec.
// Priority: $XDG_CONFIG_HOME/temporal-tui > ~/.config/temporal-tui > ~/.temporal-tui
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "temporal-tui")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ".temporal-tui"
	}

	// Prefer ~/.config/temporal-tui if .config exists
	configDir := filepath.Join(home, ".config")
	if info, err := os.Stat(configDir); err == nil && info.IsDir() {
		return filepath.Join(configDir, "temporal-tui")
	}

	return filepath.Join(home, ".temporal-tui")
}

// ConfigPath returns the full path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// ThemesDir returns the directory for custom themes.
func ThemesDir() string {
	return filepath.Join(ConfigDir(), "themes")
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() error {
	dir := ConfigDir()
	return os.MkdirAll(dir, 0755)
}

// EnsureThemesDir creates the themes directory if it doesn't exist.
func EnsureThemesDir() error {
	dir := ThemesDir()
	return os.MkdirAll(dir, 0755)
}
