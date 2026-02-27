package devcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmingles/wt/pkg/workspace"
)

// setupGitRepoE2E initializes a git repo with an initial commit in dir.
func setupGitRepoE2E(t *testing.T, dir string) {
	t.Helper()
	runGitIn(t, dir, "init")
	runGitIn(t, dir, "config", "user.email", "test@test.com")
	runGitIn(t, dir, "config", "user.name", "Test User")
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-m", "initial commit")
}

// addWorktreeE2E creates a git worktree from primaryDir in worktreeDir.
func addWorktreeE2E(t *testing.T, primaryDir, worktreeDir string) {
	t.Helper()
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}
	runGitIn(t, primaryDir, "worktree", "add", worktreeDir, "-b", "e2e-branch")
	t.Cleanup(func() {
		exec.Command("git", "-C", primaryDir, "worktree", "remove", "--force", worktreeDir).Run() //nolint
	})
}

func runGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// assertFileExists checks the file exists.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist: %s — %v", path, err)
	}
}

// readJSONFile reads and unmarshals a JSON file into a map.
func readJSONFile(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", path, err)
	}
	return result
}

// getColorCustomizations extracts workbench.colorCustomizations from devcontainer.json.
func getColorCustomizations(t *testing.T, devcontainerJSONPath string) map[string]interface{} {
	t.Helper()
	doc := readJSONFile(t, devcontainerJSONPath)

	customizations, ok := doc["customizations"].(map[string]interface{})
	if !ok {
		t.Fatalf("customizations not found or wrong type in %s", devcontainerJSONPath)
	}
	vscode, ok := customizations["vscode"].(map[string]interface{})
	if !ok {
		t.Fatalf("customizations.vscode not found in %s", devcontainerJSONPath)
	}
	settings, ok := vscode["settings"].(map[string]interface{})
	if !ok {
		t.Fatalf("customizations.vscode.settings not found in %s", devcontainerJSONPath)
	}
	colors, ok := settings["workbench.colorCustomizations"].(map[string]interface{})
	if !ok {
		t.Fatalf("workbench.colorCustomizations not found in %s", devcontainerJSONPath)
	}
	return colors
}

// getMounts extracts the mounts array from devcontainer.json.
func getMounts(t *testing.T, devcontainerJSONPath string) []string {
	t.Helper()
	doc := readJSONFile(t, devcontainerJSONPath)
	rawMounts, ok := doc["mounts"]
	if !ok {
		return nil
	}
	mountsList, ok := rawMounts.([]interface{})
	if !ok {
		t.Fatalf("mounts field is not an array in %s", devcontainerJSONPath)
	}
	mounts := make([]string, 0, len(mountsList))
	for _, m := range mountsList {
		s, ok := m.(string)
		if !ok {
			t.Fatalf("mount entry is not a string: %v", m)
		}
		mounts = append(mounts, s)
	}
	return mounts
}

