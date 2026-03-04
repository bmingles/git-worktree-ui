package workspace

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// WorkspaceFile represents the structure of a VS Code workspace file.
type WorkspaceFile struct {
	Folders  []WorkspaceFolder       `json:"folders"`
	Settings map[string]interface{}  `json:"settings"`
}

// WorkspaceFolder represents a folder in the workspace.
type WorkspaceFolder struct {
	Path string `json:"path"`
}

// ColorCustomizations represents the workbench color customizations.
type ColorCustomizations struct {
	StatusBarBackground       string `json:"statusBar.background"`
	StatusBarForeground       string `json:"statusBar.foreground"`
	TitleBarActiveBackground  string `json:"titleBar.activeBackground"`
	TitleBarActiveForeground  string `json:"titleBar.activeForeground"`
	TitleBarInactiveBackground string `json:"titleBar.inactiveBackground"`
}

// GenerateColorFromPath generates a hex color code based on the MD5 hash of the given path.
// It returns a 6-character hex color (e.g., "d37cef").
// Uses XOR mixing of hash bytes to ensure uniform color distribution across RGB channels.
func GenerateColorFromPath(path string) string {
	hash := md5.Sum([]byte(path))
	
	// XOR pairs of bytes from different positions to mix entropy
	// This eliminates bias toward any particular color channel
	r := hash[0] ^ hash[8]   // XOR byte 0 with byte 8
	g := hash[5] ^ hash[13]  // XOR byte 5 with byte 13
	b := hash[10] ^ hash[2]  // XOR byte 10 with byte 2
	
	return fmt.Sprintf("%02x%02x%02x", r, g, b)
}

// GetColorForPath returns the appropriate color for a path.
// If customColor is provided and non-empty, it's used.
// Otherwise, generates a color from the primary project path.
func GetColorForPath(targetPath string, customColor string) string {
	if customColor != "" {
		return customColor
	}
	
	// Get primary project path for consistent color generation
	primaryPath, err := GetPrimaryProjectPath(targetPath)
	if err != nil {
		primaryPath = targetPath
	}
	
	return GenerateColorFromPath(primaryPath)
}

// GetContrastingForeground returns either "#000000" or "#ffffff" based on the
// luminance of the given background color to ensure proper contrast.
func GetContrastingForeground(bgColor string) string {
	// Remove # if present
	bgColor = strings.TrimPrefix(bgColor, "#")
	
	// Parse RGB components
	if len(bgColor) != 6 {
		return "#ffffff" // Default to white if invalid
	}
	
	r, err1 := strconv.ParseInt(bgColor[0:2], 16, 64)
	g, err2 := strconv.ParseInt(bgColor[2:4], 16, 64)
	b, err3 := strconv.ParseInt(bgColor[4:6], 16, 64)
	
	if err1 != nil || err2 != nil || err3 != nil {
		return "#ffffff"
	}
	
	// Calculate relative luminance using sRGB colorspace
	// Formula: L = 0.2126 * R + 0.7152 * G + 0.0722 * B
	luminance := (0.2126 * float64(r)) + (0.7152 * float64(g)) + (0.0722 * float64(b))
	
	// Use threshold of 128 (middle of 0-255 range)
	// Light backgrounds get black text, dark backgrounds get white text
	if luminance > 128 {
		return "#000000"
	}
	return "#ffffff"
}

// AdjustColorBrightness adjusts the brightness of a color by a percentage.
// Positive percentage makes it lighter, negative makes it darker.
func AdjustColorBrightness(hexColor string, percentage float64) string {
	hexColor = strings.TrimPrefix(hexColor, "#")
	
	if len(hexColor) != 6 {
		return hexColor
	}
	
	r, _ := strconv.ParseInt(hexColor[0:2], 16, 64)
	g, _ := strconv.ParseInt(hexColor[2:4], 16, 64)
	b, _ := strconv.ParseInt(hexColor[4:6], 16, 64)
	
	factor := 1.0 + (percentage / 100.0)
	
	r = int64(float64(r) * factor)
	g = int64(float64(g) * factor)
	b = int64(float64(b) * factor)
	
	// Clamp values to 0-255
	if r > 255 { r = 255 }
	if r < 0 { r = 0 }
	if g > 255 { g = 255 }
	if g < 0 { g = 0 }
	if b > 255 { b = 255 }
	if b < 0 { b = 0 }
	
	return fmt.Sprintf("%02x%02x%02x", r, g, b)
}

