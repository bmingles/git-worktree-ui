package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			PaddingLeft(2).
			PaddingBottom(1)

	projectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")).
			PaddingLeft(2)

	selectedProjectStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#04B575")).
				Background(lipgloss.Color("#3C3C3C")).
				PaddingLeft(2)

	worktreeStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			Foreground(lipgloss.Color("#FFFFFF"))

	selectedWorktreeStyle = lipgloss.NewStyle().
				PaddingLeft(4).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#3C3C3C"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	changesStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true)

	aheadStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	behindStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			PaddingTop(1).
			PaddingLeft(2)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true).
			PaddingLeft(2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 2)
)

// View renders the model (bubbletea.Model interface).
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Git Worktree Manager"))
	b.WriteString("\n\n")

	// If in input mode, show the create worktree prompt
	if m.inputMode {
		b.WriteString(helpStyle.Render("Create Worktree"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Branch name:"))
		b.WriteString("\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Press Enter to create • Esc to cancel"))
		return boxStyle.Render(b.String())
	}

	// If in confirmation mode, show the delete confirmation prompt
	if m.confirmMode {
		b.WriteString(errorStyle.Render("Delete Worktree"))
		b.WriteString("\n\n")
		if m.confirmWorktree != nil {
			b.WriteString(helpStyle.Render(fmt.Sprintf("Path: %s", m.confirmWorktree.Path)))
			b.WriteString("\n")
			b.WriteString(helpStyle.Render(fmt.Sprintf("Branch: %s", m.confirmWorktree.Branch)))
			b.WriteString("\n\n")
		}
		b.WriteString(helpStyle.Render("Are you sure? (y/n)"))
		return boxStyle.Render(b.String())
	}

	// Error display
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// Render items
	if len(m.items) == 0 {
		b.WriteString(helpStyle.Render("No projects configured. Add projects to ~/.config/wt/config.yaml"))
	} else {
		b.WriteString(m.renderItems())
	}

	// Help text
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return boxStyle.Render(b.String())
}

// renderItems renders the list of projects and worktrees.
func (m Model) renderItems() string {
	var b strings.Builder

	for i, item := range m.items {
		isSelected := i == m.selectedIndex

		switch item.Type {
		case ItemTypeProject:
			b.WriteString(m.renderProject(item, isSelected))
		case ItemTypeWorktree:
			b.WriteString(m.renderWorktree(item, isSelected))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderProject renders a project header.
func (m Model) renderProject(item Item, isSelected bool) string {
	style := projectStyle
	if isSelected {
		style = selectedProjectStyle
	}

	cursor := "  "
	if isSelected {
		cursor = "▶ "
	}

	return style.Render(fmt.Sprintf("%s%s", cursor, item.ProjectName))
}

// renderWorktree renders a worktree item with status indicators.
func (m Model) renderWorktree(item Item, isSelected bool) string {
	if item.Worktree == nil {
		return ""
	}

	style := worktreeStyle
	if isSelected {
		style = selectedWorktreeStyle
	}

	cursor := "  "
	if isSelected {
		cursor = "▶ "
	}

	wt := item.Worktree

	// Build status indicators
	var indicators []string

	// Primary worktree indicator
	if wt.IsPrimary {
		indicators = append(indicators, "●")
	}

	// Uncommitted changes indicator
	if wt.Status.HasChanges {
		indicators = append(indicators, changesStyle.Render("*"))
	}

	// Ahead indicator
	if wt.Status.AheadBy > 0 {
		indicators = append(indicators, aheadStyle.Render(fmt.Sprintf("↑%d", wt.Status.AheadBy)))
	}

	// Behind indicator
	if wt.Status.BehindBy > 0 {
		indicators = append(indicators, behindStyle.Render(fmt.Sprintf("↓%d", wt.Status.BehindBy)))
	}

	// Format branch name
	branchDisplay := wt.Branch
	if branchDisplay == "" {
		branchDisplay = fmt.Sprintf("(detached @ %.7s)", wt.Commit)
	}

	// Combine everything
	statusStr := ""
	if len(indicators) > 0 {
		statusStr = " " + statusStyle.Render(strings.Join(indicators, " "))
	}

	text := fmt.Sprintf("%s%s%s", cursor, branchDisplay, statusStr)
	return style.Render(text)
}

// renderHelp renders the help text.
func (m Model) renderHelp() string {
	helps := []string{
		"↑/↓: navigate",
		"enter/o: open in VS Code",
		"c: create worktree",
		"d: delete worktree",
		"q: quit",
	}
	return helpStyle.Render(strings.Join(helps, " • "))
}
