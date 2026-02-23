package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseWorktreeList(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []Worktree
		wantErr bool
	}{
		{
			name: "single worktree",
			input: `worktree /path/to/repo
HEAD abc123def456
branch refs/heads/main

`,
			want: []Worktree{
				{
					Path:      "/path/to/repo",
					Branch:    "main",
					Commit:    "abc123def456",
					IsPrimary: true,
				},
			},
			wantErr: false,
		},
		{
			name: "multiple worktrees",
			input: `worktree /path/to/repo
HEAD abc123def456
branch refs/heads/main

worktree /path/to/repo-feature
HEAD def789ghi012
branch refs/heads/feature-branch

`,
			want: []Worktree{
				{
					Path:      "/path/to/repo",
					Branch:    "main",
					Commit:    "abc123def456",
					IsPrimary: true,
				},
				{
					Path:      "/path/to/repo-feature",
					Branch:    "feature-branch",
					Commit:    "def789ghi012",
					IsPrimary: false,
				},
			},
			wantErr: false,
		},
		{
			name: "detached HEAD",
			input: `worktree /path/to/repo
HEAD abc123def456
branch refs/heads/main

worktree /path/to/repo-detached
HEAD def789ghi012
detached

`,
			want: []Worktree{
				{
					Path:      "/path/to/repo",
					Branch:    "main",
					Commit:    "abc123def456",
					IsPrimary: true,
				},
				{
					Path:      "/path/to/repo-detached",
					Branch:    "",
					Commit:    "def789ghi012",
					IsPrimary: false,
				},
			},
			wantErr: false,
		},
		{
			name: "bare repository",
			input: `worktree /path/to/bare-repo
bare
HEAD abc123def456

`,
			want: []Worktree{
				{
					Path:      "/path/to/bare-repo",
					Branch:    "",
					Commit:    "abc123def456",
					IsPrimary: true,
				},
			},
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			want:    []Worktree{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWorktreeList(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseWorktreeList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("parseWorktreeList() got %d worktrees, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i].Path != tt.want[i].Path {
					t.Errorf("worktree[%d].Path = %v, want %v", i, got[i].Path, tt.want[i].Path)
				}
				if got[i].Branch != tt.want[i].Branch {
					t.Errorf("worktree[%d].Branch = %v, want %v", i, got[i].Branch, tt.want[i].Branch)
				}
				if got[i].Commit != tt.want[i].Commit {
					t.Errorf("worktree[%d].Commit = %v, want %v", i, got[i].Commit, tt.want[i].Commit)
				}
				if got[i].IsPrimary != tt.want[i].IsPrimary {
					t.Errorf("worktree[%d].IsPrimary = %v, want %v", i, got[i].IsPrimary, tt.want[i].IsPrimary)
				}
			}
		})
	}
}

func TestListWorktrees(t *testing.T) {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping integration test")
	}

	// Create a temporary directory for test repository
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Initialize a git repository
	if err := os.Mkdir(repoPath, 0755); err != nil {
		t.Fatalf("failed to create test repo dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init git repo: %v, output: %s", err, output)
	}

	// Configure git user (required for commits)
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Create an initial commit
	readmePath := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to git add: %v, output: %s", err, output)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to git commit: %v, output: %s", err, output)
	}

	// Test listing worktrees (should have at least the main worktree)
	worktrees, err := ListWorktrees(repoPath)
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}

	if len(worktrees) < 1 {
		t.Errorf("ListWorktrees() returned %d worktrees, want at least 1", len(worktrees))
	}

	// Verify the primary worktree
	if len(worktrees) > 0 {
		primary := worktrees[0]
		if primary.Path != repoPath {
			t.Errorf("Primary worktree path = %v, want %v", primary.Path, repoPath)
		}
		if !primary.IsPrimary {
			t.Errorf("First worktree IsPrimary = false, want true")
		}
		if primary.Commit == "" {
			t.Errorf("Primary worktree Commit is empty")
		}
	}
}

func TestListWorktrees_InvalidPath(t *testing.T) {
	// Test with non-existent path
	_, err := ListWorktrees("/nonexistent/path")
	if err == nil {
		t.Error("ListWorktrees() with invalid path should return error")
	}
}

func TestListWorktrees_EmptyPath(t *testing.T) {
	// Test with empty path
	_, err := ListWorktrees("")
	if err == nil {
		t.Error("ListWorktrees() with empty path should return error")
	}
}
