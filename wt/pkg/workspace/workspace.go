package workspace

import (
	"crypto/md5"
	"encoding/hex"
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
func GenerateColorFromPath(path string) string {
	hash := md5.Sum([]byte(path))
	hexStr := hex.EncodeToString(hash[:])
	return hexStr[:6]
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

// WorkspaceFileExists checks if a workspace file already exists for the given path.
func WorkspaceFileExists(targetPath string) bool {
	// Get the primary project path for consistent naming
	primaryPath, err := GetPrimaryProjectPath(targetPath)
	if err != nil {
		primaryPath = targetPath
	}
	
	// Get the base name for the workspace file
	baseName := filepath.Base(primaryPath)
	workspaceFileName := fmt.Sprintf("%s.local.code-workspace", baseName)
	workspaceFilePath := filepath.Join(targetPath, workspaceFileName)
	
	// Check if file exists
	_, err = os.Stat(workspaceFilePath)
	return err == nil
}

// CreateWorkspaceFile creates a .code-workspace file in the specified directory
// with color customizations based on the primary project path.
func CreateWorkspaceFile(targetPath string) error {
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
	
	// Generate color based on primary path
	baseColor := GenerateColorFromPath(primaryPath)
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
