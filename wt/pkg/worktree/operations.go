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
// If worktreePath is empty, defaults to projectPath/../projectName.worktrees/branchName.
// Relative paths are resolved relative to the project directory.
// Returns an error if the branch already exists, the path is invalid, or the command fails.
func CreateWorktree(projectPath, branchName, worktreePath string) error {
	if projectPath == "" {
		return errors.New("projectPath cannot be empty")
	}
	if branchName == "" {
		return errors.New("branchName cannot be empty")
	}

	// If worktreePath is empty, use default convention
	if worktreePath == "" {
		projectName := filepath.Base(projectPath)
		worktreesDir := filepath.Join(filepath.Dir(projectPath), projectName+".worktrees")
		worktreePath = filepath.Join(worktreesDir, branchName)
	} else {
		// If worktreePath is relative, resolve it relative to project directory
		if !filepath.IsAbs(worktreePath) {
			worktreePath = filepath.Join(projectPath, worktreePath)
		}
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

// DeleteWorktree removes a Git worktree at the specified path.
// It executes 'git worktree remove <worktreePath>' from the project directory.
// Returns an error if the worktree has uncommitted changes, doesn't exist, or is a primary worktree.
func DeleteWorktree(projectPath, worktreePath string) error {
	if projectPath == "" {
		return errors.New("projectPath cannot be empty")
	}
	if worktreePath == "" {
		return errors.New("worktreePath cannot be empty")
	}

	// Clean the worktree path
	worktreePath = filepath.Clean(worktreePath)

	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = projectPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Parse specific error messages
		outputStr := string(output)
		if strings.Contains(outputStr, "contains modified or untracked files") {
			return fmt.Errorf("worktree has uncommitted changes")
		}
		if strings.Contains(outputStr, "not a working tree") || strings.Contains(outputStr, "does not exist") {
			return fmt.Errorf("worktree does not exist: %s", worktreePath)
		}
		if strings.Contains(outputStr, "is main working tree") || strings.Contains(outputStr, "cannot remove") {
			return fmt.Errorf("cannot delete primary worktree")
		}
		return fmt.Errorf("failed to delete worktree: %w (output: %s)", err, outputStr)
	}

	return nil
}
