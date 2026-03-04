package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Project represents a worktree project configuration
type Project struct {
	Name     string   `yaml:"name"`
	Path     string   `yaml:"path"`
	Tags     []string `yaml:"tags,omitempty"`
	Category string   `yaml:"category,omitempty"`
	Color    string   `yaml:"color,omitempty"` // Hex color (6 chars, e.g., "d37cef") for workspace/devcontainer theming
}

// Config represents the application configuration
type Config struct {
	Projects   []Project `yaml:"projects"`
	Categories []string  `yaml:"categories,omitempty"`
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

// FindProject finds a project by name in the config
func (c *Config) FindProject(name string) (*Project, error) {
	for i := range c.Projects {
		if c.Projects[i].Name == name {
			return &c.Projects[i], nil
		}
	}
	return nil, fmt.Errorf("project '%s' not found", name)
}

// AddTags adds tags to a project if they don't already exist
func (p *Project) AddTags(tags ...string) bool {
	modified := false
	for _, tag := range tags {
		// Check if tag already exists
		exists := false
		for _, t := range p.Tags {
			if t == tag {
				exists = true
				break
			}
		}
		if !exists {
			p.Tags = append(p.Tags, tag)
			modified = true
		}
	}
	return modified
}

// RemoveTags removes tags from a project
func (p *Project) RemoveTags(tags ...string) bool {
	modified := false
	for _, tag := range tags {
		for i := 0; i < len(p.Tags); i++ {
			if p.Tags[i] == tag {
				p.Tags = append(p.Tags[:i], p.Tags[i+1:]...)
				i--
				modified = true
			}
		}
	}
	return modified
}

// GetAllTags returns all unique tags across all projects
func (c *Config) GetAllTags() []string {
	tagSet := make(map[string]bool)
	for _, p := range c.Projects {
		for _, tag := range p.Tags {
			tagSet[tag] = true
		}
	}
	
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags
}

// FilterProjectsByTag returns projects that have the specified tag
func (c *Config) FilterProjectsByTag(tag string) []Project {
	var filtered []Project
	for _, p := range c.Projects {
		for _, t := range p.Tags {
			if t == tag {
				filtered = append(filtered, p)
				break
			}
		}
	}
	return filtered
}

// AddCategory adds a category to the config's categories list if it doesn't already exist
func (c *Config) AddCategory(category string) bool {
	// Check if category already exists
	for _, cat := range c.Categories {
		if cat == category {
			return false
		}
	}
	c.Categories = append(c.Categories, category)
	return true
}

// SetProjectCategory sets the category for a project
func (p *Project) SetCategory(category string) {
	p.Category = category
}
