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

func TestProjectTags(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	// Create test config with tags
	testConfig := &Config{
		Projects: []Project{
			{Name: "project1", Path: "/path/to/project1", Tags: []string{"frontend", "typescript"}},
			{Name: "project2", Path: "/path/to/project2", Tags: []string{"backend", "go"}},
		},
	}

	// Initialize and save
	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	err = SaveConfig(testConfig)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Load and verify tags
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(loadedConfig.Projects[0].Tags) != 2 {
		t.Errorf("Expected 2 tags for project1, got %d", len(loadedConfig.Projects[0].Tags))
	}

	if loadedConfig.Projects[0].Tags[0] != "frontend" {
		t.Errorf("Expected tag 'frontend', got '%s'", loadedConfig.Projects[0].Tags[0])
	}
}

func TestFindProject(t *testing.T) {
	config := &Config{
		Projects: []Project{
			{Name: "project1", Path: "/path/to/project1"},
			{Name: "project2", Path: "/path/to/project2"},
		},
	}

	// Test finding existing project
	project, err := config.FindProject("project1")
	if err != nil {
		t.Fatalf("FindProject failed: %v", err)
	}

	if project.Name != "project1" {
		t.Errorf("Expected project name 'project1', got '%s'", project.Name)
	}

	// Test finding non-existent project
	_, err = config.FindProject("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent project, got nil")
	}
}

func TestAddTags(t *testing.T) {
	project := &Project{
		Name: "test",
		Path: "/test/path",
		Tags: []string{"existing"},
	}

	// Add new tag
	modified := project.AddTags("new-tag")
	if !modified {
		t.Error("Expected AddTags to return true when adding new tag")
	}

	if len(project.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(project.Tags))
	}

	// Add duplicate tag
	modified = project.AddTags("existing")
	if modified {
		t.Error("Expected AddTags to return false when adding duplicate tag")
	}

	if len(project.Tags) != 2 {
		t.Errorf("Expected 2 tags after duplicate add, got %d", len(project.Tags))
	}

	// Add multiple tags at once
	modified = project.AddTags("tag1", "tag2", "existing")
	if !modified {
		t.Error("Expected AddTags to return true when adding new tags")
	}

	if len(project.Tags) != 4 {
		t.Errorf("Expected 4 tags, got %d", len(project.Tags))
	}
}

func TestRemoveTags(t *testing.T) {
	project := &Project{
		Name: "test",
		Path: "/test/path",
		Tags: []string{"tag1", "tag2", "tag3"},
	}

	// Remove existing tag
	modified := project.RemoveTags("tag2")
	if !modified {
		t.Error("Expected RemoveTags to return true when removing existing tag")
	}

	if len(project.Tags) != 2 {
		t.Errorf("Expected 2 tags after removal, got %d", len(project.Tags))
	}

	// Verify correct tag was removed
	for _, tag := range project.Tags {
		if tag == "tag2" {
			t.Error("tag2 should have been removed")
		}
	}

	// Remove non-existent tag
	modified = project.RemoveTags("nonexistent")
	if modified {
		t.Error("Expected RemoveTags to return false when removing non-existent tag")
	}

	if len(project.Tags) != 2 {
		t.Errorf("Expected 2 tags after removing non-existent tag, got %d", len(project.Tags))
	}

	// Remove multiple tags
	modified = project.RemoveTags("tag1", "tag3")
	if !modified {
		t.Error("Expected RemoveTags to return true when removing multiple tags")
	}

	if len(project.Tags) != 0 {
		t.Errorf("Expected 0 tags after removing all, got %d", len(project.Tags))
	}
}

func TestGetAllTags(t *testing.T) {
	config := &Config{
		Projects: []Project{
			{Name: "project1", Path: "/path/1", Tags: []string{"frontend", "typescript"}},
			{Name: "project2", Path: "/path/2", Tags: []string{"backend", "go"}},
			{Name: "project3", Path: "/path/3", Tags: []string{"frontend", "react"}},
		},
	}

	tags := config.GetAllTags()

	// Should have 5 unique tags
	expectedTags := map[string]bool{
		"frontend":   true,
		"typescript": true,
		"backend":    true,
		"go":         true,
		"react":      true,
	}

	if len(tags) != len(expectedTags) {
		t.Errorf("Expected %d unique tags, got %d", len(expectedTags), len(tags))
	}

	for _, tag := range tags {
		if !expectedTags[tag] {
			t.Errorf("Unexpected tag: %s", tag)
		}
	}
}

