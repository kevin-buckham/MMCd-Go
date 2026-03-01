//go:build !cli

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// UserConfig holds persistent user preferences.
type UserConfig struct {
	SelectedSensors []string `json:"selectedSensors,omitempty"`
	BaudRate        int      `json:"baudRate,omitempty"`
	LastPort        string   `json:"lastPort,omitempty"`
}

// configDir returns the path to the mmcd config directory (~/.config/mmcd/).
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "mmcd"), nil
}

// configPath returns the full path to config.json.
func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig reads the user config from disk. Returns a zero-value config if the file doesn't exist.
func LoadConfig() (UserConfig, error) {
	path, err := configPath()
	if err != nil {
		return UserConfig{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return UserConfig{}, nil
		}
		return UserConfig{}, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg UserConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return UserConfig{}, fmt.Errorf("failed to parse config: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes the user config to disk.
func SaveConfig(cfg UserConfig) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}
