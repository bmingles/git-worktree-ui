package tui

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/bmingles/wt/pkg/config"
	"github.com/bmingles/wt/pkg/vscode"
	"github.com/bmingles/wt/pkg/worktree"
)

// Model represents the TUI state for worktree navigation.
type Model struct {
	selectedIndex      int                            // Index of currently selected item (across all projects+worktrees)
	projects           []config.Project               // List of projects from config
	worktrees          map[string][]worktree.Worktree // Map of project path to its worktrees
	items              []Item                         // Flattened list of items for navigation
	err                error                          // Error state
	quitting           bool                           // True when user requests quit
	inputMode          bool                           // True when in input mode (creating worktree)
	textInput          textinput.Model                // Text input for branch name
	inputProject       string                         // Project path for the worktree being created
	confirmMode        bool                           // True when in confirmation mode (deleting worktree)
	confirmWorktree    *worktree.Worktree             // Worktree to be deleted (pending confirmation)
	confirmProject     string                         // Project path for the worktree being deleted
	addProjectMode     bool                           // True when in add project mode
	addProjectStep     int                            // Step in add project flow (0=name, 1=path)
	projectNameInput   textinput.Model                // Text input for project name
	projectPathInput   textinput.Model                // Text input for project path
	pendingProjectName string                         // Project name while collecting path
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
	ti := textinput.New()
	ti.Placeholder = "branch-name"
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50
	
	nameInput := textinput.New()
	nameInput.Placeholder = "My Project"
	nameInput.CharLimit = 100
	nameInput.Width = 50
	
	pathInput := textinput.New()
	pathInput.Placeholder = "/path/to/project"
	pathInput.CharLimit = 200
	pathInput.Width = 50
	
	m := Model{
		selectedIndex:    0,
		projects:         projects,
		worktrees:        make(map[string][]worktree.Worktree),
		items:            []Item{},
		inputMode:        false,
		textInput:        ti,
		projectNameInput: nameInput,
		projectPathInput: pathInput,
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
	// If in add project mode, handle project name/path input
	if m.addProjectMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				if m.addProjectStep == 0 {
					// Move from name to path
					name := m.projectNameInput.Value()
					if name != "" {
						m.pendingProjectName = name
						m.addProjectStep = 1
						m.projectPathInput.Focus()
						m.projectNameInput.Blur()
						return m, nil
					}
					// Empty name, cancel
					m.addProjectMode = false
					m.addProjectStep = 0
					m.projectNameInput.Reset()
					m.projectPathInput.Reset()
					m.pendingProjectName = ""
					return m, nil
				} else {
					// Submit the project
					path := m.projectPathInput.Value()
					if path != "" {
						name := m.pendingProjectName
						m.addProjectMode = false
						m.addProjectStep = 0
						m.projectNameInput.Reset()
						m.projectPathInput.Reset()
						m.pendingProjectName = ""
						return m, m.addProject(name, path)
					}
					// Empty path, cancel
					m.addProjectMode = false
					m.addProjectStep = 0
					m.projectNameInput.Reset()
					m.projectPathInput.Reset()
					m.pendingProjectName = ""
					return m, nil
				}
			case "esc", "ctrl+c":
				// Cancel add project mode
				m.addProjectMode = false
				m.addProjectStep = 0
				m.projectNameInput.Reset()
				m.projectPathInput.Reset()
				m.pendingProjectName = ""
				return m, nil
			default:
				// Update the appropriate text input
				var cmd tea.Cmd
				if m.addProjectStep == 0 {
					m.projectNameInput, cmd = m.projectNameInput.Update(msg)
				} else {
					m.projectPathInput, cmd = m.projectPathInput.Update(msg)
				}
				return m, cmd
			}
		}
		return m, nil
	}
	
	// If in confirmation mode, handle y/n input
	if m.confirmMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				// Confirm deletion
				m.confirmMode = false
				worktreePath := m.confirmWorktree.Path
				projectPath := m.confirmProject
				m.confirmWorktree = nil
				m.confirmProject = ""
				return m, m.deleteWorktree(projectPath, worktreePath)
			case "n", "N", "esc", "ctrl+c":
				// Cancel deletion
				m.confirmMode = false
				m.confirmWorktree = nil
				m.confirmProject = ""
				return m, nil
			}
		}
		return m, nil
	}
	
	// If in input mode, handle text input updates
	if m.inputMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Create the worktree
				branchName := m.textInput.Value()
				if branchName != "" {
					m.inputMode = false
					return m, m.createWorktree(m.inputProject, branchName)
				}
				m.inputMode = false
				m.textInput.Reset()
				return m, nil
			case "esc", "ctrl+c":
				// Cancel input mode
				m.inputMode = false
				m.textInput.Reset()
				return m, nil
			default:
				// Update textinput
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case worktreesLoadedMsg:
		m.worktrees[msg.projectPath] = msg.worktrees
		m.buildItems()
		return m, nil
	case worktreeCreatedMsg:
		// Reload worktrees for the project
		return m, m.reloadWorktrees(msg.projectPath)
	case worktreeDeletedMsg:
		// Reload worktrees for the project
		return m, m.reloadWorktrees(msg.projectPath)
	case worktreeErrorMsg:
		m.err = msg.err
		return m, nil
	case vsCodeErrorMsg:
		m.err = msg.err
		return m, nil
	case projectAddedMsg:
		// Reload config to get the new project list
		cfg, err := config.LoadConfig()
		if err != nil {
			m.err = err
			return m, nil
		}
		m.projects = cfg.Projects
		m.buildItems()
		m.loadWorktrees()
		
		// Find and select the newly added project
		for i, item := range m.items {
			if item.Type == ItemTypeProject && item.ProjectPath == msg.projectPath {
				m.selectedIndex = i
				m.buildItems() // Rebuild to expand the selected project
				break
			}
		}
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
			m.buildItems() // Rebuild to update expansion
		}
		
	case "down", "j":
		if m.selectedIndex < len(m.items)-1 {
			m.selectedIndex++
			m.buildItems() // Rebuild to update expansion
		}
		
	case "enter", "o":
		// Handle Enter key based on item type
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			switch item.Type {
			case ItemTypeProject:
				// Project selection handled by buildItems automatically
				return m, nil
			case ItemTypeWorktree:
				// Open worktree in VS Code
				return m, m.openInVSCode()
			}
		}
		return m, nil
		
	case " ":
		// Space does nothing now - expansion is automatic
		return m, nil
		
	case "a":
		// Trigger add project mode
		m.addProjectMode = true
		m.addProjectStep = 0
		m.projectNameInput.Reset()
		m.projectPathInput.Reset()
		m.projectNameInput.Focus()
		m.pendingProjectName = ""
		m.err = nil
		return m, nil
		
	case "c":
		// Enter create mode
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			m.inputMode = true
			m.inputProject = item.ProjectPath
			m.textInput.Reset()
			m.textInput.Focus()
			m.err = nil // Clear any previous errors
		}
		
	case "d":
		// Enter delete confirmation mode
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			if item.Type == ItemTypeWorktree && item.Worktree != nil {
				// Check if it's a primary worktree
				if item.Worktree.IsPrimary {
					m.err = fmt.Errorf("cannot delete primary worktree")
					return m, nil
				}
				
				m.confirmMode = true
				m.confirmWorktree = item.Worktree
				m.confirmProject = item.ProjectPath
				m.err = nil // Clear any previous errors
			}
		}
	}
	
	return m, nil
}