// GetPrimaryProjectPath returns the primary project path (the main repository path).
// If the given path is a worktree, it returns the path of the primary worktree.
// Otherwise, it returns the path itself.
func GetPrimaryProjectPath(path string) (string, error) {
	// Try to get git worktree list
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = path
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Not a git repository or worktree, return the path itself
		return path, nil
	}
	
	// Parse the output to find the primary worktree (first one listed)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			primaryPath := strings.TrimPrefix(line, "worktree ")
			return primaryPath, nil
		}
	}
	
	// If we can't determine, return the original path
	return path, nil
}

// WorkspaceFileExists checks if any .code-workspace file exists in the given path.
// It first checks for the standard .local.code-workspace file, then any other .code-workspace file.
func WorkspaceFileExists(targetPath string) bool {
	// First check for the standard .local.code-workspace file
	primaryPath, err := GetPrimaryProjectPath(targetPath)
	if err != nil {
		primaryPath = targetPath
	}
	
	baseName := filepath.Base(primaryPath)
	workspaceFileName := fmt.Sprintf("%s.local.code-workspace", baseName)
	workspaceFilePath := filepath.Join(targetPath, workspaceFileName)
	
	if _, err := os.Stat(workspaceFilePath); err == nil {
		return true
	}
	
	// Check for any other .code-workspace file
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return false
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".code-workspace") {
			return true
		}
	}
	
	return false
}

// CreateWorkspaceFile creates a .code-workspace file in the specified directory
// with color customizations based on the primary project path.
func CreateWorkspaceFile(targetPath string) error {
	return CreateWorkspaceFileWithColor(targetPath, "")
}

// CreateWorkspaceFileWithColor creates a workspace file with an optional custom color.
// If customColor is empty, generates a color from the primary project path.
func CreateWorkspaceFileWithColor(targetPath string, customColor string) error {
	// Get the primary project path for consistent coloring
	primaryPath, err := GetPrimaryProjectPath(targetPath)
	if err != nil {
		primaryPath = targetPath
	}
	
	// Get the base name for the workspace file
	baseName := filepath.Base(primaryPath)
	workspaceFileName := fmt.Sprintf("%s.local.code-workspace", baseName)
	workspaceFilePath := filepath.Join(targetPath, workspaceFileName)
	
	// Check if file already exists
	if _, err := os.Stat(workspaceFilePath); err == nil {
		return fmt.Errorf("workspace file already exists: %s", workspaceFilePath)
	}
	
	// Get color (use custom if provided, otherwise generate)
	baseColor := GetColorForPath(targetPath, customColor)
	foregroundColor := GetContrastingForeground(baseColor)
	inactiveColor := AdjustColorBrightness(baseColor, -15) // Slightly darker for inactive
	
	// Create workspace structure
	workspace := WorkspaceFile{
		Folders: []WorkspaceFolder{
			{Path: "."},
		},
		Settings: map[string]interface{}{
			"workbench.colorCustomizations": map[string]string{
				"statusBar.background":        "#" + baseColor,
				"statusBar.foreground":        foregroundColor,
				"titleBar.activeBackground":   "#" + baseColor,
				"titleBar.activeForeground":   foregroundColor,
				"titleBar.inactiveBackground": "#" + inactiveColor,
			},
		},
	}
	
	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(workspace, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workspace file: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(workspaceFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workspace file: %w", err)
	}
	
	return nil
}

// IsWorktree checks if the given path is a worktree (not the primary repository).
// Returns true if it's a worktree, false if it's the primary or not a git repo.
func IsWorktree(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	
	gitDir := strings.TrimSpace(string(output))
	// Worktrees have .git as a file (containing path to actual git dir)
	// Primary repo has .git as a directory
	return !strings.HasSuffix(gitDir, ".git")
}

// GetWorkspaceFilePath returns the expected workspace file path for a given directory.
func GetWorkspaceFilePath(targetPath string) (string, error) {
	primaryPath, err := GetPrimaryProjectPath(targetPath)
	if err != nil {
		primaryPath = targetPath
	}
	
	baseName := filepath.Base(primaryPath)
	workspaceFileName := fmt.Sprintf("%s.local.code-workspace", baseName)
	return filepath.Join(targetPath, workspaceFileName), nil
}

// CopyWorkspaceFile copies a workspace file from the primary project to a worktree.
func CopyWorkspaceFile(primaryPath, worktreePath string) error {
	// Get source workspace file from primary
	srcPath, err := GetWorkspaceFilePath(primaryPath)
	if err != nil {
		return fmt.Errorf("failed to get primary workspace path: %w", err)
	}
	
	// Check if source exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("workspace file does not exist in primary: %s", srcPath)
	}
	
	// Get destination workspace file path
	dstPath, err := GetWorkspaceFilePath(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to get worktree workspace path: %w", err)
	}
	
	// Read source file
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read workspace file: %w", err)
	}
	
	// Write to destination
	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workspace file: %w", err)
	}
	
	return nil
}

