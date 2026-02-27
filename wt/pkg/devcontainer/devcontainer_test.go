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