// buildItems creates a flattened list of items for navigation.
func (m *Model) buildItems() {
	// Determine which project should be expanded (the one containing the selected item)
	selectedProjectPath := ""
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
		selectedItem := m.items[m.selectedIndex]
		selectedProjectPath = selectedItem.ProjectPath
	} else if len(m.projects) > 0 {
		// If no selection yet, expand first project
		selectedProjectPath = m.projects[0].Path
	}
	
	oldItemsCount := len(m.items)
	m.items = []Item{}
	
	for _, project := range m.projects {
		// Add project header
		m.items = append(m.items, Item{
			Type:        ItemTypeProject,
			ProjectName: project.Name,
			ProjectPath: project.Path,
		})
		
		// Add worktrees for this project only if it's the selected project
		if project.Path == selectedProjectPath {
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
	
	// Ensure selectedIndex is still valid after rebuild
	if m.selectedIndex >= len(m.items) && len(m.items) > 0 {
		m.selectedIndex = len(m.items) - 1
	}
	
	// If items count changed significantly, might need to adjust selection
	if oldItemsCount > 0 && len(m.items) > 0 && m.selectedIndex >= 0 {
		// Keep selection roughly in the same area
		if m.selectedIndex >= len(m.items) {
			m.selectedIndex = len(m.items) - 1
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

// vsCodeErrorMsg is sent when VS Code fails to open.
type vsCodeErrorMsg struct {
	err error
}

// worktreeCreatedMsg is sent when a worktree is successfully created.
type worktreeCreatedMsg struct {
	projectPath string
}

// worktreeDeletedMsg is sent when a worktree is successfully deleted.
type worktreeDeletedMsg struct {
	projectPath string
}

// worktreeErrorMsg is sent when a worktree operation fails.
type worktreeErrorMsg struct {
	err error
}

// projectAddedMsg is sent when a project is successfully added.
type projectAddedMsg struct {
	projectPath string
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
		if err := vscode.OpenInVSCode(item.Worktree.Path); err != nil {
			return vsCodeErrorMsg{err: err}
		}
		return nil
	}
}

// createWorktree creates a new worktree for the given project.
func (m Model) createWorktree(projectPath, branchName string) tea.Cmd {
	return func() tea.Msg {
		// Calculate worktree path: projectPath/../projectName.worktrees/branchName
		projectName := filepath.Base(projectPath)
		worktreesDir := filepath.Join(filepath.Dir(projectPath), projectName+".worktrees")
		worktreePath := filepath.Join(worktreesDir, branchName)
		
		err := worktree.CreateWorktree(projectPath, branchName, worktreePath)
		if err != nil {
			return worktreeErrorMsg{err: err}
		}
		
		return worktreeCreatedMsg{projectPath: projectPath}
	}
}

// reloadWorktrees reloads the worktrees for a given project.
func (m Model) reloadWorktrees(projectPath string) tea.Cmd {
	return func() tea.Msg {
		wts, err := worktree.ListWorktrees(projectPath)
		if err != nil {
			return worktreeErrorMsg{err: err}
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
		
		return worktreesLoadedMsg{
			projectPath: projectPath,
			worktrees:   wts,
		}
	}
}

// deleteWorktree deletes a worktree.
func (m Model) deleteWorktree(projectPath, worktreePath string) tea.Cmd {
	return func() tea.Msg {
		err := worktree.DeleteWorktree(projectPath, worktreePath)
		if err != nil {
			return worktreeErrorMsg{err: err}
		}
		
		return worktreeDeletedMsg{projectPath: projectPath}
	}
}

// addProject adds a new project to the config.
func (m Model) addProject(name, path string) tea.Cmd {
	return func() tea.Msg {
		// Make path absolute
		absPath, err := filepath.Abs(path)
		if err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("invalid path: %w", err)}
		}
		
		// Check if path exists
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return worktreeErrorMsg{err: fmt.Errorf("path does not exist: %s", absPath)}
		}
		
		// Load existing config
		cfg, err := config.LoadConfig()
		if err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("failed to load config: %w", err)}
		}
		
		// Check if project with this name already exists
		for _, p := range cfg.Projects {
			if p.Name == name {
				return worktreeErrorMsg{err: fmt.Errorf("project '%s' already exists", name)}
			}
		}
		
		// Add new project
		cfg.Projects = append(cfg.Projects, config.Project{
			Name: name,
			Path: absPath,
		})
		
		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}
		
		return projectAddedMsg{projectPath: absPath}
	}
}
