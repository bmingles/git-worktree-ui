package devcontainer

import (
	"os"
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
