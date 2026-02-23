package vscode

import (
	"fmt"
	"os/exec"
)

// OpenInVSCode opens the specified worktree path in VS Code.
// It uses the 'code' command which should be available in PATH.
// Returns an error if VS Code is not installed or the command fails.
func OpenInVSCode(worktreePath string) error {
	// Check if 'code' command is available
	_, err := exec.LookPath("code")
	if err != nil {
		return fmt.Errorf("VS Code 'code' command not found in PATH. Please install VS Code and ensure the 'code' command is available: %w", err)
	}
	
	// Execute the 'code' command to open the worktree path
	cmd := exec.Command("code", worktreePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open VS Code at path %s: %w", worktreePath, err)
	}
	
	return nil
}
