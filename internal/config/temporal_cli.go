package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// LoadTemporalCLIProfiles discovers and loads profiles from the Temporal CLI
// configuration files. It checks YAML first (takes precedence per temporal CLI
// behavior), then TOML. Returns a merged map with profile names as keys.
func LoadTemporalCLIProfiles() map[string]ConnectionConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	configDir := filepath.Join(home, ".config", "temporalio")
	profiles := make(map[string]ConnectionConfig)

	// Load YAML envs first (takes precedence)
	yamlPath := filepath.Join(configDir, "temporal.yaml")
	yamlProfiles, err := loadTemporalEnvYAML(yamlPath)
	if err == nil {
		for name, cfg := range yamlProfiles {
			profiles[name] = cfg
		}
	}

	// Load TOML profiles; YAML entries take precedence on conflict
	tomlPath := filepath.Join(configDir, "temporal.toml")
	tomlProfiles, err := loadTemporalProfileTOML(tomlPath)
	if err == nil {
		for name, cfg := range tomlProfiles {
			if _, exists := profiles[name]; !exists {
				profiles[name] = cfg
			}
		}
	}

	if len(profiles) == 0 {
		return nil
	}
	return profiles
}

// temporalYAMLConfig represents the top-level structure of temporal.yaml.
type temporalYAMLConfig struct {
	Env map[string]map[string]string `yaml:"env"`
}

func loadTemporalEnvYAML(path string) (map[string]ConnectionConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg temporalYAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	profiles := make(map[string]ConnectionConfig)
	for name, props := range cfg.Env {
		conn := ConnectionConfig{
			Address:   props["address"],
			Namespace: props["namespace"],
			TLS: TLSConfig{
				Cert:       props["tls-cert-path"],
				Key:        props["tls-key-path"],
				CA:         props["tls-ca-path"],
				ServerName: props["tls-server-name"],
			},
			APIKey: props["api-key"],
		}
		profiles[name] = conn
	}

	return profiles, nil
}

// temporalTOMLConfig represents the top-level structure of temporal.toml.
type temporalTOMLConfig struct {
	Profile map[string]temporalTOMLProfile `toml:"profile"`
}

type temporalTOMLProfile struct {
	Address   string              `toml:"address"`
	Namespace string              `toml:"namespace"`
	APIKey    string              `toml:"api_key"`
	TLS       temporalTOMLTLS     `toml:"tls"`
}

type temporalTOMLTLS struct {
	ClientCertPath string `toml:"client_cert_path"`
	ClientKeyPath  string `toml:"client_key_path"`
	CAPath         string `toml:"ca_path"`
	ServerName     string `toml:"server_name"`
}

func loadTemporalProfileTOML(path string) (map[string]ConnectionConfig, error) {
	var cfg temporalTOMLConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}

	profiles := make(map[string]ConnectionConfig)
	for name, p := range cfg.Profile {
		conn := ConnectionConfig{
			Address:   p.Address,
			Namespace: p.Namespace,
			TLS: TLSConfig{
				Cert:       p.TLS.ClientCertPath,
				Key:        p.TLS.ClientKeyPath,
				CA:         p.TLS.CAPath,
				ServerName: p.TLS.ServerName,
			},
			APIKey: p.APIKey,
		}
		profiles[name] = conn
	}

	return profiles, nil
}