// TestE2E_PrimaryBranch verifies that CreateDevcontainer on a primary branch:
// - Creates .devcontainer with all required template files
// - Generates devcontainer.json without a .git bind-mount
// - Colors match those generated from the primary path
func TestE2E_PrimaryBranch(t *testing.T) {
	primaryDir := t.TempDir()
	setupGitRepoE2E(t, primaryDir)

	// Scenario 1: Create devcontainer on primary branch
	if err := CreateDevcontainer(primaryDir); err != nil {
		t.Fatalf("CreateDevcontainer() error = %v", err)
	}

	// Verify .devcontainer folder exists
	devcontainerDir := filepath.Join(primaryDir, ".devcontainer")
	if !HasDevcontainer(primaryDir) {
		t.Fatal("expected .devcontainer to be created in primary branch")
	}

	// Verify template files
	for _, f := range []string{"setup.sh", "setup-bash.sh", "setup-agents.sh", ".gitignore", "devcontainer.json"} {
		assertFileExists(t, filepath.Join(devcontainerDir, f))
	}

	// Verify devcontainer.json has no .git bind-mount
	devcontainerJSONPath := filepath.Join(devcontainerDir, "devcontainer.json")
	mounts := getMounts(t, devcontainerJSONPath)
	for _, m := range mounts {
		if strings.Contains(m, ".git,") && strings.Contains(m, "type=bind") {
			t.Errorf("primary branch devcontainer.json should NOT have .git bind-mount, found: %s", m)
		}
	}

	// Verify colors match workspace color generation from primary path
	primaryPath, _ := workspace.GetPrimaryProjectPath(primaryDir)
	expectedBase := workspace.GenerateColorFromPath(primaryPath)
	expectedFg := workspace.GetContrastingForeground(expectedBase)
	expectedInactive := workspace.AdjustColorBrightness(expectedBase, -15)

	colors := getColorCustomizations(t, devcontainerJSONPath)
	assertEqual(t, "statusBar.background", "#"+expectedBase, fmt.Sprintf("%v", colors["statusBar.background"]))
	assertEqual(t, "statusBar.foreground", expectedFg, fmt.Sprintf("%v", colors["statusBar.foreground"]))
	assertEqual(t, "titleBar.activeBackground", "#"+expectedBase, fmt.Sprintf("%v", colors["titleBar.activeBackground"]))
	assertEqual(t, "titleBar.activeForeground", expectedFg, fmt.Sprintf("%v", colors["titleBar.activeForeground"]))
	assertEqual(t, "titleBar.inactiveBackground", "#"+expectedInactive, fmt.Sprintf("%v", colors["titleBar.inactiveBackground"]))

	t.Log("PASS: Primary branch - .devcontainer created without .git bind-mount, colors correct")
}

// TestE2E_Idempotency verifies that calling CreateDevcontainer twice doesn't change anything.
func TestE2E_Idempotency(t *testing.T) {
	primaryDir := t.TempDir()
	setupGitRepoE2E(t, primaryDir)

	// First call
	if err := CreateDevcontainer(primaryDir); err != nil {
		t.Fatalf("first CreateDevcontainer() error = %v", err)
	}

	// Write a sentinel file to verify it's not overwritten
	sentinelPath := filepath.Join(primaryDir, ".devcontainer", "sentinel.txt")
	if err := os.WriteFile(sentinelPath, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to write sentinel: %v", err)
	}

	// Second call - should be idempotent
	if err := CreateDevcontainer(primaryDir); err != nil {
		t.Fatalf("second CreateDevcontainer() error = %v", err)
	}

	data, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatalf("sentinel file should still exist: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("idempotency violated: sentinel content changed to %q", string(data))
	}

	t.Log("PASS: Idempotency verified - repeated calls don't overwrite .devcontainer")
}

// TestE2E_WorktreeWithPrimaryDevcontainer verifies that when a worktree's primary
// already has a .devcontainer, it is copied to the worktree.
func TestE2E_WorktreeWithPrimaryDevcontainer(t *testing.T) {
	primaryDir := t.TempDir()
	setupGitRepoE2E(t, primaryDir)

	// Create .devcontainer in primary with custom content
	primaryDevDir := filepath.Join(primaryDir, ".devcontainer")
	if err := os.MkdirAll(primaryDevDir, 0755); err != nil {
		t.Fatalf("failed to create primary .devcontainer: %v", err)
	}
	customContent := []byte(`{"name":"custom-from-primary"}`)
	if err := os.WriteFile(filepath.Join(primaryDevDir, "devcontainer.json"), customContent, 0644); err != nil {
		t.Fatalf("failed to write primary devcontainer.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(primaryDevDir, "sentinel.txt"), []byte("from-primary"), 0644); err != nil {
		t.Fatalf("failed to write primary sentinel: %v", err)
	}

	// Create worktree
	worktreeDir := t.TempDir()
	addWorktreeE2E(t, primaryDir, worktreeDir)

	// Run CreateDevcontainer on worktree
	if err := CreateDevcontainer(worktreeDir); err != nil {
		t.Fatalf("CreateDevcontainer() on worktree error = %v", err)
	}

	if !HasDevcontainer(worktreeDir) {
		t.Fatal("expected .devcontainer to be created in worktree")
	}

	// Verify sentinel was copied from primary
	sentinelPath := filepath.Join(worktreeDir, ".devcontainer", "sentinel.txt")
	data, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatalf("expected sentinel file to be copied: %v", err)
	}
	if string(data) != "from-primary" {
		t.Errorf("copied sentinel = %q, expected %q", string(data), "from-primary")
	}

	// Verify devcontainer.json content was copied from primary
	copiedJSON := filepath.Join(worktreeDir, ".devcontainer", "devcontainer.json")
	copiedData, err := os.ReadFile(copiedJSON)
	if err != nil {
		t.Fatalf("expected devcontainer.json to be copied: %v", err)
	}
	if string(copiedData) != string(customContent) {
		t.Errorf("copied devcontainer.json content = %q, expected %q", string(copiedData), string(customContent))
	}

	t.Log("PASS: Worktree with primary .devcontainer - folder copied from primary")
}

