package devcontainer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHasDevcontainer(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("no .devcontainer folder", func(t *testing.T) {
		result := HasDevcontainer(tmpDir)
		if result {
			t.Error("HasDevcontainer() = true, expected false when .devcontainer does not exist")
		}
	})

	t.Run("with .devcontainer folder", func(t *testing.T) {
		tmpDir2 := t.TempDir()
		devcontainerPath := filepath.Join(tmpDir2, ".devcontainer")
		if err := os.Mkdir(devcontainerPath, 0755); err != nil {
			t.Fatalf("failed to create .devcontainer: %v", err)
		}

		result := HasDevcontainer(tmpDir2)
		if !result {
			t.Error("HasDevcontainer() = false, expected true when .devcontainer exists")
		}
	})

	t.Run(".devcontainer is a file not a dir", func(t *testing.T) {
		tmpDir3 := t.TempDir()
		devcontainerFile := filepath.Join(tmpDir3, ".devcontainer")
		if err := os.WriteFile(devcontainerFile, []byte(""), 0644); err != nil {
			t.Fatalf("failed to create .devcontainer file: %v", err)
		}

		result := HasDevcontainer(tmpDir3)
		if result {
			t.Error("HasDevcontainer() = true, expected false when .devcontainer is a file")
		}
	})
}

func TestCopyTemplateFiles(t *testing.T) {
	tmpDir := t.TempDir()

	if err := copyTemplateFiles(tmpDir); err != nil {
		t.Fatalf("copyTemplateFiles() error = %v", err)
	}

	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")

	scripts := []string{"setup.sh", "setup-bash.sh", "setup-agents.sh"}
	for _, script := range scripts {
		path := filepath.Join(devcontainerDir, script)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected script %s to exist: %v", script, err)
			continue
		}
		if info.Mode().Perm()&0755 != 0755 {
			t.Errorf("script %s has permissions %v, expected 0755", script, info.Mode().Perm())
		}
	}

	gitignore := filepath.Join(devcontainerDir, ".gitignore")
	data, err := os.ReadFile(gitignore)
	if err != nil {
		t.Fatalf("expected .gitignore to exist: %v", err)
	}
	if string(data) != "*\n" {
		t.Errorf(".gitignore content = %q, expected %q", string(data), "*\n")
	}
}

func TestGetPrimaryDevcontainerPath(t *testing.T) {
	t.Run("non-git directory without .devcontainer", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := GetPrimaryDevcontainerPath(tmpDir)
		if err == nil {
			t.Error("GetPrimaryDevcontainerPath() expected error when .devcontainer does not exist, got nil")
		}
	})

	t.Run("non-git directory with .devcontainer", func(t *testing.T) {
		tmpDir := t.TempDir()
		devcontainerPath := filepath.Join(tmpDir, ".devcontainer")
		if err := os.Mkdir(devcontainerPath, 0755); err != nil {
			t.Fatalf("failed to create .devcontainer: %v", err)
		}

		result, err := GetPrimaryDevcontainerPath(tmpDir)
		if err != nil {
			t.Errorf("GetPrimaryDevcontainerPath() error = %v", err)
		}
		if result != devcontainerPath {
			t.Errorf("GetPrimaryDevcontainerPath() = %s, expected %s", result, devcontainerPath)
		}
	})
}

// setupGitRepo initializes a git repo with an initial commit in a temp dir.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test User")
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial commit")
	return dir
}

// addWorktree creates a git worktree of primaryDir and returns its path.
func addWorktree(t *testing.T, primaryDir string) string {
	t.Helper()
	baseDir := t.TempDir()
	worktreeDir := filepath.Join(baseDir, "worktree")
	runGit(t, primaryDir, "worktree", "add", worktreeDir, "-b", "test-branch")
	t.Cleanup(func() {
		exec.Command("git", "-C", primaryDir, "worktree", "remove", "--force", worktreeDir).Run() //nolint
	})
	return worktreeDir
}

