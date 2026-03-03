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

	// Build mounts section with comments
	mountsSection := ""
	if isWorktree {
		primaryName := filepath.Base(primaryPath)
		mountsSection = fmt.Sprintf(`  "mounts": [
    // Bind-mount the primary .git directory since we are in a worktree
    "source=${localWorkspaceFolder}/../../%s/.git,target=${localWorkspaceFolder}/../../%s/.git,type=bind,consistency=cached",

    // Isolated Claude settings per worktree
    "source=claude-code-config-${localWorkspaceFolderBasename},target=/home/vscode/.claude,type=volume",

    // Bind-mount .claude.json so login persists across rebuilds
    "source=${localEnv:HOME}/.claude.json,target=/home/vscode/.claude.json,type=bind,consistency=cached",

    // Bind-mount skills from host into project directory for easier debugging of skills.
    // Can remove the 'readonly' setting if you want to edit them from the container
    "source=${localEnv:HOME}/.agents/skills,target=${containerWorkspaceFolder}/.claude/skills,type=bind,consistency=cached,readonly"
  ],`, primaryName, primaryName)
	} else {
		mountsSection = `  "mounts": [
    // Isolated Claude settings per worktree
    "source=claude-code-config-${localWorkspaceFolderBasename},target=/home/vscode/.claude,type=volume",

    // Bind-mount .claude.json so login persists across rebuilds
    "source=${localEnv:HOME}/.claude.json,target=/home/vscode/.claude.json,type=bind,consistency=cached",

    // Bind-mount skills from host into project directory for easier debugging of skills.
    // Can remove the 'readonly' setting if you want to edit them from the container
    "source=${localEnv:HOME}/.agents/skills,target=${containerWorkspaceFolder}/.claude/skills,type=bind,consistency=cached,readonly"
  ],`
	}

	// Build the complete devcontainer.json content with comments
	content := fmt.Sprintf(`// For format details, see https://aka.ms/devcontainer.json. For config options, see the
// README at: https://github.com/devcontainers/templates/tree/main/src/ubuntu
{
  "name": "Ubuntu",
  // Or use a Dockerfile or Docker Compose file. More info: https://containers.dev/guide/dockerfile
  "image": "mcr.microsoft.com/devcontainers/base:noble",
  "features": {
    "ghcr.io/devcontainers/features/node:1": {
      "nodeGypDependencies": true,
      "version": "latest",
      "pnpmVersion": "none",
      "nvmVersion": "latest"
    }
  },
%s
  "postCreateCommand": ".devcontainer/setup.sh",
  "customizations": {
    "vscode": {
      "extensions": [
        "anthropic.claude-code",
        "dbaeumer.vscode-eslint",
        "esbenp.prettier-vscode"
      ],

      "settings": {
        // "Yolo" permissions for Claude
        "claudeCode.allowDangerouslySkipPermissions": true,
        "claudeCode.initialPermissionMode": "bypassPermissions",

        // Copilot settings
        "chat.agent.maxRequests": 50,
        "chat.tools.terminal.autoApprove": {
          "/.*/": true
        },
        "chat.tools.terminal.ignoreDefaultAutoApproveRules": true,

        "editor.defaultFormatter": "esbenp.prettier-vscode",
        "editor.formatOnSave": true,
        "terminal.integrated.defaultProfile.linux": "bash",
        "workbench.colorCustomizations": {
          "statusBar.background": "#%s",
          "statusBar.foreground": "%s",
          "titleBar.activeBackground": "#%s",
          "titleBar.activeForeground": "%s",
          "titleBar.inactiveBackground": "#%s",
		  "chat.requestBorder": "#e21010",
		  "statusBarItem.remoteBackground": "#e21010"
        }
      }
    }
  }
}
`, mountsSection, baseColor, foregroundColor, baseColor, foregroundColor, inactiveColor)

	devcontainerPath := filepath.Join(targetPath, ".devcontainer", "devcontainer.json")
	if err := os.WriteFile(devcontainerPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write devcontainer.json: %w", err)
	}

	return nil
}

// copyDevcontainerFolder copies all files from srcDir into targetPath/.devcontainer/.
func copyDevcontainerFolder(srcDir, targetPath string) error {
	dstDir := filepath.Join(targetPath, ".devcontainer")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create .devcontainer directory: %w", err)
	}

	return filepath.Walk(srcDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path: %w", err)
		}

		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		if err := os.WriteFile(dstPath, data, info.Mode()); err != nil {
			return fmt.Errorf("failed to write %s: %w", dstPath, err)
		}

		return nil
	})
}

// CreateDevcontainer creates a .devcontainer folder in the specified directory.
// It is idempotent: returns nil if .devcontainer already exists.
// For worktrees whose primary project has a .devcontainer, the folder is copied from primary,
// but the devcontainer.json is regenerated to include the .git bind-mount.
// Otherwise, template files and a generated devcontainer.json are created.
func CreateDevcontainer(path string) error {
	// Idempotent: skip if .devcontainer already exists
	if HasDevcontainer(path) {
		return nil
	}

	// For worktrees, try to copy from primary if primary has .devcontainer
	if workspace.IsWorktree(path) {
		primaryPath, err := workspace.GetPrimaryProjectPath(path)
		if err != nil {
			return fmt.Errorf("failed to get primary project path: %w", err)
		}

		if HasDevcontainer(primaryPath) {
			primaryDevcontainerPath := filepath.Join(primaryPath, ".devcontainer")
			if err := copyDevcontainerFolder(primaryDevcontainerPath, path); err != nil {
				return fmt.Errorf("failed to copy .devcontainer from primary project: %w", err)
			}
			// Regenerate devcontainer.json to add the .git bind-mount for worktree
			if err := generateDevcontainerJSON(path); err != nil {
				return fmt.Errorf("failed to generate devcontainer.json for worktree: %w", err)
			}
			return nil
		}
	}

	// Create new .devcontainer with template files and generated devcontainer.json
	if err := copyTemplateFiles(path); err != nil {
		return fmt.Errorf("failed to copy template files: %w", err)
	}
	if err := generateDevcontainerJSON(path); err != nil {
		return fmt.Errorf("failed to generate devcontainer.json: %w", err)
	}
	return nil
}