// TestE2E_WorktreeWithoutPrimaryDevcontainer verifies that a worktree without
// primary .devcontainer creates new folder with correct .git bind-mount.
func TestE2E_WorktreeWithoutPrimaryDevcontainer(t *testing.T) {
	primaryDir := t.TempDir()
	setupGitRepoE2E(t, primaryDir)

	// No .devcontainer in primary
	worktreeDir := t.TempDir()
	addWorktreeE2E(t, primaryDir, worktreeDir)

	if err := CreateDevcontainer(worktreeDir); err != nil {
		t.Fatalf("CreateDevcontainer() on worktree error = %v", err)
	}

	if !HasDevcontainer(worktreeDir) {
		t.Fatal("expected .devcontainer to be created in worktree")
	}

	devcontainerDir := filepath.Join(worktreeDir, ".devcontainer")
	devcontainerJSONPath := filepath.Join(devcontainerDir, "devcontainer.json")

	// Verify template files exist
	for _, f := range []string{"setup.sh", "setup-bash.sh", "setup-agents.sh", ".gitignore", "devcontainer.json"} {
		assertFileExists(t, filepath.Join(devcontainerDir, f))
	}

	// Verify .git bind-mount is present with correct primary path
	primaryPath, err := workspace.GetPrimaryProjectPath(worktreeDir)
	if err != nil {
		t.Fatalf("failed to get primary path: %v", err)
	}
	primaryName := filepath.Base(primaryPath)
	expectedMount := fmt.Sprintf(
		"source=${localWorkspaceFolder}/../../%s/.git,target=${localWorkspaceFolder}/../../%s/.git,type=bind,consistency=cached",
		primaryName, primaryName,
	)

	mounts := getMounts(t, devcontainerJSONPath)
	foundGitMount := false
	for _, m := range mounts {
		if m == expectedMount {
			foundGitMount = true
			break
		}
	}
	if !foundGitMount {
		t.Errorf("expected .git bind-mount not found in mounts.\nExpected: %s\nGot: %v", expectedMount, mounts)
	}

	t.Log("PASS: Worktree without primary .devcontainer - created with correct .git bind-mount")
}

// TestE2E_ColorMatchesWorkspaceFile verifies that the colors in devcontainer.json
// match what a workspace file would have for the same project.
func TestE2E_ColorMatchesWorkspaceFile(t *testing.T) {
	primaryDir := t.TempDir()
	setupGitRepoE2E(t, primaryDir)

	// Create devcontainer
	if err := CreateDevcontainer(primaryDir); err != nil {
		t.Fatalf("CreateDevcontainer() error = %v", err)
	}

	// Create workspace file
	if err := workspace.CreateWorkspaceFile(primaryDir); err != nil {
		t.Fatalf("CreateWorkspaceFile() error = %v", err)
	}

	// Read workspace file colors
	primaryPath, _ := workspace.GetPrimaryProjectPath(primaryDir)
	baseName := filepath.Base(primaryPath)
	workspaceFilePath := filepath.Join(primaryDir, baseName+".local.code-workspace")
	workspaceDoc := readJSONFile(t, workspaceFilePath)

	settings, ok := workspaceDoc["settings"].(map[string]interface{})
	if !ok {
		t.Fatal("settings not found in workspace file")
	}
	wsColors, ok := settings["workbench.colorCustomizations"].(map[string]interface{})
	if !ok {
		t.Fatal("workbench.colorCustomizations not found in workspace file")
	}

	// Read devcontainer.json colors
	devcontainerJSONPath := filepath.Join(primaryDir, ".devcontainer", "devcontainer.json")
	dcColors := getColorCustomizations(t, devcontainerJSONPath)

	// Compare colors
	colorKeys := []string{
		"statusBar.background",
		"statusBar.foreground",
		"titleBar.activeBackground",
		"titleBar.activeForeground",
		"titleBar.inactiveBackground",
	}
	for _, key := range colorKeys {
		wsVal := fmt.Sprintf("%v", wsColors[key])
		dcVal := fmt.Sprintf("%v", dcColors[key])
		if wsVal != dcVal {
			t.Errorf("color mismatch for %s: workspace=%s, devcontainer=%s", key, wsVal, dcVal)
		}
	}

	t.Log("PASS: Colors in devcontainer.json match workspace file colors")
}

