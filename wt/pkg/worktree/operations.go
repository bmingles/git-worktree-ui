package worktree

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateWorktree creates a new Git worktree at the specified path with the given branch name.
// It executes 'git worktree add <worktreePath> -b <branchName>' from the project directory.
// Returns an error if the branch already exists, the path is invalid, or the command fails.
func CreateWorktree(projectPath, branchName, worktreePath string) error {
	if projectPath == "" {
		return errors.New("projectPath cannot be empty")
	}
	if branchName == "" {
		return errors.New("branchName cannot be empty")
	}
	if worktreePath == "" {
		return errors.New("worktreePath cannot be empty")
	}

	// Clean the worktree path
	worktreePath = filepath.Clean(worktreePath)

	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branchName)
	cmd.Dir = projectPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Parse specific error messages
		outputStr := string(output)
		if strings.Contains(outputStr, "already exists") {
			return fmt.Errorf("branch '%s' already exists", branchName)
		}
		if strings.Contains(outputStr, "invalid reference") || strings.Contains(outputStr, "not a valid branch") {
			return fmt.Errorf("invalid branch name: %s", branchName)
		}
		if strings.Contains(outputStr, "cannot create directory") || strings.Contains(outputStr, "Permission denied") {
			return fmt.Errorf("invalid or inaccessible path: %s", worktreePath)
		}
		return fmt.Errorf("failed to create worktree: %w (output: %s)", err, outputStr)
	}

	return nil
}
