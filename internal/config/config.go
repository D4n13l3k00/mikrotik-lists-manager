package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = ".mikrotik-lists-manager.yaml"

// Config holds all connection and behaviour settings.
type Config struct {
	Host          string `yaml:"host"`
	User          string `yaml:"user"`
	Pass          string `yaml:"pass"`
	List          string `yaml:"list"`
	SkipTLSVerify bool   `yaml:"insecure"`
	// DefaultFormat for sync input: auto, native, mikrotik
	DefaultFormat string `yaml:"default_format"`
}

// Load reads a config file. Returns empty Config (not an error) if the file does not exist.
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes cfg to path, creating or overwriting the file.
func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Template returns the annotated YAML template written by `config init`.
func Template() string {
	return `# mikrotik-lists-manager configuration
# All values can be overridden by CLI flags or environment variables.
# Priority: flag > env > this file

# MikroTik router address. Can include port: 192.168.1.1:443
host: ""

# API username
user: ""

# API password (optional — will be prompted interactively if omitted)
pass: ""

# Address-list name to sync
list: ""

# Skip TLS certificate verification (useful for self-signed certs)
insecure: false

# Default input format for sync: auto, native, mikrotik
default_format: auto
`
}
