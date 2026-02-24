package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	inputStep          int                            // Step in create worktree flow (0=branch name, 1=path)
	textInput          textinput.Model                // Text input for branch name
	pathInput          textinput.Model                // Text input for optional destination path
	pendingBranchName  string                         // Branch name while collecting path
	inputProject       string                         // Project path for the worktree being created
	confirmMode        bool                           // True when in confirmation mode (deleting worktree)
	confirmWorktree    *worktree.Worktree             // Worktree to be deleted (pending confirmation)
	confirmProject     string                         // Project path for the worktree being deleted
	addProjectMode     bool                           // True when in add project mode
	addProjectStep     int                            // Step in add project flow (0=name, 1=path)
	projectNameInput   textinput.Model                // Text input for project name
	projectPathInput   textinput.Model                // Text input for project path
	pendingProjectName string                         // Project name while collecting path
	pathSuggestions    []string                       // Path autocompletion suggestions
	selectedSuggestion int                            // Index of selected suggestion
	expandedProjects   map[string]bool                // Map of project path to expansion state
	width              int                            // Terminal width
	height             int                            // Terminal height
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
	
	worktreePathInput := textinput.New()
	worktreePathInput.Placeholder = "path/to/worktree"
	worktreePathInput.CharLimit = 200
	worktreePathInput.Width = 70
	
	nameInput := textinput.New()
	nameInput.Placeholder = "My Project"
	nameInput.CharLimit = 100
	nameInput.Width = 50
	
	projectPathInput := textinput.New()
	projectPathInput.Placeholder = "/path/to/project"
	projectPathInput.CharLimit = 200
	projectPathInput.Width = 50
	
	m := Model{
		selectedIndex:    0,
		projects:         projects,
		worktrees:        make(map[string][]worktree.Worktree),
		items:            []Item{},
		inputMode:        false,
		inputStep:        0,
		textInput:        ti,
		pathInput:        worktreePathInput,
		projectNameInput: nameInput,
		projectPathInput: projectPathInput,
		expandedProjects: make(map[string]bool),
		width:            80,  // Default width
		height:           24,  // Default height
	}
	
	// Initialize all projects as collapsed (default state)
	for _, project := range projects {
		m.expandedProjects[project.Path] = false
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
				m.pathSuggestions = nil
				m.selectedSuggestion = 0
				return m, nil
			case "tab":
				// Handle tab completion for path input
				if m.addProjectStep == 1 {
					input := m.projectPathInput.Value()
					
					// Generate suggestions if we don't have any
					if len(m.pathSuggestions) == 0 {
						m.pathSuggestions = getPathSuggestions(input)
						m.selectedSuggestion = 0
						
						// Show first suggestion or complete if only one
						if len(m.pathSuggestions) == 1 {
							// Only one match - auto-complete it
							m.projectPathInput.SetValue(m.pathSuggestions[0])
							m.projectPathInput.SetCursor(len(m.pathSuggestions[0]))
							m.pathSuggestions = nil // Clear after completing
						} else if len(m.pathSuggestions) > 1 {
							// Multiple matches - show first one
							m.projectPathInput.SetValue(m.pathSuggestions[0])
							m.projectPathInput.SetCursor(len(m.pathSuggestions[0]))
						}
					} else if len(m.pathSuggestions) > 1 {
						// Suggestions already showing - cycle to next
						m.selectedSuggestion = (m.selectedSuggestion + 1) % len(m.pathSuggestions)
						m.projectPathInput.SetValue(m.pathSuggestions[m.selectedSuggestion])
						m.projectPathInput.SetCursor(len(m.pathSuggestions[m.selectedSuggestion]))
					}
					return m, nil
				}
			case "shift+tab":
				// Cycle backwards through suggestions
				if m.addProjectStep == 1 && len(m.pathSuggestions) > 1 {
					m.selectedSuggestion--
					if m.selectedSuggestion < 0 {
						m.selectedSuggestion = len(m.pathSuggestions) - 1
					}
					m.projectPathInput.SetValue(m.pathSuggestions[m.selectedSuggestion])
					m.projectPathInput.SetCursor(len(m.pathSuggestions[m.selectedSuggestion]))
					return m, nil
				}
			default:
				// Update the appropriate text input
				var cmd tea.Cmd
				if m.addProjectStep == 0 {
					m.projectNameInput, cmd = m.projectNameInput.Update(msg)
				} else {
					m.projectPathInput, cmd = m.projectPathInput.Update(msg)
					// Clear suggestions when user types (they'll reappear on Tab)
					m.pathSuggestions = nil
					m.selectedSuggestion = 0
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
				if m.inputStep == 0 {
					// Move from branch name to path input
					branchName := m.textInput.Value()
					if branchName != "" {
						m.pendingBranchName = branchName
						m.inputStep = 1
						// Pre-populate with default path (sibling directory)
						projectName := filepath.Base(m.inputProject)
						defaultPath := filepath.Join("..", projectName+".worktrees", branchName)
						m.pathInput.SetValue(defaultPath)
						m.pathInput.Focus()
						m.textInput.Blur()
						return m, nil
					}
					// Empty branch name, cancel
					m.inputMode = false
					m.inputStep = 0
					m.textInput.Reset()
					m.pathInput.Reset()
					m.pendingBranchName = ""
					return m, nil
				} else {
					// Create the worktree with optional path
					branchName := m.pendingBranchName
					customPath := m.pathInput.Value()
					m.inputMode = false
					m.inputStep = 0
					m.textInput.Reset()
					m.pathInput.Reset()
					m.pendingBranchName = ""
					return m, m.createWorktree(m.inputProject, branchName, customPath)
				}
			case "esc", "ctrl+c":
				// Cancel input mode
				m.inputMode = false
				m.inputStep = 0
				m.textInput.Reset()
				m.pathInput.Reset()
				m.pendingBranchName = ""
				return m, nil
			default:
				// Update the appropriate textinput based on step
				var cmd tea.Cmd
				if m.inputStep == 0 {
					m.textInput, cmd = m.textInput.Update(msg)
				} else {
					m.pathInput, cmd = m.pathInput.Update(msg)
				}
				return m, cmd
			}
		}
		var cmd tea.Cmd
		if m.inputStep == 0 {
			m.textInput, cmd = m.textInput.Update(msg)
		} else {
			m.pathInput, cmd = m.pathInput.Update(msg)
		}
		return m, cmd
	}
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
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
		
		// Initialize expansion state for new project (collapsed by default)
		if _, exists := m.expandedProjects[msg.projectPath]; !exists {
			m.expandedProjects[msg.projectPath] = false
		}
		
		// Find and select the newly added project
		for i, item := range m.items {
			if item.Type == ItemTypeProject && item.ProjectPath == msg.projectPath {
				m.selectedIndex = i
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
	case "q", "ctrl+c", "esc":
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
		
	case "enter":
		// Handle Enter key based on item type
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			switch item.Type {
			case ItemTypeProject:
				// Toggle project expansion on Enter
				m.expandedProjects[item.ProjectPath] = !m.expandedProjects[item.ProjectPath]
				m.buildItems()
				return m, nil
			case ItemTypeWorktree:
				// Open worktree in VS Code
				return m, m.openInVSCode()
			}
		}
		return m, nil
		
	case "o":
		// 'o' always opens in VS Code (for both projects and worktrees)
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			return m, m.openInVSCode()
		}
		return m, nil
		
	case " ":
		// Space toggles project expansion
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			if item.Type == ItemTypeProject {
				m.expandedProjects[item.ProjectPath] = !m.expandedProjects[item.ProjectPath]
				m.buildItems()
			}
		}
		return m, nil
		
	case "right", "l":
		// Right arrow expands project
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			if item.Type == ItemTypeProject {
				m.expandedProjects[item.ProjectPath] = true
				m.buildItems()
			}
		}
		return m, nil
		
	case "left", "h":
		// Left arrow collapses project
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			if item.Type == ItemTypeProject {
				m.expandedProjects[item.ProjectPath] = false
				m.buildItems()
			}
		}
		return m, nil
		
	case "n":
		// Trigger add project mode
		m.addProjectMode = true
		m.addProjectStep = 0
		m.projectNameInput.Reset()
		m.projectPathInput.Reset()
		m.projectNameInput.Focus()
		m.pendingProjectName = ""
		m.err = nil
		return m, nil
		
	case "a":
		// Enter create mode
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			m.inputMode = true
			m.inputStep = 0
			m.inputProject = item.ProjectPath
			m.textInput.Reset()
			m.textInput.Focus()
			m.pathInput.Reset()
			m.pathInput.Blur()
			m.pendingBranchName = ""
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
	oldItemsCount := len(m.items)
	m.items = []Item{}
	
	for _, project := range m.projects {
		// Add project header
		m.items = append(m.items, Item{
			Type:        ItemTypeProject,
			ProjectName: project.Name,
			ProjectPath: project.Path,
		})
		
		// Add worktrees for this project only if it's expanded
		if m.expandedProjects[project.Path] {
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

// openInVSCode opens the selected item (project or worktree) in VS Code.
func (m Model) openInVSCode() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.items) {
		return nil
	}
	
	item := m.items[m.selectedIndex]
	
	// Determine the path to open based on item type
	var pathToOpen string
	switch item.Type {
	case ItemTypeProject:
		pathToOpen = item.ProjectPath
	case ItemTypeWorktree:
		if item.Worktree == nil {
			return nil
		}
		pathToOpen = item.Worktree.Path
	default:
		return nil
	}
	
	return func() tea.Msg {
		if err := vscode.OpenInVSCode(pathToOpen); err != nil {
			return vsCodeErrorMsg{err: err}
		}
		return nil
	}
}

