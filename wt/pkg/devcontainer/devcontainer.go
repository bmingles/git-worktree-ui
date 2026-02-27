package devcontainer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmingles/wt/pkg/workspace"
)

// HasDevcontainer checks if a .devcontainer folder exists in the given path.
func HasDevcontainer(path string) bool {
	devcontainerPath := filepath.Join(path, ".devcontainer")
	info, err := os.Stat(devcontainerPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetPrimaryDevcontainerPath returns the .devcontainer path in the primary project.
// If the given path is a worktree, it returns the .devcontainer path of the primary worktree.
func GetPrimaryDevcontainerPath(path string) (string, error) {
	primaryPath, err := workspace.GetPrimaryProjectPath(path)
	if err != nil {
		return "", fmt.Errorf("failed to get primary project path: %w", err)
	}

	devcontainerPath := filepath.Join(primaryPath, ".devcontainer")
	if _, err := os.Stat(devcontainerPath); os.IsNotExist(err) {
		return "", fmt.Errorf(".devcontainer folder does not exist in primary project: %s", devcontainerPath)
	}

	return devcontainerPath, nil
}

// CreateDevcontainer creates a .devcontainer folder in the specified directory.
// This is a scaffold; full implementation will be added in later tasks.
func CreateDevcontainer(path string) error {
	return fmt.Errorf("CreateDevcontainer not yet implemented")
}
