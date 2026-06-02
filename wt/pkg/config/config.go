package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Project represents a worktree project configuration
type Project struct {
	Name        string            `yaml:"name"`
	Path        string            `yaml:"path"`
	Tags        []string          `yaml:"tags,omitempty"`
	Category    string            `yaml:"category,omitempty"`
	Color       string            `yaml:"color,omitempty"`        // Hex color (6 chars, e.g., "d37cef") for workspace/devcontainer theming
	SubFolder   string            `yaml:"subfolder,omitempty"`    // Optional subdirectory within the checkout for workspace operations (e.g., monorepos)
	CommandArgs map[string]string `yaml:"command_args,omitempty"` // Per-project overrides for command arg defaults; key -> value
}

// Command is a user-defined shell command bound to a key and offered on nodes of a
// given scope. cwd is derived from the selected node (worktree path or project path + subfolder).
type Command struct {
	Key     string            `yaml:"key"`            // e.g. "g", "F", "ctrl+g" — must equal tea.KeyMsg.String()
	Label   string            `yaml:"label"`          // shown in the help footer, e.g. "lazygit"
	Scope   string            `yaml:"scope"`          // "worktree" | "project" | "any"
	Command string            `yaml:"command"`        // shell line, run via `sh -c`
	Args    map[string]string `yaml:"args,omitempty"` // arg name -> default value; injected as $WT_ARG_<name>
}

// Config represents the application configuration
type Config struct {
	Projects   []Project `yaml:"projects"`
	Categories []string  `yaml:"categories,omitempty"`
	Commands   []Command `yaml:"commands,omitempty"` // User-defined key bindings
}

// ReservedKeys is the set of keys that are reserved for built-in actions.
// NOTE: If you add a new built-in key to handleKeyPress in tui/model.go, add it here too.
var ReservedKeys = map[string]bool{
	"q":      true,
	"ctrl+c": true,
	"esc":    true,
	"/":      true,
	"up":     true,
	"down":   true,
	"k":      true,
	"j":      true,
	"enter":  true,
	"o":      true,
	" ":      true, // space
	"right":  true,
	"left":   true,
	"l":      true,
	"h":      true,
	"n":      true,
	"a":      true,
	"d":      true,
	"c":      true,
	"t":      true,
	"v":      true,
	"i":      true,
	"e":      true,
	"r":      true,
}

var validScopes = map[string]bool{
	"worktree": true,
	"project":  true,
	"any":      true,
}

var argNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateCommands validates the commands list in the config.
// Returns an error if any command is invalid.
func (c *Config) ValidateCommands() error {
	seen := make(map[string]bool) // key "(key,scope)"
	for i, cmd := range c.Commands {
		if cmd.Key == "" {
			return fmt.Errorf("custom command[%d]: key must not be empty", i)
		}
		if cmd.Label == "" {
			return fmt.Errorf("custom command[%d]: label must not be empty (key %q)", i, cmd.Key)
		}
		if cmd.Command == "" {
			return fmt.Errorf("custom command[%d]: command must not be empty (key %q)", i, cmd.Key)
		}
		if ReservedKeys[cmd.Key] {
			return fmt.Errorf("custom command: key %q is reserved by a built-in action", cmd.Key)
		}
		if !validScopes[cmd.Scope] {
			return fmt.Errorf("custom command: key %q has invalid scope %q (must be \"worktree\", \"project\", or \"any\")", cmd.Key, cmd.Scope)
		}
		pairKey := cmd.Key + "\x00" + cmd.Scope
		if seen[pairKey] {
			return fmt.Errorf("custom command: duplicate (key %q, scope %q)", cmd.Key, cmd.Scope)
		}
		seen[pairKey] = true
		for name := range cmd.Args {
			if !argNameRe.MatchString(name) {
				return fmt.Errorf("custom command: key %q has invalid arg name %q (must match [A-Za-z_][A-Za-z0-9_]*)", cmd.Key, name)
			}
		}
	}
	for _, p := range c.Projects {
		for name := range p.CommandArgs {
			if !argNameRe.MatchString(name) {
				return fmt.Errorf("project %q has invalid command_args name %q (must match [A-Za-z_][A-Za-z0-9_]*)", p.Name, name)
			}
		}
	}
	return nil
}

// ResolveArgs resolves the effective arg map for a command dispatched on a given project.
// Values in project.CommandArgs take precedence over cmd.Args defaults.
// Returns an empty map (never nil) when cmd has no args and project has no overrides.
func ResolveArgs(cmd Command, p *Project) map[string]string {
	result := make(map[string]string)
	for k, v := range cmd.Args {
		result[k] = v
	}
	if p != nil {
		for k, v := range p.CommandArgs {
			result[k] = v
		}
	}
	return result
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

	if err := config.ValidateCommands(); err != nil {
		return nil, err
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