// runGit runs a git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestCreateDevcontainer(t *testing.T) {
	t.Run("idempotent - returns nil if .devcontainer already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
		if err := os.Mkdir(devcontainerDir, 0755); err != nil {
			t.Fatalf("failed to create .devcontainer: %v", err)
		}
		sentinelPath := filepath.Join(devcontainerDir, "sentinel.txt")
		if err := os.WriteFile(sentinelPath, []byte("original"), 0644); err != nil {
			t.Fatalf("failed to write sentinel: %v", err)
		}

		if err := CreateDevcontainer(tmpDir); err != nil {
			t.Errorf("CreateDevcontainer() error = %v, expected nil", err)
		}

		data, err := os.ReadFile(sentinelPath)
		if err != nil {
			t.Errorf("expected sentinel file to still exist: %v", err)
		}
		if string(data) != "original" {
			t.Errorf("sentinel file was modified, got %q", string(data))
		}
	})

	t.Run("non-git path creates new devcontainer with template files", func(t *testing.T) {
		tmpDir := t.TempDir()

		if err := CreateDevcontainer(tmpDir); err != nil {
			t.Fatalf("CreateDevcontainer() error = %v", err)
		}

		if !HasDevcontainer(tmpDir) {
			t.Error("expected .devcontainer to be created")
		}

		devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
		for _, script := range []string{"setup.sh", "setup-bash.sh", "setup-agents.sh"} {
			if _, err := os.Stat(filepath.Join(devcontainerDir, script)); err != nil {
				t.Errorf("expected %s to exist: %v", script, err)
			}
		}
		if _, err := os.Stat(filepath.Join(devcontainerDir, "devcontainer.json")); err != nil {
			t.Errorf("expected devcontainer.json to exist: %v", err)
		}
	})

	t.Run("worktree with primary devcontainer - copies from primary", func(t *testing.T) {
		primaryDir := setupGitRepo(t)

		// Create .devcontainer in primary with a sentinel file
		primaryDevDir := filepath.Join(primaryDir, ".devcontainer")
		if err := os.MkdirAll(primaryDevDir, 0755); err != nil {
			t.Fatalf("failed to create primary .devcontainer: %v", err)
		}
		sentinelContent := []byte("from-primary")
		if err := os.WriteFile(filepath.Join(primaryDevDir, "sentinel.txt"), sentinelContent, 0644); err != nil {
			t.Fatalf("failed to write sentinel: %v", err)
		}

		worktreeDir := addWorktree(t, primaryDir)

		if err := CreateDevcontainer(worktreeDir); err != nil {
			t.Fatalf("CreateDevcontainer() error = %v", err)
		}

		if !HasDevcontainer(worktreeDir) {
			t.Error("expected .devcontainer to be created in worktree")
		}

		copiedSentinel := filepath.Join(worktreeDir, ".devcontainer", "sentinel.txt")
		data, err := os.ReadFile(copiedSentinel)
		if err != nil {
			t.Errorf("expected sentinel file to be copied: %v", err)
		}
		if string(data) != string(sentinelContent) {
			t.Errorf("copied sentinel content = %q, expected %q", string(data), string(sentinelContent))
		}
	})

	t.Run("worktree without primary devcontainer - creates new", func(t *testing.T) {
		primaryDir := setupGitRepo(t)
		worktreeDir := addWorktree(t, primaryDir)

		if err := CreateDevcontainer(worktreeDir); err != nil {
			t.Fatalf("CreateDevcontainer() error = %v", err)
		}

		if !HasDevcontainer(worktreeDir) {
			t.Error("expected .devcontainer to be created in worktree")
		}

		devcontainerDir := filepath.Join(worktreeDir, ".devcontainer")
		for _, script := range []string{"setup.sh", "setup-bash.sh", "setup-agents.sh"} {
			if _, err := os.Stat(filepath.Join(devcontainerDir, script)); err != nil {
				t.Errorf("expected %s to exist: %v", script, err)
			}
		}
		if _, err := os.Stat(filepath.Join(devcontainerDir, "devcontainer.json")); err != nil {
			t.Errorf("expected devcontainer.json to exist: %v", err)
		}
	})

	t.Run("primary git branch - creates new devcontainer", func(t *testing.T) {
		primaryDir := setupGitRepo(t)

		if err := CreateDevcontainer(primaryDir); err != nil {
			t.Fatalf("CreateDevcontainer() error = %v", err)
		}

		if !HasDevcontainer(primaryDir) {
			t.Error("expected .devcontainer to be created")
		}

		devcontainerDir := filepath.Join(primaryDir, ".devcontainer")
		if _, err := os.Stat(filepath.Join(devcontainerDir, "devcontainer.json")); err != nil {
			t.Errorf("expected devcontainer.json to exist: %v", err)
		}
	})
}
