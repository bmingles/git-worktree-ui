package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Project represents a worktree project configuration
type Project struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// Config represents the application configuration
type Config struct {
	Projects []Project `yaml:"projects"`
}

// customConfigPath holds the custom config path if set via --config flag
var customConfigPath string

// SetConfigPath sets a custom config path
func SetConfigPath(path string) {
	customConfigPath = path
}

// GetConfigPath returns the path to the config file (exported for use in cmd package)
func GetConfigPath() (string, error) {
	return getConfigPath()
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	// Use custom path if set
	if customConfigPath != "" {
		return customConfigPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "wt", "config.yaml"), nil
}

// getConfigDir returns the path to the config directory
func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "wt"), nil
}

// InitConfig creates the config directory and a default config file if they don't exist
func InitConfig() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Check if config file already exists
	if _, err := os.Stat(configPath); err == nil {
		// Config file already exists
		return nil
	}

	// Create default config
	defaultConfig := Config{
		Projects: []Project{},
	}

	return SaveConfig(&defaultConfig)
}

// LoadConfig reads the config file and returns a Config struct
func LoadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Initialize config if it doesn't exist
		if err := InitConfig(); err != nil {
			return nil, fmt.Errorf("failed to initialize config: %w", err)
		}
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig writes the Config struct to the config file
func SaveConfig(config *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
