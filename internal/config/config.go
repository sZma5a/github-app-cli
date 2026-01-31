package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	configDir  = "github-app-cli"
	configFile = "config.yaml"
)

// Config holds GitHub App credentials.
type Config struct {
	AppID          int64  `yaml:"app_id"`
	InstallationID int64  `yaml:"installation_id"`
	PrivateKeyPath string `yaml:"private_key_path"`
}

// Dir returns the configuration directory path.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", configDir), nil
}

// Load reads configuration from disk.
func Load() (*Config, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, configFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration not found — run 'github-app-cli configure' first")
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.AppID == 0 {
		return nil, fmt.Errorf("app_id is required in config")
	}
	if cfg.InstallationID == 0 {
		return nil, fmt.Errorf("installation_id is required in config")
	}
	if cfg.PrivateKeyPath == "" {
		return nil, fmt.Errorf("private_key_path is required in config")
	}

	return &cfg, nil
}

// Save writes configuration to disk.
func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, configFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
