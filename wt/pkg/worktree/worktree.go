package worktree

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Worktree represents a Git worktree with its associated metadata.
type Worktree struct {
	Path      string
	Branch    string
	Commit    string
	IsPrimary bool
}

// ListWorktrees returns all worktrees for a given project path.
// It executes 'git worktree list --porcelain' and parses the output.
func ListWorktrees(projectPath string) ([]Worktree, error) {
	if projectPath == "" {
		return nil, errors.New("projectPath cannot be empty")
	}

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = projectPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w (output: %s)", err, string(output))
	}

	return parseWorktreeList(string(output))
}

// parseWorktreeList parses the output of 'git worktree list --porcelain'.
// The format is:
//   worktree <path>
//   HEAD <commit>
//   branch <branch>
//   [bare]
//   
//   worktree <path>
//   HEAD <commit>
//   detached
func parseWorktreeList(output string) ([]Worktree, error) {
	var worktrees []Worktree
	var current *Worktree
	firstWorktree := true

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// Empty line indicates end of worktree entry
		if line == "" {
			if current != nil {
				// First worktree in the list is the primary one
				if firstWorktree {
					current.IsPrimary = true
					firstWorktree = false
				}
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}

		fields := strings.SplitN(line, " ", 2)
		if len(fields) < 2 {
			// Handle single-word lines like "bare" or "detached"
			if len(fields) == 1 {
				if fields[0] == "bare" && current != nil {
					// Bare repository marker
					continue
				}
				if fields[0] == "detached" && current != nil {
					// Detached HEAD - branch will remain empty
					continue
				}
			}
			continue
		}

		key := fields[0]
		value := fields[1]

		switch key {
		case "worktree":
			current = &Worktree{
				Path: value,
			}
		case "HEAD":
			if current != nil {
				current.Commit = value
			}
		case "branch":
			if current != nil {
				// Branch is in format "refs/heads/<branch>"
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		}
	}

	// Handle last worktree if there's no trailing newline
	if current != nil {
		if firstWorktree {
			current.IsPrimary = true
		}
		worktrees = append(worktrees, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error parsing worktree list: %w", err)
	}

	return worktrees, nil
}
