package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateColorFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "basic path",
			path:     "/path/to/project",
			expected: "3e5a7b", // First 6 chars of MD5 hash
		},
		{
			name:     "another path",
			path:     "/different/path",
			expected: "dd3f6e",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateColorFromPath(tt.path)
			if len(result) != 6 {
				t.Errorf("GenerateColorFromPath() returned color with length %d, expected 6", len(result))
			}
			// Verify it's a valid hex string
			for _, c := range result {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("GenerateColorFromPath() returned invalid hex character: %c", c)
				}
			}
		})
	}
}

func TestGetContrastingForeground(t *testing.T) {
	tests := []struct {
		name     string
		bgColor  string
		expected string
	}{
		{
			name:     "light background",
			bgColor:  "ffffff",
			expected: "#000000",
		},
		{
			name:     "light background with hash",
			bgColor:  "#ffffff",
			expected: "#000000",
		},
		{
			name:     "dark background",
			bgColor:  "000000",
			expected: "#ffffff",
		},
		{
			name:     "medium-light background",
			bgColor:  "d37cef",
			expected: "#000000",
		},
		{
			name:     "medium-dark background",
			bgColor:  "3e5a7b",
			expected: "#ffffff",
		},
		{
			name:     "invalid color",
			bgColor:  "xyz",
			expected: "#ffffff",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetContrastingForeground(tt.bgColor)
			if result != tt.expected {
				t.Errorf("GetContrastingForeground(%s) = %s, expected %s", tt.bgColor, result, tt.expected)
			}
		})
	}
}

func TestAdjustColorBrightness(t *testing.T) {
	tests := []struct {
		name       string
		hexColor   string
		percentage float64
	}{
		{
			name:       "darken by 15%",
			hexColor:   "d37cef",
			percentage: -15,
		},
		{
			name:       "lighten by 15%",
			hexColor:   "d37cef",
			percentage: 15,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AdjustColorBrightness(tt.hexColor, tt.percentage)
			if len(result) != 6 {
				t.Errorf("AdjustColorBrightness() returned color with length %d, expected 6", len(result))
			}
		})
	}
}

func TestGetPrimaryProjectPath(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	
	t.Run("non-git directory", func(t *testing.T) {
		result, err := GetPrimaryProjectPath(tmpDir)
		if err != nil {
			t.Errorf("GetPrimaryProjectPath() error = %v", err)
		}
		if result != tmpDir {
			t.Errorf("GetPrimaryProjectPath() = %s, expected %s", result, tmpDir)
		}
	})
}

// setupGitRepo initializes a git repo with an initial commit in a temp dir.
func setupGitRepo(t *testing.T, branchName string) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test User")
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial commit")
	if branchName != "" && branchName != "master" && branchName != "main" {
		runGit(t, dir, "checkout", "-b", branchName)
	}
	return dir
}

// runGit runs a git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestWorkspaceFileExists(t *testing.T) {
	t.Run("file does not exist", func(t *testing.T) {
		tmpDir := setupGitRepo(t, "main")
		exists := WorkspaceFileExists(tmpDir)
		if exists {
			t.Error("WorkspaceFileExists() = true, expected false for non-existent file")
		}
	})
	
	t.Run("file exists", func(t *testing.T) {
		tmpDir := setupGitRepo(t, "main")
		
		// Create workspace file
		err := CreateWorkspaceFile(tmpDir)
		if err != nil {
			t.Errorf("CreateWorkspaceFile() error = %v", err)
		}
		
		// Check if it exists
		exists := WorkspaceFileExists(tmpDir)
		if !exists {
			t.Error("WorkspaceFileExists() = false, expected true after creating file")
		}
	})
}

func TestGetTargetPath(t *testing.T) {
	tests := []struct {
		name      string
		basePath  string
		subFolder string
		expected  string
	}{
		{
			name:      "empty subFolder returns basePath unchanged",
			basePath:  "/base/path",
			subFolder: "",
			expected:  "/base/path",
		},
		{
			name:      "single-segment subFolder returns joined path",
			basePath:  "/base",
			subFolder: "sub",
			expected:  "/base/sub",
		},
		{
			name:      "multi-segment subFolder returns joined path",
			basePath:  "/base",
			subFolder: "a/b/c",
			expected:  "/base/a/b/c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTargetPath(tt.basePath, tt.subFolder)
			if result != tt.expected {
				t.Errorf("GetTargetPath(%q, %q) = %q, expected %q", tt.basePath, tt.subFolder, result, tt.expected)
			}
		})
	}
}

func TestCreateWorkspaceFile(t *testing.T) {
	t.Run("create workspace file", func(t *testing.T) {
		tmpDir := setupGitRepo(t, "DH-12345_some-feature")
		
		err := CreateWorkspaceFile(tmpDir)
		if err != nil {
			t.Errorf("CreateWorkspaceFile() error = %v", err)
		}
		
		// Verify file was created with branch name
		expectedFile := filepath.Join(tmpDir, "DH-12345_some-feature.local.code-workspace")
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Workspace file was not created: %s", expectedFile)
		}
	})
	
	t.Run("file already exists", func(t *testing.T) {
		tmpDir := setupGitRepo(t, "main")
		
		// Create file first time
		err := CreateWorkspaceFile(tmpDir)
		if err != nil {
			t.Errorf("First CreateWorkspaceFile() error = %v", err)
		}
		
		// Try to create again
		err = CreateWorkspaceFile(tmpDir)
		if err == nil {
			t.Error("Expected error when creating duplicate workspace file, got nil")
		}
	})
}
