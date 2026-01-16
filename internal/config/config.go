package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	APIKey    string    `yaml:"api_key,omitempty"` // For Temporal Cloud API key authentication
}

// ExpandEnv expands environment variables in sensitive fields.
// Supports ${VAR}, $VAR, and ${VAR:-default} syntax.
func (c ConnectionConfig) ExpandEnv() ConnectionConfig {
	return ConnectionConfig{
		Address:   c.Address,
		Namespace: c.Namespace,
		TLS:       c.TLS,
		APIKey:    expandEnvVar(c.APIKey),
	}
}

// expandEnvVar expands environment variable references in a string.
// Supports ${VAR}, $VAR, and ${VAR:-default} syntax.
func expandEnvVar(s string) string {
	if s == "" {
		return s
	}

	// Handle ${VAR} and ${VAR:-default} syntax
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		inner := s[2 : len(s)-1]
		// Check for default value syntax: ${VAR:-default}
		if idx := strings.Index(inner, ":-"); idx != -1 {
			varName := inner[:idx]
			defaultVal := inner[idx+2:]
			if val := os.Getenv(varName); val != "" {
				return val
			}
			return defaultVal
		}
		// Simple ${VAR} syntax
		return os.Getenv(inner)
	}

	// Handle $VAR syntax (must be the entire string)
	if strings.HasPrefix(s, "$") && !strings.ContainsAny(s[1:], " \t${}") {
		return os.Getenv(s[1:])
	}

	// Return as-is if no env var pattern detected
	return s
}

// ToTemporalConfig converts config.ConnectionConfig to temporal-compatible format.
// Returns address, namespace, TLS fields, and API key as separate values.
func (c ConnectionConfig) ToTemporalConfig() (address, namespace, tlsCert, tlsKey, tlsCA, tlsServerName string, tlsSkipVerify bool, apiKey string) {
	return c.Address, c.Namespace, c.TLS.Cert, c.TLS.Key, c.TLS.CA, c.TLS.ServerName, c.TLS.SkipVerify, c.APIKey
}

// FromTemporalConfig creates a ConnectionConfig from temporal-style flat fields.
func FromTemporalConfig(address, namespace, tlsCert, tlsKey, tlsCA, tlsServerName string, tlsSkipVerify bool, apiKey string) ConnectionConfig {
	return ConnectionConfig{
		Address:   address,
		Namespace: namespace,
		TLS: TLSConfig{
			Cert:       tlsCert,
			Key:        tlsKey,
			CA:         tlsCA,
			ServerName: tlsServerName,
			SkipVerify: tlsSkipVerify,
		},
		APIKey: apiKey,
	}
}

// SavedFilter represents a saved visibility query.
type SavedFilter struct {
	Name      string `yaml:"name"`
	Query     string `yaml:"query"`
	IsDefault bool   `yaml:"is_default,omitempty"`
}

// Config represents the application configuration.
type Config struct {
	Theme         string                      `yaml:"theme"`
	ActiveProfile string                      `yaml:"active_profile,omitempty"`
	Profiles      map[string]ConnectionConfig `yaml:"profiles,omitempty"`
	SavedFilters  []SavedFilter               `yaml:"saved_filters,omitempty"`
	CheckUpdates  *bool                       `yaml:"check_updates,omitempty"`
}

// ShouldCheckUpdates returns whether update checking is enabled.
// Defaults to true if not explicitly set.
func (c *Config) ShouldCheckUpdates() bool {
	if c.CheckUpdates == nil {
		return true
	}
	return *c.CheckUpdates
}