// createWorktree creates a new worktree for the given project.
// If customPath is empty, defaults to projectPath/../projectName.worktrees/branchName.
func (m Model) createWorktree(projectPath, branchName, customPath string) tea.Cmd {
	return func() tea.Msg {
		err := worktree.CreateWorktree(projectPath, branchName, customPath)
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
		
		// Check if project with this path already exists
		for _, p := range cfg.Projects {
			existingAbsPath, err := filepath.Abs(p.Path)
			if err == nil && existingAbsPath == absPath {
				return worktreeErrorMsg{err: fmt.Errorf("project path '%s' already exists", absPath)}
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

// getPathSuggestions returns filesystem path suggestions for the given input.
func getPathSuggestions(input string) []string {
	if input == "" {
		input = "."
	}
	
	// Expand ~ to home directory
	if strings.HasPrefix(input, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			input = filepath.Join(home, input[1:])
		}
	}
	
	// Get the directory to search and the prefix to match
	dir := filepath.Dir(input)
	prefix := filepath.Base(input)
	
	// If input ends with /, we're completing within that directory
	if strings.HasSuffix(input, string(filepath.Separator)) {
		dir = input
		prefix = ""
	}
	
	// Read directory entries
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	
	var suggestions []string
	for _, entry := range entries {
		name := entry.Name()
		
		// Skip hidden files unless prefix starts with .
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		
		// Check if entry matches prefix
		if prefix == "" || strings.HasPrefix(name, prefix) {
			fullPath := filepath.Join(dir, name)
			
			// Add trailing slash for directories
			if entry.IsDir() {
				fullPath += string(filepath.Separator)
			}
			
			suggestions = append(suggestions, fullPath)
		}
	}
	
	return suggestions
}

// updatePathSuggestions refreshes the path suggestions based on current input.
func (m *Model) updatePathSuggestions() {
	input := m.projectPathInput.Value()
	m.pathSuggestions = getPathSuggestions(input)
	m.selectedSuggestion = 0
}