func TestValidateCommands(t *testing.T) {
	valid := Config{
		Commands: []Command{
			{Key: "g", Label: "lazygit", Scope: "worktree", Command: "lazygit"},
			{Key: "F", Label: "fetch", Scope: "project", Command: "git fetch --all"},
			{Key: "g", Label: "any-g", Scope: "any", Command: "echo hi"},
		},
	}
	if err := valid.ValidateCommands(); err != nil {
		t.Errorf("expected valid config to pass, got: %v", err)
	}

	// reserved key
	c := Config{Commands: []Command{{Key: "d", Label: "lbl", Scope: "worktree", Command: "ls"}}}
	if err := c.ValidateCommands(); err == nil {
		t.Error("expected error for reserved key 'd'")
	}

	// empty key
	c = Config{Commands: []Command{{Key: "", Label: "lbl", Scope: "worktree", Command: "ls"}}}
	if err := c.ValidateCommands(); err == nil {
		t.Error("expected error for empty key")
	}

	// empty label
	c = Config{Commands: []Command{{Key: "g", Label: "", Scope: "worktree", Command: "ls"}}}
	if err := c.ValidateCommands(); err == nil {
		t.Error("expected error for empty label")
	}

	// empty command
	c = Config{Commands: []Command{{Key: "g", Label: "lbl", Scope: "worktree", Command: ""}}}
	if err := c.ValidateCommands(); err == nil {
		t.Error("expected error for empty command")
	}

	// invalid scope
	c = Config{Commands: []Command{{Key: "g", Label: "lbl", Scope: "global", Command: "ls"}}}
	if err := c.ValidateCommands(); err == nil {
		t.Error("expected error for invalid scope")
	}

	// duplicate (key, scope)
	c = Config{Commands: []Command{
		{Key: "g", Label: "lbl1", Scope: "worktree", Command: "ls"},
		{Key: "g", Label: "lbl2", Scope: "worktree", Command: "echo"},
	}}
	if err := c.ValidateCommands(); err == nil {
		t.Error("expected error for duplicate (key, scope)")
	}

	// same key different scope is OK
	c = Config{Commands: []Command{
		{Key: "g", Label: "lbl1", Scope: "worktree", Command: "ls"},
		{Key: "g", Label: "lbl2", Scope: "project", Command: "echo"},
	}}
	if err := c.ValidateCommands(); err != nil {
		t.Errorf("same key different scope should be valid, got: %v", err)
	}

	// illegal arg name
	c = Config{Commands: []Command{{Key: "g", Label: "lbl", Scope: "worktree", Command: "ls", Args: map[string]string{"bad-name": "val"}}}}
	if err := c.ValidateCommands(); err == nil {
		t.Error("expected error for illegal arg name in command.args")
	}

	// illegal project command_args name
	c = Config{
		Projects: []Project{{Name: "p", Path: "/p", CommandArgs: map[string]string{"bad-name": "val"}}},
	}
	if err := c.ValidateCommands(); err == nil {
		t.Error("expected error for illegal arg name in project.command_args")
	}

	// no commands -> nil slice loads fine
	c = Config{}
	if err := c.ValidateCommands(); err != nil {
		t.Errorf("empty commands should be valid, got: %v", err)
	}
}

func TestResolveArgs(t *testing.T) {
	cmd := Command{
		Key:     "s",
		Label:   "scaffold",
		Scope:   "project",
		Command: "echo $WT_ARG_template",
		Args:    map[string]string{"template": "default-template", "env": "dev"},
	}

	// No project -> use defaults
	result := ResolveArgs(cmd, nil)
	if result["template"] != "default-template" {
		t.Errorf("expected 'default-template', got %q", result["template"])
	}
	if result["env"] != "dev" {
		t.Errorf("expected 'dev', got %q", result["env"])
	}

	// Project overrides one key
	p := &Project{CommandArgs: map[string]string{"template": "api-template"}}
	result = ResolveArgs(cmd, p)
	if result["template"] != "api-template" {
		t.Errorf("project override should win: expected 'api-template', got %q", result["template"])
	}
	if result["env"] != "dev" {
		t.Errorf("non-overridden default should remain: expected 'dev', got %q", result["env"])
	}

	// Project-only key (not in command.Args) is included
	p2 := &Project{CommandArgs: map[string]string{"extra": "val"}}
	result = ResolveArgs(cmd, p2)
	if result["extra"] != "val" {
		t.Errorf("project-only key should be included, got %q", result["extra"])
	}

	// Empty default yields empty string
	cmdEmpty := Command{Args: map[string]string{"x": ""}}
	result = ResolveArgs(cmdEmpty, nil)
	if v, ok := result["x"]; !ok || v != "" {
		t.Errorf("empty default should yield empty string, got %q (ok=%v)", v, ok)
	}

	// No args, no project command_args -> empty map (not nil)
	cmdNoArgs := Command{Key: "z", Label: "z", Scope: "any", Command: "echo"}
	result = ResolveArgs(cmdNoArgs, nil)
	if result == nil {
		t.Error("ResolveArgs should never return nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestFilterProjectsByTag(t *testing.T) {
	config := &Config{
		Projects: []Project{
			{Name: "project1", Path: "/path/1", Tags: []string{"frontend", "typescript"}},
			{Name: "project2", Path: "/path/2", Tags: []string{"backend", "go"}},
			{Name: "project3", Path: "/path/3", Tags: []string{"frontend", "react"}},
		},
	}

	// Filter by frontend tag
	frontendProjects := config.FilterProjectsByTag("frontend")
	if len(frontendProjects) != 2 {
		t.Errorf("Expected 2 frontend projects, got %d", len(frontendProjects))
	}

	// Filter by backend tag
	backendProjects := config.FilterProjectsByTag("backend")
	if len(backendProjects) != 1 {
		t.Errorf("Expected 1 backend project, got %d", len(backendProjects))
	}

	if backendProjects[0].Name != "project2" {
		t.Errorf("Expected project2, got %s", backendProjects[0].Name)
	}

	// Filter by non-existent tag
	noneProjects := config.FilterProjectsByTag("nonexistent")
	if len(noneProjects) != 0 {
		t.Errorf("Expected 0 projects for non-existent tag, got %d", len(noneProjects))
	}
}