// TestE2E_WorktreeColorMatchesWorkspaceFile verifies color consistency for worktrees.
func TestE2E_WorktreeColorMatchesWorkspaceFile(t *testing.T) {
	primaryDir := t.TempDir()
	setupGitRepoE2E(t, primaryDir)

	worktreeDir := t.TempDir()
	addWorktreeE2E(t, primaryDir, worktreeDir)

	// Create devcontainer on worktree (no primary .devcontainer, so creates new)
	if err := CreateDevcontainer(worktreeDir); err != nil {
		t.Fatalf("CreateDevcontainer() error = %v", err)
	}

	// Create workspace file on worktree for comparison
	if err := workspace.CreateWorkspaceFile(worktreeDir); err != nil {
		t.Fatalf("CreateWorkspaceFile() error = %v", err)
	}

	// Read workspace file colors
	primaryPath, _ := workspace.GetPrimaryProjectPath(worktreeDir)
	baseName := filepath.Base(primaryPath)
	workspaceFilePath := filepath.Join(worktreeDir, baseName+".local.code-workspace")
	workspaceDoc := readJSONFile(t, workspaceFilePath)

	settings, ok := workspaceDoc["settings"].(map[string]interface{})
	if !ok {
		t.Fatal("settings not found in workspace file")
	}
	wsColors, ok := settings["workbench.colorCustomizations"].(map[string]interface{})
	if !ok {
		t.Fatal("workbench.colorCustomizations not found in workspace file")
	}

	// Read devcontainer.json colors
	devcontainerJSONPath := filepath.Join(worktreeDir, ".devcontainer", "devcontainer.json")
	dcColors := getColorCustomizations(t, devcontainerJSONPath)

	// Compare colors
	colorKeys := []string{
		"statusBar.background",
		"statusBar.foreground",
		"titleBar.activeBackground",
		"titleBar.activeForeground",
		"titleBar.inactiveBackground",
	}
	for _, key := range colorKeys {
		wsVal := fmt.Sprintf("%v", wsColors[key])
		dcVal := fmt.Sprintf("%v", dcColors[key])
		if wsVal != dcVal {
			t.Errorf("color mismatch for %s: workspace=%s, devcontainer=%s", key, wsVal, dcVal)
		}
	}

	t.Log("PASS: Worktree colors in devcontainer.json match workspace file colors")
}

// TestE2E_UICommandVisibility verifies HasDevcontainer returns correct values
// (this is what the TUI uses to show/hide the [i] command).
func TestE2E_UICommandVisibility(t *testing.T) {
	tmpDir := t.TempDir()

	// Before creation: [i] command should be shown (HasDevcontainer = false)
	if HasDevcontainer(tmpDir) {
		t.Error("HasDevcontainer() = true before creation, expected false ([i] command should be shown)")
	}

	// Create devcontainer
	if err := CreateDevcontainer(tmpDir); err != nil {
		t.Fatalf("CreateDevcontainer() error = %v", err)
	}

	// After creation: [i] command should be hidden (HasDevcontainer = true)
	if !HasDevcontainer(tmpDir) {
		t.Error("HasDevcontainer() = false after creation, expected true ([i] command should be hidden)")
	}

	t.Log("PASS: UI command visibility - [i] shown before creation, hidden after")
}

func assertEqual(t *testing.T, field, expected, actual string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %q, got %q", field, expected, actual)
	}
}
