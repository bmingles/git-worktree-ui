package worktree

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// GitStatus represents the git status of a worktree.
type GitStatus struct {
	HasChanges bool // True if there are uncommitted changes
	AheadBy    int  // Number of commits ahead of remote
	BehindBy   int  // Number of commits behind remote
}

// GetStatus returns the git status for a worktree at the given path.
// It executes 'git status --porcelain --branch' and parses the output
// to determine if there are changes and the ahead/behind counts.
func GetStatus(worktreePath string) (GitStatus, error) {
	if worktreePath == "" {
		return GitStatus{}, fmt.Errorf("worktreePath cannot be empty")
	}

	cmd := exec.Command("git", "status", "--porcelain", "--branch")
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return GitStatus{}, fmt.Errorf("failed to get git status: %w (output: %s)", err, string(output))
	}

	return parseGitStatus(string(output))
}

// parseGitStatus parses the output of 'git status --porcelain --branch'.
// The format is:
//   ## branch...remote [ahead N, behind M]
//   M file1.txt
//   ?? file2.txt
//   ...
func parseGitStatus(output string) (GitStatus, error) {
	status := GitStatus{}
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		// Parse branch line to extract ahead/behind counts
		if strings.HasPrefix(line, "## ") {
			status.AheadBy, status.BehindBy = parseBranchLine(line)
			continue
		}

		// Any non-branch line indicates a change
		if line != "" {
			status.HasChanges = true
		}
	}

	if err := scanner.Err(); err != nil {
		return GitStatus{}, fmt.Errorf("error parsing git status: %w", err)
	}

	return status, nil
}

// parseBranchLine extracts ahead/behind counts from a branch status line.
// Format: ## branch...remote [ahead N, behind M]
// Can also be: ## branch...remote [ahead N] or [behind M] or neither
func parseBranchLine(line string) (ahead int, behind int) {
	// Look for [ahead N, behind M] pattern
	aheadRegex := regexp.MustCompile(`ahead (\d+)`)
	behindRegex := regexp.MustCompile(`behind (\d+)`)

	if matches := aheadRegex.FindStringSubmatch(line); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &ahead)
	}

	if matches := behindRegex.FindStringSubmatch(line); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &behind)
	}

	return ahead, behind
}
