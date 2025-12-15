package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TLSConfig holds TLS connection settings.
type TLSConfig struct {
	Cert       string `yaml:"cert,omitempty"`
	Key        string `yaml:"key,omitempty"`
	CA         string `yaml:"ca,omitempty"`
	ServerName string `yaml:"server_name,omitempty"`
	SkipVerify bool   `yaml:"skip_verify,omitempty"`
}

// ConnectionConfig holds Temporal connection settings.
type ConnectionConfig struct {
	Address   string    `yaml:"address"`
	Namespace string    `yaml:"namespace"`
	TLS       TLSConfig `yaml:"tls,omitempty"`
}

// Config represents the application configuration.
type Config struct {
	Theme      string           `yaml:"theme"`
	Connection ConnectionConfig `yaml:"connection"`
}

// DefaultConfig returns a config with default values.
func DefaultConfig() *Config {
	return &Config{
		Theme: DefaultTheme,
		Connection: ConnectionConfig{
			Address:   "localhost:7233",
			Namespace: "default",
		},
	}
}

// Load reads the config file from disk.
// Returns default config if file doesn't exist.
func Load() (*Config, error) {
	path := ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// Save writes the config to disk.
func (c *Config) Save() error {
	if err := EnsureConfigDir(); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := ConfigPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// LoadTheme loads a theme by name or path.
// If name matches a built-in theme, returns that.
// Otherwise, attempts to load from custom themes directory or absolute path.
func LoadTheme(name string) (*ParsedTheme, error) {
	// Check built-in themes first
	if theme, ok := BuiltinThemes[name]; ok {
		parsed, err := theme.Parse()
		if err != nil {
			return nil, err
		}
		parsed.Key = name
		return parsed, nil
	}

	// Check custom themes directory
	customPath := filepath.Join(ThemesDir(), name+".yaml")
	if _, err := os.Stat(customPath); err == nil {
		parsed, err := loadThemeFile(customPath)
		if err != nil {
			return nil, err
		}
		parsed.Key = name
		return parsed, nil
	}

	// Try as absolute/relative path
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
		if _, err := os.Stat(name); err == nil {
			parsed, err := loadThemeFile(name)
			if err != nil {
				return nil, err
			}
			parsed.Key = name
			return parsed, nil
		}
	}

	return nil, fmt.Errorf("theme not found: %s", name)
}

// Save writes the config to disk (standalone function).
func Save(c *Config) error {
	return c.Save()
}

// loadThemeFile loads a theme from a YAML file.
func loadThemeFile(path string) (*ParsedTheme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading theme file: %w", err)
	}

	var theme Theme
	if err := yaml.Unmarshal(data, &theme); err != nil {
		return nil, fmt.Errorf("parsing theme file: %w", err)
	}

	return theme.Parse()
}

// ValidateTheme checks if a theme name is valid.
func ValidateTheme(name string) bool {
	// Built-in theme
	if _, ok := BuiltinThemes[name]; ok {
		return true
	}

	// Custom theme file
	customPath := filepath.Join(ThemesDir(), name+".yaml")
	if _, err := os.Stat(customPath); err == nil {
		return true
	}

	// Absolute path
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
		if _, err := os.Stat(name); err == nil {
			return true
		}
	}

	return false
}
