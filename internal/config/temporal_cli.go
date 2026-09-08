package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// LoadTemporalCLIProfiles discovers and loads profiles from the Temporal CLI
// configuration files. It checks YAML first (takes precedence per temporal CLI
// behavior), then TOML. Returns a merged map with profile names as keys.
func LoadTemporalCLIProfiles() map[string]ConnectionConfig {
	profiles := make(map[string]ConnectionConfig)

	// Load YAML envs first (takes precedence)
	if home, err := os.UserHomeDir(); err == nil {
		yamlPath := filepath.Join(home, ".config", "temporalio", "temporal.yaml")
		yamlProfiles, err := loadTemporalEnvYAML(yamlPath)
		if err == nil {
			for name, cfg := range yamlProfiles {
				profiles[name] = cfg
			}
		}
	}

	// Load TOML profiles; YAML entries take precedence on conflict
	tomlPath := temporalTOMLPath()
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

func temporalTOMLPath() string {
	if path := os.Getenv("TEMPORAL_CONFIG_FILE"); path != "" {
		return path
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "temporalio", "temporal.toml")
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
			Codec: CodecConfig{
				Endpoint: props["codec-endpoint"],
				Auth:     props["codec-auth"],
			},
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
	Address   string            `toml:"address"`
	Namespace string            `toml:"namespace"`
	APIKey    string            `toml:"api_key"`
	TLS       *temporalTOMLTLS  `toml:"tls"`
	Codec     temporalTOMLCodec `toml:"codec"`
	GRPCMeta  map[string]string `toml:"grpc_meta"`
	Authority string            `toml:"authority"`
}

type temporalTOMLCodec struct {
	Endpoint string `toml:"endpoint"`
	Auth     string `toml:"auth"`
}

type temporalTOMLTLS struct {
	Disabled                bool   `toml:"disabled"`
	ClientCertPath          string `toml:"client_cert_path"`
	ClientCertData          string `toml:"client_cert_data"`
	ClientKeyPath           string `toml:"client_key_path"`
	ClientKeyData           string `toml:"client_key_data"`
	ServerCACertPath        string `toml:"server_ca_cert_path"`
	ServerCACertData        string `toml:"server_ca_cert_data"`
	LegacyCAPath            string `toml:"ca_path"`
	ServerName              string `toml:"server_name"`
	DisableHostVerification bool   `toml:"disable_host_verification"`
}

func loadTemporalProfileTOML(path string) (map[string]ConnectionConfig, error) {
	var cfg temporalTOMLConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}

	profiles := make(map[string]ConnectionConfig)
	for name, p := range cfg.Profile {
		var tlsConfig TLSConfig
		if p.TLS != nil {
			serverCAPath := p.TLS.ServerCACertPath
			if serverCAPath == "" {
				serverCAPath = p.TLS.LegacyCAPath
			}
			tlsConfig = TLSConfig{
				Enabled:    true,
				Disabled:   p.TLS.Disabled,
				Cert:       p.TLS.ClientCertPath,
				CertData:   p.TLS.ClientCertData,
				Key:        p.TLS.ClientKeyPath,
				KeyData:    p.TLS.ClientKeyData,
				CA:         serverCAPath,
				CAData:     p.TLS.ServerCACertData,
				ServerName: p.TLS.ServerName,
				SkipVerify: p.TLS.DisableHostVerification,
			}
		}
		var grpcMeta map[string]string
		if len(p.GRPCMeta) > 0 {
			grpcMeta = make(map[string]string, len(p.GRPCMeta))
			for key, value := range p.GRPCMeta {
				grpcMeta[normalizeGRPCMetaKey(key)] = value
			}
		}
		conn := ConnectionConfig{
			Address:   p.Address,
			Namespace: p.Namespace,
			Codec: CodecConfig{
				Endpoint: p.Codec.Endpoint,
				Auth:     p.Codec.Auth,
			},
			TLS:       tlsConfig,
			APIKey:    p.APIKey,
			GRPCMeta:  grpcMeta,
			Authority: p.Authority,
		}
		profiles[name] = conn
	}

	return profiles, nil
}

func normalizeGRPCMetaKey(key string) string {
	return strings.ToLower(strings.ReplaceAll(key, "_", "-"))
}
