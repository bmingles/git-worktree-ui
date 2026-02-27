package devcontainer

import (
	"embed"
	"encoding/json"
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

// generateDevcontainerJSON creates a devcontainer.json file in targetPath/.devcontainer/.
// For worktrees, a bind-mount for the primary project's .git directory is added.
// For primary branches, no .git bind mount is added.
// Color customizations are generated consistently with workspace files.
func generateDevcontainerJSON(targetPath string) error {
	// Determine if this path is a worktree
	isWorktree := workspace.IsWorktree(targetPath)

	// Get primary project path for consistent color generation
	primaryPath, err := workspace.GetPrimaryProjectPath(targetPath)
	if err != nil {
		primaryPath = targetPath
	}

	// Generate colors matching workspace file convention
	baseColor := workspace.GenerateColorFromPath(primaryPath)
	foregroundColor := workspace.GetContrastingForeground(baseColor)
	inactiveColor := workspace.AdjustColorBrightness(baseColor, -15)

	// Build mounts list
	mounts := []string{}

	if isWorktree {
		primaryName := filepath.Base(primaryPath)
		gitMount := fmt.Sprintf(
			"source=${localWorkspaceFolder}/../../%s/.git,target=${localWorkspaceFolder}/../../%s/.git,type=bind,consistency=cached",
			primaryName, primaryName,
		)
		mounts = append(mounts, gitMount)
	}

	mounts = append(mounts,
		"source=claude-code-config-${devcontainerId},target=/home/vscode/.claude,type=volume",
		"source=${localEnv:HOME}/.claude.json,target=/home/vscode/.claude.json,type=bind,consistency=cached",
		"source=${localEnv:HOME}/.agents/skills,target=${containerWorkspaceFolder}/.claude/skills,type=bind,consistency=cached,readonly",
	)

	// Build devcontainer structure
	devcontainer := map[string]interface{}{
		"name":  "Ubuntu",
		"image": "mcr.microsoft.com/devcontainers/base:noble",
		"features": map[string]interface{}{
			"ghcr.io/devcontainers/features/node:1": map[string]interface{}{
				"nodeGypDependencies": true,
				"version":             "latest",
				"pnpmVersion":         "none",
				"nvmVersion":          "latest",
			},
		},
		"mounts":             mounts,
		"postCreateCommand":  ".devcontainer/setup.sh",
		"customizations": map[string]interface{}{
			"vscode": map[string]interface{}{
				"extensions": []string{
					"anthropic.claude-code",
					"dbaeumer.vscode-eslint",
					"esbenp.prettier-vscode",
				},
				"settings": map[string]interface{}{
					"claudeCode.allowDangerouslySkipPermissions": true,
					"claudeCode.initialPermissionMode":           "bypassPermissions",
					"chat.agent.maxRequests":                     50,
					"chat.tools.terminal.autoApprove": map[string]interface{}{
						"/.*/": true,
					},
					"chat.tools.terminal.ignoreDefaultAutoApproveRules": true,
					"editor.defaultFormatter":                            "esbenp.prettier-vscode",
					"editor.formatOnSave":                                true,
					"terminal.integrated.defaultProfile.linux":           "bash",
					"workbench.colorCustomizations": map[string]string{
						"statusBar.background":        "#" + baseColor,
						"statusBar.foreground":        foregroundColor,
						"titleBar.activeBackground":   "#" + baseColor,
						"titleBar.activeForeground":   foregroundColor,
						"titleBar.inactiveBackground": "#" + inactiveColor,
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(devcontainer, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal devcontainer.json: %w", err)
	}

	devcontainerPath := filepath.Join(targetPath, ".devcontainer", "devcontainer.json")
	if err := os.WriteFile(devcontainerPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write devcontainer.json: %w", err)
	}

	return nil
}

// CreateDevcontainer creates a .devcontainer folder in the specified directory
// with setup scripts and a generated devcontainer.json.
func CreateDevcontainer(path string) error {
	if err := copyTemplateFiles(path); err != nil {
		return fmt.Errorf("failed to copy template files: %w", err)
	}
	if err := generateDevcontainerJSON(path); err != nil {
		return fmt.Errorf("failed to generate devcontainer.json: %w", err)
	}
	return nil
}
