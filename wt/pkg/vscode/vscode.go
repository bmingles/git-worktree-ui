package vscode

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmingles/wt/pkg/workspace"
)

// OpenInVSCode opens the specified worktree path in VS Code.
// It first checks for a .local.code-workspace file in the directory.
// If found, it opens the workspace file; otherwise, it opens the directory.
// It uses the 'code' command which should be available in PATH.
// Returns an error if VS Code is not installed or the command fails.
func OpenInVSCode(worktreePath string) error {
	// Check if 'code' command is available
	_, err := exec.LookPath("code")
	if err != nil {
		return fmt.Errorf("VS Code 'code' command not found in PATH. Please install VS Code and ensure the 'code' command is available: %w", err)
	}
	
	// Determine what to open: workspace file or directory
	targetPath, err := resolveTargetPath(worktreePath)
	if err != nil {
		// If we can't resolve, fall back to opening the directory
		targetPath = worktreePath
	}
	
	// Verify the target path exists
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("target path does not exist or is not accessible: %s: %w", targetPath, err)
	}
	
	// Execute the 'code' command to open the target path
	cmd := exec.Command("code", targetPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to open VS Code at path %s: %w (output: %s)", targetPath, err, string(output))
	}
	
	return nil
}

// resolveTargetPath checks for .code-workspace files in the given directory.
// It prefers .local.code-workspace if it exists, otherwise uses any other .code-workspace file.
// If no workspace files exist, it returns the original directory path.
func resolveTargetPath(dirPath string) (string, error) {
	// Read directory contents
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		// Can't read directory, fall back to opening the directory itself
		return dirPath, nil
	}
	
	// First, check if the standard .local.code-workspace file exists
	preferredPath, err := workspace.GetWorkspaceFilePath(dirPath)
	if err == nil {
		if _, err := os.Stat(preferredPath); err == nil {
			// Preferred .local.code-workspace file exists
			return preferredPath, nil
		}
	}
	
	// Collect all .code-workspace files
	var workspaceFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".code-workspace") {
			workspaceFiles = append(workspaceFiles, entry.Name())
		}
	}
	
	// If no workspace files found, return original directory
	if len(workspaceFiles) == 0 {
		return dirPath, nil
	}
	
	// Sort alphabetically for deterministic selection
	sort.Strings(workspaceFiles)
	
	// Return the path to the first workspace file
	workspacePath := filepath.Join(dirPath, workspaceFiles[0])
	return workspacePath, nil
}
