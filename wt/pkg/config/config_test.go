package config

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestConfig creates a temporary config directory for testing
func setupTestConfig(t *testing.T) (string, func()) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "wt-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Set HOME to temp directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)

	// Return cleanup function
	cleanup := func() {
		os.Setenv("HOME", originalHome)
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestInitConfig(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// Test InitConfig creates directory and file
	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	// Verify config directory exists
	configDir, err := getConfigDir()
	if err != nil {
		t.Fatalf("Failed to get config dir: %v", err)
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Errorf("Config directory was not created: %s", configDir)
	}

	// Verify config file exists
	configPath, err := getConfigPath()
	if err != nil {
		t.Fatalf("Failed to get config path: %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config file was not created: %s", configPath)
	}

	// Test InitConfig is idempotent (calling again should not error)
	err = InitConfig()
	if err != nil {
		t.Errorf("InitConfig should be idempotent, but got error: %v", err)
	}
}

func TestLoadConfig_NewConfig(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// Load config (should initialize if not exists)
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify empty projects list
	if config.Projects == nil {
		t.Error("Projects should be initialized, not nil")
	}

	if len(config.Projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(config.Projects))
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// Create test config
	testConfig := &Config{
		Projects: []Project{
			{Name: "project1", Path: "/path/to/project1"},
			{Name: "project2", Path: "/path/to/project2"},
		},
	}

	// Initialize config directory
	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	// Save config
	err = SaveConfig(testConfig)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Load config
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify projects
	if len(loadedConfig.Projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(loadedConfig.Projects))
	}

	if loadedConfig.Projects[0].Name != "project1" {
		t.Errorf("Expected project name 'project1', got '%s'", loadedConfig.Projects[0].Name)
	}

	if loadedConfig.Projects[0].Path != "/path/to/project1" {
		t.Errorf("Expected project path '/path/to/project1', got '%s'", loadedConfig.Projects[0].Path)
	}

	if loadedConfig.Projects[1].Name != "project2" {
		t.Errorf("Expected project name 'project2', got '%s'", loadedConfig.Projects[1].Name)
	}

	if loadedConfig.Projects[1].Path != "/path/to/project2" {
		t.Errorf("Expected project path '/path/to/project2', got '%s'", loadedConfig.Projects[1].Path)
	}
}

func TestSaveConfig_CreatesFile(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// Initialize config
	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	testConfig := &Config{
		Projects: []Project{
			{Name: "test", Path: "/test/path"},
		},
	}

	// Save config
	err = SaveConfig(testConfig)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file exists and has content
	configPath, err := getConfigPath()
	if err != nil {
		t.Fatalf("Failed to get config path: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Config file is empty")
	}
}

func TestConfigPaths(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// Test getConfigPath
	configPath, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath failed: %v", err)
	}

	expectedPath := filepath.Join(os.Getenv("HOME"), ".config", "wt", "config.yaml")
	if configPath != expectedPath {
		t.Errorf("Expected config path '%s', got '%s'", expectedPath, configPath)
	}

	// Test getConfigDir
	configDir, err := getConfigDir()
	if err != nil {
		t.Fatalf("getConfigDir failed: %v", err)
	}

	expectedDir := filepath.Join(os.Getenv("HOME"), ".config", "wt")
	if configDir != expectedDir {
		t.Errorf("Expected config dir '%s', got '%s'", expectedDir, configDir)
	}
}

func TestEmptyConfig(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// Initialize with empty config
	emptyConfig := &Config{
		Projects: []Project{},
	}

	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	err = SaveConfig(emptyConfig)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Load and verify
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loadedConfig.Projects == nil {
		t.Error("Projects should not be nil")
	}

	if len(loadedConfig.Projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(loadedConfig.Projects))
	}
}
