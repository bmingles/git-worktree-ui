package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bmingles/wt/pkg/config"
	"github.com/bmingles/wt/pkg/worktree"
)

// Model represents the TUI state for worktree navigation.
type Model struct {
	selectedIndex int                           // Index of currently selected item (across all projects+worktrees)
	projects      []config.Project              // List of projects from config
	worktrees     map[string][]worktree.Worktree // Map of project path to its worktrees
	items         []Item                        // Flattened list of items for navigation
	err           error                         // Error state
	quitting      bool                          // True when user requests quit
}

// ItemType represents the type of item in the navigation list.
type ItemType int

const (
	ItemTypeProject ItemType = iota
	ItemTypeWorktree
)

// Item represents a navigable item in the TUI (either a project header or worktree).
type Item struct {
	Type        ItemType
	ProjectName string
	ProjectPath string
	Worktree    *worktree.Worktree // nil for project items
}

// NewModel creates a new TUI model with the given projects.
func NewModel(projects []config.Project) Model {
	m := Model{
		selectedIndex: 0,
		projects:      projects,
		worktrees:     make(map[string][]worktree.Worktree),
		items:         []Item{},
	}
	
	// Build initial items list and load worktrees
	m.buildItems()
	m.loadWorktrees()
	
	return m
}

// Init initializes the model (bubbletea.Model interface).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model (bubbletea.Model interface).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case worktreesLoadedMsg:
		m.worktrees[msg.projectPath] = msg.worktrees
		m.buildItems()
		return m, nil
	}
	
	return m, nil
}

// handleKeyPress processes keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
		
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
		
	case "down", "j":
		if m.selectedIndex < len(m.items)-1 {
			m.selectedIndex++
		}
		
	case "enter", "o":
		return m, m.openInVSCode()
		
	case "c":
		// TODO: Create worktree
		
	case "d":
		// TODO: Delete worktree
	}
	
	return m, nil
}

// buildItems creates a flattened list of items for navigation.
func (m *Model) buildItems() {
	m.items = []Item{}
	
	for _, project := range m.projects {
		// Add project header
		m.items = append(m.items, Item{
			Type:        ItemTypeProject,
			ProjectName: project.Name,
			ProjectPath: project.Path,
		})
		
		// Add worktrees for this project
		if wts, ok := m.worktrees[project.Path]; ok {
			for i := range wts {
				m.items = append(m.items, Item{
					Type:        ItemTypeWorktree,
					ProjectName: project.Name,
					ProjectPath: project.Path,
					Worktree:    &wts[i],
				})
			}
		}
	}
}

// loadWorktrees starts loading worktrees for all projects.
func (m *Model) loadWorktrees() {
	for _, project := range m.projects {
		wts, err := worktree.ListWorktrees(project.Path)
		if err != nil {
			m.err = err
			continue
		}
		
		// Load status for each worktree
		for i := range wts {
			status, err := worktree.GetStatus(wts[i].Path)
			if err != nil {
				// Continue with empty status on error
				continue
			}
			wts[i].Status = status
		}
		
		m.worktrees[project.Path] = wts
	}
	m.buildItems()
}

// worktreesLoadedMsg is sent when worktrees are loaded for a project.
type worktreesLoadedMsg struct {
	projectPath string
	worktrees   []worktree.Worktree
}

// openInVSCode opens the selected worktree in VS Code.
func (m Model) openInVSCode() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.items) {
		return nil
	}
	
	item := m.items[m.selectedIndex]
	if item.Type != ItemTypeWorktree || item.Worktree == nil {
		return nil
	}
	
	return func() tea.Msg {
		// This would be implemented to actually execute the VS Code command
		// For now, we just return a message
		fmt.Printf("Opening %s in VS Code\n", item.Worktree.Path)
		return nil
	}
}
