package worktree

import (
	"testing"
)

func TestParseGitStatus_NoChanges(t *testing.T) {
	output := `## main...origin/main
`
	status, err := parseGitStatus(output)
	if err != nil {
		t.Fatalf("parseGitStatus failed: %v", err)
	}

	if status.HasChanges {
		t.Error("Expected HasChanges to be false")
	}
	if status.AheadBy != 0 {
		t.Errorf("Expected AheadBy to be 0, got %d", status.AheadBy)
	}
	if status.BehindBy != 0 {
		t.Errorf("Expected BehindBy to be 0, got %d", status.BehindBy)
	}
}

func TestParseGitStatus_WithChanges(t *testing.T) {
	output := `## main...origin/main
M file1.txt
?? file2.txt
`
	status, err := parseGitStatus(output)
	if err != nil {
		t.Fatalf("parseGitStatus failed: %v", err)
	}

	if !status.HasChanges {
		t.Error("Expected HasChanges to be true")
	}
	if status.AheadBy != 0 {
		t.Errorf("Expected AheadBy to be 0, got %d", status.AheadBy)
	}
	if status.BehindBy != 0 {
		t.Errorf("Expected BehindBy to be 0, got %d", status.BehindBy)
	}
}

func TestParseGitStatus_AheadBehind(t *testing.T) {
	output := `## main...origin/main [ahead 3, behind 2]
`
	status, err := parseGitStatus(output)
	if err != nil {
		t.Fatalf("parseGitStatus failed: %v", err)
	}

	if status.HasChanges {
		t.Error("Expected HasChanges to be false")
	}
	if status.AheadBy != 3 {
		t.Errorf("Expected AheadBy to be 3, got %d", status.AheadBy)
	}
	if status.BehindBy != 2 {
		t.Errorf("Expected BehindBy to be 2, got %d", status.BehindBy)
	}
}

func TestParseGitStatus_AheadOnly(t *testing.T) {
	output := `## main...origin/main [ahead 5]
M file1.txt
`
	status, err := parseGitStatus(output)
	if err != nil {
		t.Fatalf("parseGitStatus failed: %v", err)
	}

	if !status.HasChanges {
		t.Error("Expected HasChanges to be true")
	}
	if status.AheadBy != 5 {
		t.Errorf("Expected AheadBy to be 5, got %d", status.AheadBy)
	}
	if status.BehindBy != 0 {
		t.Errorf("Expected BehindBy to be 0, got %d", status.BehindBy)
	}
}

func TestParseGitStatus_BehindOnly(t *testing.T) {
	output := `## main...origin/main [behind 4]
`
	status, err := parseGitStatus(output)
	if err != nil {
		t.Fatalf("parseGitStatus failed: %v", err)
	}

	if status.HasChanges {
		t.Error("Expected HasChanges to be false")
	}
	if status.AheadBy != 0 {
		t.Errorf("Expected AheadBy to be 0, got %d", status.AheadBy)
	}
	if status.BehindBy != 4 {
		t.Errorf("Expected BehindBy to be 4, got %d", status.BehindBy)
	}
}

func TestParseGitStatus_DetachedHead(t *testing.T) {
	output := `## HEAD (no branch)
M file1.txt
`
	status, err := parseGitStatus(output)
	if err != nil {
		t.Fatalf("parseGitStatus failed: %v", err)
	}

	if !status.HasChanges {
		t.Error("Expected HasChanges to be true")
	}
	if status.AheadBy != 0 {
		t.Errorf("Expected AheadBy to be 0, got %d", status.AheadBy)
	}
	if status.BehindBy != 0 {
		t.Errorf("Expected BehindBy to be 0, got %d", status.BehindBy)
	}
}

func TestParseGitStatus_EmptyOutput(t *testing.T) {
	output := ``
	status, err := parseGitStatus(output)
	if err != nil {
		t.Fatalf("parseGitStatus failed: %v", err)
	}

	if status.HasChanges {
		t.Error("Expected HasChanges to be false")
	}
	if status.AheadBy != 0 {
		t.Errorf("Expected AheadBy to be 0, got %d", status.AheadBy)
	}
	if status.BehindBy != 0 {
		t.Errorf("Expected BehindBy to be 0, got %d", status.BehindBy)
	}
}

func TestParseBranchLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		expAhead  int
		expBehind int
	}{
		{
			name:   "ahead and behind",
			line:   "## main...origin/main [ahead 3, behind 2]",
			expAhead:  3,
			expBehind: 2,
		},
		{
			name:   "ahead only",
			line:   "## main...origin/main [ahead 5]",
			expAhead:  5,
			expBehind: 0,
		},
		{
			name:   "behind only",
			line:   "## main...origin/main [behind 4]",
			expAhead:  0,
			expBehind: 4,
		},
		{
			name:   "no tracking",
			line:   "## main...origin/main",
			expAhead:  0,
			expBehind: 0,
		},
		{
			name:   "detached head",
			line:   "## HEAD (no branch)",
			expAhead:  0,
			expBehind: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ahead, behind := parseBranchLine(tt.line)
			if ahead != tt.expAhead {
				t.Errorf("Expected ahead=%d, got %d", tt.expAhead, ahead)
			}
			if behind != tt.expBehind {
				t.Errorf("Expected behind=%d, got %d", tt.expBehind, behind)
			}
		})
	}
}

func TestGetStatus_EmptyPath(t *testing.T) {
	_, err := GetStatus("")
	if err == nil {
		t.Error("Expected error for empty path")
	}
	if err != nil && err.Error() != "worktreePath cannot be empty" {
		t.Errorf("Expected 'worktreePath cannot be empty', got: %v", err)
	}
}

// TestGetStatus verifies GetStatus works with a real git repository
// This test requires running in a git repository
func TestGetStatus(t *testing.T) {
	// This test will only work in a git repository
	// We'll use "." as the path to test the current directory
	status, err := GetStatus(".")
	if err != nil {
		// If we're not in a git repo, skip this test
		t.Skipf("Not in a git repository: %v", err)
	}

	// Just verify we got a status back with reasonable values
	// We can't assert specific values since they depend on the current state
	if status.AheadBy < 0 {
		t.Errorf("AheadBy should not be negative: %d", status.AheadBy)
	}
	if status.BehindBy < 0 {
		t.Errorf("BehindBy should not be negative: %d", status.BehindBy)
	}
}