// CreateOrCopyWorkspaceFile creates or copies a workspace file based on context:
// - For primary branch: creates file if it doesn't exist, skips if it does
// - For worktrees: copies from primary if one exists, otherwise creates new, skips if already exists
func CreateOrCopyWorkspaceFile(targetPath string) error {
	return CreateOrCopyWorkspaceFileWithColor(targetPath, "")
}

// CreateOrCopyWorkspaceFileWithColor creates or copies a workspace file with optional custom color.
func CreateOrCopyWorkspaceFileWithColor(targetPath string, customColor string) error {
	// Check if workspace file already exists
	if WorkspaceFileExists(targetPath) {
		// File exists, skip
		return nil
	}
	
	// Check if this is a worktree
	if IsWorktree(targetPath) {
		// Try to get primary path and copy from it
		primaryPath, err := GetPrimaryProjectPath(targetPath)
		if err == nil && primaryPath != targetPath {
			// Check if primary has a workspace file
			if WorkspaceFileExists(primaryPath) {
				// Copy from primary
				return CopyWorkspaceFile(primaryPath, targetPath)
			}
		}
		// Primary doesn't have a workspace file, create new one
	}
	
	// For primary branch or when primary has no workspace file, create new
	return createWorkspaceFileInternal(targetPath, customColor)
}

// createWorkspaceFileInternal creates a new workspace file (internal use).
func createWorkspaceFileInternal(targetPath string, customColor string) error {
	// Get the primary project path for consistent coloring
	primaryPath, err := GetPrimaryProjectPath(targetPath)
	if err != nil {
		primaryPath = targetPath
	}
	
	// Get the base name for the workspace file
	baseName := filepath.Base(primaryPath)
	workspaceFileName := fmt.Sprintf("%s.local.code-workspace", baseName)
	workspaceFilePath := filepath.Join(targetPath, workspaceFileName)
	
	// Get color (use custom if provided, otherwise generate)
	baseColor := GetColorForPath(targetPath, customColor)
	foregroundColor := GetContrastingForeground(baseColor)
	inactiveColor := AdjustColorBrightness(baseColor, -15) // Slightly darker for inactive
	
	// Create workspace structure
	workspace := WorkspaceFile{
		Folders: []WorkspaceFolder{
			{Path: "."},
		},
		Settings: map[string]interface{}{
			"workbench.colorCustomizations": map[string]string{
				"statusBar.background":        "#" + baseColor,
				"statusBar.foreground":        foregroundColor,
				"titleBar.activeBackground":   "#" + baseColor,
				"titleBar.activeForeground":   foregroundColor,
				"titleBar.inactiveBackground": "#" + inactiveColor,
			},
		},
	}
	
	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(workspace, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workspace file: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(workspaceFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workspace file: %w", err)
	}
	
	return nil
}