// DefaultConfig returns a config with default values.
func DefaultConfig() *Config {
	return &Config{
		Theme:         DefaultTheme,
		ActiveProfile: "default",
		Profiles: map[string]ConnectionConfig{
			"default": {
				Address:   "localhost:7233",
				Namespace: "default",
			},
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

	// Ensure profiles and active profile are set
	cfg.ensureDefaults()

	return cfg, nil
}

// ensureDefaults ensures the config has valid profiles and active profile.
func (c *Config) ensureDefaults() {
	if c.Profiles == nil || len(c.Profiles) == 0 {
		c.Profiles = map[string]ConnectionConfig{
			"default": {
				Address:   "localhost:7233",
				Namespace: "default",
			},
		}
		c.ActiveProfile = "default"
	}

	// Ensure ActiveProfile is set and valid
	if c.ActiveProfile == "" {
		for name := range c.Profiles {
			c.ActiveProfile = name
			break
		}
	} else if _, ok := c.Profiles[c.ActiveProfile]; !ok {
		// Active profile doesn't exist, use first available
		for name := range c.Profiles {
			c.ActiveProfile = name
			break
		}
	}
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

// GetProfile returns a profile by name.
func (c *Config) GetProfile(name string) (ConnectionConfig, bool) {
	if c.Profiles == nil {
		return ConnectionConfig{}, false
	}
	profile, ok := c.Profiles[name]
	return profile, ok
}

// GetActiveProfile returns the active profile name and its configuration.
func (c *Config) GetActiveProfile() (string, ConnectionConfig) {
	if c.Profiles == nil || c.ActiveProfile == "" {
		return "default", ConnectionConfig{
			Address:   "localhost:7233",
			Namespace: "default",
		}
	}
	profile, ok := c.Profiles[c.ActiveProfile]
	if !ok {
		// Active profile doesn't exist, return first available
		for name, cfg := range c.Profiles {
			return name, cfg
		}
		return "default", ConnectionConfig{
			Address:   "localhost:7233",
			Namespace: "default",
		}
	}
	return c.ActiveProfile, profile
}

// SetActiveProfile sets the active profile by name.
// Returns error if profile doesn't exist.
func (c *Config) SetActiveProfile(name string) error {
	if c.Profiles == nil {
		return fmt.Errorf("no profiles configured")
	}
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	c.ActiveProfile = name
	return nil
}

// SaveProfile saves or updates a profile.
func (c *Config) SaveProfile(name string, cfg ConnectionConfig) {
	if c.Profiles == nil {
		c.Profiles = make(map[string]ConnectionConfig)
	}
	c.Profiles[name] = cfg
}

// DeleteProfile deletes a profile by name.
// Returns error if trying to delete the active profile or if profile doesn't exist.
func (c *Config) DeleteProfile(name string) error {
	if c.Profiles == nil {
		return fmt.Errorf("profile %q not found", name)
	}
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	if c.ActiveProfile == name {
		return fmt.Errorf("cannot delete active profile %q", name)
	}
	delete(c.Profiles, name)
	return nil
}

// ListProfiles returns a sorted list of profile names.
func (c *Config) ListProfiles() []string {
	if c.Profiles == nil {
		return nil
	}
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	// Sort for consistent ordering
	sort.Strings(names)
	return names
}

// ProfileExists checks if a profile with the given name exists.
func (c *Config) ProfileExists(name string) bool {
	if c.Profiles == nil {
		return false
	}
	_, ok := c.Profiles[name]
	return ok
}

// Saved filter management methods

// GetSavedFilters returns all saved filters.
func (c *Config) GetSavedFilters() []SavedFilter {
	return c.SavedFilters
}

// GetSavedFilter returns a saved filter by name.
func (c *Config) GetSavedFilter(name string) (SavedFilter, bool) {
	for _, f := range c.SavedFilters {
		if f.Name == name {
			return f, true
		}
	}
	return SavedFilter{}, false
}

// SaveFilter adds or updates a saved filter.
func (c *Config) SaveFilter(filter SavedFilter) {
	// Check if filter with same name exists
	for i, f := range c.SavedFilters {
		if f.Name == filter.Name {
			c.SavedFilters[i] = filter
			return
		}
	}
	// Add new filter
	c.SavedFilters = append(c.SavedFilters, filter)
}

// DeleteFilter removes a saved filter by name.
func (c *Config) DeleteFilter(name string) error {
	for i, f := range c.SavedFilters {
		if f.Name == name {
			c.SavedFilters = append(c.SavedFilters[:i], c.SavedFilters[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("filter %q not found", name)
}

// GetDefaultFilter returns the default filter if one is set.
func (c *Config) GetDefaultFilter() (SavedFilter, bool) {
	for _, f := range c.SavedFilters {
		if f.IsDefault {
			return f, true
		}
	}
	return SavedFilter{}, false
}

// SetDefaultFilter sets a filter as the default, clearing any previous default.
func (c *Config) SetDefaultFilter(name string) error {
	found := false
	for i := range c.SavedFilters {
		if c.SavedFilters[i].Name == name {
			c.SavedFilters[i].IsDefault = true
			found = true
		} else {
			c.SavedFilters[i].IsDefault = false
		}
	}
	if !found {
		return fmt.Errorf("filter %q not found", name)
	}
	return nil
}

// ClearDefaultFilter clears the default filter.
func (c *Config) ClearDefaultFilter() {
	for i := range c.SavedFilters {
		c.SavedFilters[i].IsDefault = false
	}
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
