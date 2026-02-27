package devcontainer

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bmingles/wt/pkg/workspace"
)

//go:embed templates/setup.sh templates/setup-bash.sh templates/setup-agents.sh
var templateFiles embed.FS

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

// copyTemplateFiles copies the setup scripts from embedded templates to targetPath/.devcontainer/
// and creates a .gitignore that excludes everything in that directory.
func copyTemplateFiles(targetPath string) error {
	devcontainerDir := filepath.Join(targetPath, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .devcontainer directory: %w", err)
	}

	scripts := []string{"setup.sh", "setup-bash.sh", "setup-agents.sh"}
	for _, script := range scripts {
		data, err := fs.ReadFile(templateFiles, "templates/"+script)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", script, err)
		}
		dst := filepath.Join(devcontainerDir, script)
		if err := os.WriteFile(dst, data, 0755); err != nil {
			return fmt.Errorf("failed to write %s: %w", script, err)
		}
	}

	gitignore := filepath.Join(devcontainerDir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte("*\n"), 0644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}

	return nil
}

// CreateDevcontainer creates a .devcontainer folder in the specified directory.
// This is a scaffold; full implementation will be added in later tasks.
func CreateDevcontainer(path string) error {
	return fmt.Errorf("CreateDevcontainer not yet implemented")
}
