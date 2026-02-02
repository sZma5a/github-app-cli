package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// Dir returns the configuration directory path, respecting XDG_CONFIG_HOME.
func Dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, configDir), nil
	}
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
			return nil, fmt.Errorf("configuration not found - run 'gha configure' first")
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.AppID <= 0 {
		return nil, fmt.Errorf("app_id must be a positive integer")
	}
	if cfg.InstallationID < 0 {
		return nil, fmt.Errorf("installation_id must not be negative")
	}
	if strings.TrimSpace(cfg.PrivateKeyPath) == "" {
		return nil, fmt.Errorf("private_key_path is required in config")
	}
	cfg.PrivateKeyPath = filepath.Clean(strings.TrimSpace(cfg.PrivateKeyPath))

	return &cfg, nil
}

// Save writes configuration to disk with secure file permissions.
func Save(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config must not be nil")
	}

	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("setting config directory permissions: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, configFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("setting config file permissions: %w", err)
	}

	return nil
}
