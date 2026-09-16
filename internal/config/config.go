package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile        = ".mlm.yaml"
	LegacyConfigFile         = ".mikrotik-lists-manager.yaml"
	ConfigSubDirName         = "mlm"
	LegacyConfigSubDirName   = "mikrotik-lists-manager"
	GlobalConfigFileName     = "config.yaml"
	DefaultHomeConfigDotfile = ".mlm.yaml"
	LegacyHomeConfigDotfile  = ".mikrotik-lists-manager.yaml"
)

// ProfileConfig holds connection and behaviour settings for a specific router profile.
type ProfileConfig struct {
	Host          string `yaml:"host,omitempty"`
	User          string `yaml:"user,omitempty"`
	Pass          string `yaml:"pass,omitempty"`
	List          string `yaml:"list,omitempty"`
	SkipTLSVerify *bool  `yaml:"insecure,omitempty"`
	DefaultFormat string `yaml:"default_format,omitempty"`
	Proxy         string `yaml:"proxy,omitempty"`
}

// Insecure returns the profile's SkipTLSVerify value or falls back to the global setting.
func (p ProfileConfig) Insecure(fallback bool) bool {
	if p.SkipTLSVerify != nil {
		return *p.SkipTLSVerify
	}
	return fallback
}

// Config holds all connection and behaviour settings, including profiles.
type Config struct {
	Host           string                   `yaml:"host,omitempty"`
	User           string                   `yaml:"user,omitempty"`
	Pass           string                   `yaml:"pass,omitempty"`
	List           string                   `yaml:"list,omitempty"`
	SkipTLSVerify  bool                     `yaml:"insecure,omitempty"`
	DefaultFormat  string                   `yaml:"default_format,omitempty"`
	Proxy          string                   `yaml:"proxy,omitempty"`
	DefaultProfile string                   `yaml:"default_profile,omitempty"`
	Profiles       map[string]ProfileConfig `yaml:"profiles,omitempty"`
}

// DefaultGlobalConfigPath returns the cross-platform path to global config.
func DefaultGlobalConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("getting UserConfigDir: %w", err)
	}
	return filepath.Join(dir, ConfigSubDirName, GlobalConfigFileName), nil
}

// FindConfigFile looks for a config file following the priority:
// 1. Explicit path (if non-empty)
// 2. ./.mlm.yaml (or legacy ./.mikrotik-lists-manager.yaml) in current directory
// 3. System user config dir (os.UserConfigDir()/mlm/config.yaml or legacy)
// 4. ~/.mlm.yaml (or legacy ~/.mikrotik-lists-manager.yaml)
func FindConfigFile(explicitPath string) (path string, found bool, err error) {
	if explicitPath != "" {
		if fi, err := os.Stat(explicitPath); err == nil && !fi.IsDir() {
			return explicitPath, true, nil
		}
		return explicitPath, false, fmt.Errorf("конфигурационный файл %q не найден", explicitPath)
	}

	// 1. Current directory
	if fi, err := os.Stat(DefaultConfigFile); err == nil && !fi.IsDir() {
		return DefaultConfigFile, true, nil
	}
	if fi, err := os.Stat(LegacyConfigFile); err == nil && !fi.IsDir() {
		return LegacyConfigFile, true, nil
	}

	// 2. Global user config dir
	if globalPath, err := DefaultGlobalConfigPath(); err == nil {
		if fi, err := os.Stat(globalPath); err == nil && !fi.IsDir() {
			return globalPath, true, nil
		}
	}
	if dir, err := os.UserConfigDir(); err == nil {
		legacyGlobal := filepath.Join(dir, LegacyConfigSubDirName, GlobalConfigFileName)
		if fi, err := os.Stat(legacyGlobal); err == nil && !fi.IsDir() {
			return legacyGlobal, true, nil
		}
	}

	// 3. Home dotfile
	if home, err := os.UserHomeDir(); err == nil {
		homePath := filepath.Join(home, DefaultHomeConfigDotfile)
		if fi, err := os.Stat(homePath); err == nil && !fi.IsDir() {
			return homePath, true, nil
		}
		legacyHome := filepath.Join(home, LegacyHomeConfigDotfile)
		if fi, err := os.Stat(legacyHome); err == nil && !fi.IsDir() {
			return legacyHome, true, nil
		}
	}

	return "", false, nil
}

// EffectiveProfile returns the merged configuration for the given profile name.
// If profileName is empty, it uses c.DefaultProfile.
// Profile values override global values.
func (c *Config) EffectiveProfile(profileName string) (ProfileConfig, error) {
	if profileName == "" {
		profileName = c.DefaultProfile
	}

	effective := ProfileConfig{
		Host:          c.Host,
		User:          c.User,
		Pass:          c.Pass,
		List:          c.List,
		SkipTLSVerify: &c.SkipTLSVerify,
		DefaultFormat: c.DefaultFormat,
		Proxy:         c.Proxy,
	}

	if profileName == "" {
		return effective, nil
	}

	prof, exists := c.Profiles[profileName]
	if !exists {
		var names []string
		for k := range c.Profiles {
			names = append(names, k)
		}
		sort.Strings(names)
		if len(names) == 0 {
			return effective, fmt.Errorf("профиль %q не найден: в конфиге отсутствуют профили", profileName)
		}
		return effective, fmt.Errorf("профиль %q не найден (доступные профили: %s)", profileName, strings.Join(names, ", "))
	}

	if prof.Host != "" {
		effective.Host = prof.Host
	}
	if prof.User != "" {
		effective.User = prof.User
	}
	if prof.Pass != "" {
		effective.Pass = prof.Pass
	}
	if prof.List != "" {
		effective.List = prof.List
	}
	if prof.SkipTLSVerify != nil {
		effective.SkipTLSVerify = prof.SkipTLSVerify
	}
	if prof.DefaultFormat != "" {
		effective.DefaultFormat = prof.DefaultFormat
	}
	if prof.Proxy != "" {
		effective.Proxy = prof.Proxy
	}

	return effective, nil
}

// Load reads a config file. Returns empty Config (not an error) if the path is empty or file does not exist.
func Load(path string) (Config, error) {
	var cfg Config
	if path == "" {
		return cfg, nil
	}
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Template returns the annotated YAML template written by `config init`.
func Template() string {
	return `# mlm configuration
# Priority: CLI flag > env > selected profile > global config > defaults

# Profile to use when --profile / -P is omitted
default_profile: ""

# Global defaults (used when no profile is selected, or as fallback)
host: ""
user: ""
pass: ""
list: ""
insecure: false
default_format: auto
# proxy: "socks5://127.0.0.1:1080" # socks5, socks5h, http, https

# Router profiles: mlm <cmd> -P <name>
profiles:
  office:
    host: "192.168.88.1"
    user: "admin"
    pass: ""
    list: "office-routes"
    insecure: false
    default_format: native
    # proxy: "http://proxy.corp:8080"
  home:
    host: "192.168.1.1:8443"
    user: "admin"
    pass: ""
    list: "vpn-routes"
    insecure: true
    default_format: auto
    # proxy: "socks5://127.0.0.1:1080"
`
}
