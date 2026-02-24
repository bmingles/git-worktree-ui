package tui

import (
	"fmt"
	"path/filepath"
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

	selectedSuggestionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

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

	// Calculate content width - use min of (terminal width - margins) or max width
	contentWidth := m.width - 8 // Leave margin for terminal edges
	if contentWidth > 120 {
		contentWidth = 120
	}
	if contentWidth < 40 {
		contentWidth = 40
	}
	
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Git Worktree Manager"))
	b.WriteString("\n\n")

	// If in add project mode, show the add project prompt
	if m.addProjectMode {
		b.WriteString(helpStyle.Render("Add Project"))
		b.WriteString("\n\n")
		if m.addProjectStep == 0 {
			b.WriteString(helpStyle.Render("Project name: "))
			b.WriteString(m.projectNameInput.View())
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("Press Enter to continue • Esc to cancel"))
		} else {
			b.WriteString(helpStyle.Render(fmt.Sprintf("Project: %s", m.pendingProjectName)))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("Project path: "))
			b.WriteString(m.projectPathInput.View())
			b.WriteString("\n")
			
			// Show path suggestions in wrapped format
			if len(m.pathSuggestions) > 0 {
				b.WriteString("\n")
				var parts []string
				for i, suggestion := range m.pathSuggestions {
					// Show just the basename for cleaner display
					display := filepath.Base(suggestion)
					if suggestion[len(suggestion)-1] == filepath.Separator {
						display += "/"
					}
					if i == m.selectedSuggestion {
						// White and bold for selected
						parts = append(parts, "\033[1;37m"+display+"\033[0m")
					} else {
						// Gray for unselected
						parts = append(parts, "\033[38;5;244m"+display+"\033[0m")
					}
				}
				b.WriteString(strings.Join(parts, "  "))
				b.WriteString("\n")
			}
			
			b.WriteString("\n")
			b.WriteString(helpStyle.Render("Press Enter to add • Esc to cancel"))
		}
		content := boxStyle.Width(contentWidth).Render(b.String())
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
	}

	// If in input mode, show the create worktree prompt
	if m.inputMode {
		b.WriteString(helpStyle.Render("Create Worktree"))
		b.WriteString("\n\n")
		
		// Show which project this worktree will be added to
		projectName := ""
		for _, proj := range m.projects {
			if proj.Path == m.inputProject {
				projectName = proj.Name
				break
			}
		}
		if projectName != "" {
			b.WriteString(helpStyle.Render(fmt.Sprintf("Project: %s", projectName)))
			b.WriteString("\n\n")
		}
		
		if m.inputStep == 0 {
			b.WriteString(helpStyle.Render("Branch name: "))
			b.WriteString(m.textInput.View())
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("Press Enter to continue • Esc to cancel"))
		} else {
			b.WriteString(helpStyle.Render(fmt.Sprintf("Branch: %s", m.pendingBranchName)))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("Destination path: "))
			b.WriteString(m.pathInput.View())
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("Press Enter to create • Esc to cancel"))
		}
		content := boxStyle.Width(contentWidth).Render(b.String())
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
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
		content := boxStyle.Width(contentWidth).Render(b.String())
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
	}

	// Error display
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// Render items
	b.WriteString(m.renderItems())

	// Help text
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	content := boxStyle.Width(contentWidth).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
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
	
	// Check if this project is expanded by looking at the next item
	isExpanded := false
	for i, it := range m.items {
		if it.Type == ItemTypeProject && it.ProjectPath == item.ProjectPath {
			// Check if next item is a worktree from this project
			if i+1 < len(m.items) && m.items[i+1].Type == ItemTypeWorktree && m.items[i+1].ProjectPath == item.ProjectPath {
				isExpanded = true
			}
			break
		}
	}
	
	// Expansion indicator
	expandIcon := "▸"
	if isExpanded {
		expandIcon = "▾"
	}
	
	// Worktree count (always show for consistency)
	countStr := ""
	if wts, ok := m.worktrees[item.ProjectPath]; ok {
		count := len(wts)
		countStr = statusStyle.Render(fmt.Sprintf(" (%d)", count))
	}

	return style.Render(fmt.Sprintf("%s %s%s", expandIcon, item.ProjectName, countStr))
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

	text := fmt.Sprintf("  %s%s", branchDisplay, statusStr)
	return style.Render(text)
}

// renderHelp renders the help text.
func (m Model) renderHelp() string {
	var help []string
	
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
		item := m.items[m.selectedIndex]
		
		switch item.Type {
		case ItemTypeProject:
			help = []string{
				"[a] add worktree",
				"[n] new project",
				"[esc/q] quit",
			}
		case ItemTypeWorktree:
			if item.Worktree != nil && !item.Worktree.IsPrimary {
				help = []string{
					"[a] add worktree",
					"[d] delete worktree",
					"[n] new project",
					"[esc/q] quit",
				}
			} else {
				help = []string{
					"[a] add worktree",
					"[n] new project",
					"[esc/q] quit",
				}
			}
		}
	} else {
		help = []string{
			"[n] new project",
			"[esc/q] quit",
		}
	}
	
	return helpStyle.Render(strings.Join(help, "\n"))
}
