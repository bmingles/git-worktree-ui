package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/bmingles/wt/pkg/config"
	"github.com/bmingles/wt/pkg/devcontainer"
	"github.com/bmingles/wt/pkg/vscode"
	"github.com/bmingles/wt/pkg/workspace"
	"github.com/bmingles/wt/pkg/worktree"
)

// Model represents the TUI state for worktree navigation.
type Model struct {
	selectedIndex      int                            // Index of currently selected item (across all projects+worktrees)
	projects           []config.Project               // List of projects from config
	categories         []string                       // List of categories from config
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
	expandedCategories map[string]bool                // Map of category name to expansion state
	width              int                            // Terminal width
	height             int                            // Terminal height
	searchMode         bool                           // True when actively typing in search box
	filterActive       bool                           // True when filter is applied (not in search input mode)
	searchInput        textinput.Model                // Text input for search/filter
	filterTerm         string                         // Current filter term
	categoryInputMode  bool                           // True when in category input mode
	categoryInput      textinput.Model                // Text input for category name
	categoryProject    string                         // Project path for category assignment
	tagInputMode       bool                           // True when in tag input mode
	tagInput           textinput.Model                // Text input for tag names
	tagProject         string                         // Project path for tag assignment
}

// ItemType represents the type of item in the navigation list.
type ItemType int

const (
	ItemTypeCategory ItemType = iota
	ItemTypeProject
	ItemTypeWorktree
)

// Item represents a navigable item in the TUI (category, project header, or worktree).
type Item struct {
	Type        ItemType
	Category    string                 // Category name (for category items)
	ProjectName string
	ProjectPath string
	ProjectTags []string               // Tags for the project
	Worktree    *worktree.Worktree     // nil for category and project items
}

// NewModel creates a new TUI model with the given projects and categories.
func NewModel(projects []config.Project, categories []string) Model {
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
	
	searchInput := textinput.New()
	searchInput.Placeholder = "Search projects, worktrees, tags, or categories..."
	searchInput.CharLimit = 100
	searchInput.Width = 70
	
	categoryInput := textinput.New()
	categoryInput.Placeholder = "Enter category name"
	categoryInput.CharLimit = 100
	categoryInput.Width = 50
	
	tagInput := textinput.New()
	tagInput.Placeholder = "Enter tags (comma-separated)"
	tagInput.CharLimit = 200
	tagInput.Width = 50
	
	m := Model{
		selectedIndex:      0,
		projects:           projects,
		categories:         categories,
		worktrees:          make(map[string][]worktree.Worktree),
		items:              []Item{},
		inputMode:          false,
		inputStep:          0,
		textInput:          ti,
		pathInput:          worktreePathInput,
		projectNameInput:   nameInput,
		projectPathInput:   projectPathInput,
		searchInput:        searchInput,
		categoryInput:      categoryInput,
		tagInput:           tagInput,
		expandedProjects:   make(map[string]bool),
		expandedCategories: make(map[string]bool),
		width:              80,  // Default width
		height:             24,  // Default height
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
	// If in search mode, handle search input
	if m.searchMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Exit search input mode but keep filter applied
				if m.searchInput.Value() != "" {
					m.searchMode = false
					m.filterActive = true
					m.filterTerm = m.searchInput.Value()
					m.searchInput.Blur()
					m.buildItems()
					return m, nil
				}
				// Empty search, exit search mode and clear filter
				m.searchMode = false
				m.filterActive = false
				m.filterTerm = ""
				m.searchInput.Reset()
				m.searchInput.Blur()
				m.buildItems()
				return m, nil
			case "esc", "ctrl+c":
				// Cancel search mode and clear filter
				m.searchMode = false
				m.filterActive = false
				m.filterTerm = ""
				m.searchInput.Reset()
				m.searchInput.Blur()
				m.buildItems()
				return m, nil
			default:
				// Update search input and apply filter in real-time
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.filterTerm = m.searchInput.Value()
				m.buildItems()
				return m, cmd
			}
		default:
			// Update search input for non-key messages
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}
	}
	
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
						m.selectedSuggestion = -1 // -1 means no selection yet
						
						// If only one suggestion, auto-complete it
						if len(m.pathSuggestions) == 1 {
							// Only one match - auto-complete it
							m.projectPathInput.SetValue(m.pathSuggestions[0])
							m.projectPathInput.SetCursor(len(m.pathSuggestions[0]))
							m.pathSuggestions = nil // Clear after completing
						}
						// If multiple suggestions, just show them (don't select yet)
					} else if len(m.pathSuggestions) > 1 {
						// Suggestions already showing - cycle to next
						if m.selectedSuggestion == -1 {
							m.selectedSuggestion = 0 // Start at first item
						} else {
							m.selectedSuggestion = (m.selectedSuggestion + 1) % len(m.pathSuggestions)
						}
						m.projectPathInput.SetValue(m.pathSuggestions[m.selectedSuggestion])
						m.projectPathInput.SetCursor(len(m.pathSuggestions[m.selectedSuggestion]))
					}
					return m, nil
				}
			case "shift+tab":
				// Cycle backwards through suggestions
				if m.addProjectStep == 1 && len(m.pathSuggestions) > 1 {
					if m.selectedSuggestion == -1 {
						// Start at last item if not yet cycling
						m.selectedSuggestion = len(m.pathSuggestions) - 1
					} else {
						m.selectedSuggestion--
						if m.selectedSuggestion < 0 {
							m.selectedSuggestion = len(m.pathSuggestions) - 1
						}
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
	
	// If in category input mode, handle category input
	if m.categoryInputMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Submit the category
				categoryName := m.categoryInput.Value()
				if categoryName != "" {
					projectPath := m.categoryProject
					m.categoryInputMode = false
					m.categoryInput.Reset()
					m.categoryInput.Blur()
					m.categoryProject = ""
					return m, m.assignCategory(projectPath, categoryName)
				}
				// Empty category name, cancel
				m.categoryInputMode = false
				m.categoryInput.Reset()
				m.categoryInput.Blur()
				m.categoryProject = ""
				return m, nil
			case "esc", "ctrl+c":
				// Cancel category input mode
				m.categoryInputMode = false
				m.categoryInput.Reset()
				m.categoryInput.Blur()
				m.categoryProject = ""
				return m, nil
			default:
				// Update category input
				var cmd tea.Cmd
				m.categoryInput, cmd = m.categoryInput.Update(msg)
				return m, cmd
			}
		}
		var cmd tea.Cmd
		m.categoryInput, cmd = m.categoryInput.Update(msg)
		return m, cmd
	}

	// If in tag input mode, handle tag input
	if m.tagInputMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Submit the tags
				tagInput := m.tagInput.Value()
				if tagInput != "" {
					projectPath := m.tagProject
					m.tagInputMode = false
					m.tagInput.Reset()
					m.tagInput.Blur()
					m.tagProject = ""
					return m, m.assignTags(projectPath, tagInput)
				}
				// Empty tag input, cancel
				m.tagInputMode = false
				m.tagInput.Reset()
				m.tagInput.Blur()
				m.tagProject = ""
				return m, nil
			case "esc", "ctrl+c":
				// Cancel tag input mode
				m.tagInputMode = false
				m.tagInput.Reset()
				m.tagInput.Blur()
				m.tagProject = ""
				return m, nil
			default:
				// Update tag input
				var cmd tea.Cmd
				m.tagInput, cmd = m.tagInput.Update(msg)
				return m, cmd
			}
		}
		var cmd tea.Cmd
		m.tagInput, cmd = m.tagInput.Update(msg)
		return m, cmd
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
		// Reload config to get the new project list and categories
		cfg, err := config.LoadConfig()
		if err != nil {
			m.err = err
			return m, nil
		}
		m.projects = cfg.Projects
		m.categories = cfg.Categories
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
	case categoryAssignedMsg:
		// Reload config to get the updated project categories and categories list
		cfg, err := config.LoadConfig()
		if err != nil {
			m.err = err
			return m, nil
		}
		m.projects = cfg.Projects
		m.categories = cfg.Categories
		m.buildItems()
		
		// Keep the current project selected
		for i, item := range m.items {
			if item.Type == ItemTypeProject && item.ProjectPath == msg.projectPath {
				m.selectedIndex = i
				break
			}
		}
		return m, nil
	case tagsAssignedMsg:
		// Reload config to get the updated project tags
		cfg, err := config.LoadConfig()
		if err != nil {
			m.err = err
			return m, nil
		}
		m.projects = cfg.Projects
		m.buildItems()
		
		// Keep the current project selected
		for i, item := range m.items {
			if item.Type == ItemTypeProject && item.ProjectPath == msg.projectPath {
				m.selectedIndex = i
				break
			}
		}
		return m, nil
	case workspaceCreatedMsg:
		// Workspace file created successfully, clear any previous errors
		m.err = nil
		return m, nil
	case devcontainerCreatedMsg:
		// Devcontainer created successfully, clear any previous errors
		m.err = nil
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
		
	case "esc":
		// Handle Esc based on current mode
		if m.searchMode {
			// In search input mode: exit search mode and clear filter
			m.searchMode = false
			m.filterActive = false
			m.filterTerm = ""
			m.searchInput.Reset()
			m.searchInput.Blur()
			m.buildItems()
			return m, nil
		} else if m.filterActive {
			// Filter is active but not in search mode: clear filter
			m.filterActive = false
			m.filterTerm = ""
			m.buildItems()
			return m, nil
		}
		// No filter: quit app
		m.quitting = true
		return m, tea.Quit
		
	case "/":
		// Activate search mode
		m.searchMode = true
		m.searchInput.Focus()
		m.err = nil // Clear any previous errors
		return m, nil
		
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
			// Skip category items
			for m.selectedIndex >= 0 && m.items[m.selectedIndex].Type == ItemTypeCategory {
				m.selectedIndex--
			}
			// If we went past the start, wrap to first non-category
			if m.selectedIndex < 0 {
				m.selectedIndex = 0
				for m.selectedIndex < len(m.items) && m.items[m.selectedIndex].Type == ItemTypeCategory {
					m.selectedIndex++
				}
			}
		}
		
	case "down", "j":
		if m.selectedIndex < len(m.items)-1 {
			m.selectedIndex++
			// Skip category items
			for m.selectedIndex < len(m.items) && m.items[m.selectedIndex].Type == ItemTypeCategory {
				m.selectedIndex++
			}
			// If we went past the end, stay at last non-category
			if m.selectedIndex >= len(m.items) {
				m.selectedIndex = len(m.items) - 1
				for m.selectedIndex >= 0 && m.items[m.selectedIndex].Type == ItemTypeCategory {
					m.selectedIndex--
				}
			}
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
		// Enter create mode (only for project and worktree items)
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			if item.Type != ItemTypeCategory {
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
		
	case "c":
		// Enter category input mode (only for projects)
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			if item.Type == ItemTypeProject {
				m.categoryInputMode = true
				m.categoryProject = item.ProjectPath
				m.categoryInput.Reset()
				m.categoryInput.Focus()
				m.err = nil // Clear any previous errors
				return m, nil
			}
		}		
	case "t":
		// Enter tag input mode (only for projects)
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			item := m.items[m.selectedIndex]
			if item.Type == ItemTypeProject {
				m.tagInputMode = true
				m.tagProject = item.ProjectPath
				m.tagInput.Reset()
				m.tagInput.Focus()
				m.err = nil // Clear any previous errors
				return m, nil
			}
		}
	
	case "v":
		// Create workspace file for selected item
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			return m, m.createWorkspaceFile()
		}
		return m, nil

	case "i":
		// Create devcontainer for selected item
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) {
			return m, m.createDevcontainer()
		}
		return m, nil

	case "e":
		// Open config file in VS Code
		return m, m.openConfigInVSCode()
	
	case "r":
		// Refresh worktrees and status
		m.err = nil // Clear any previous errors
		m.loadWorktrees()
		// Try to maintain the selected index, or reset to first valid item if out of bounds
		if m.selectedIndex >= len(m.items) {
			m.selectedIndex = 0
			for m.selectedIndex < len(m.items) && m.items[m.selectedIndex].Type == ItemTypeCategory {
				m.selectedIndex++
			}
		}
		return m, nil
	}
	
	return m, nil
}

// matchesFilter checks if an item matches the current filter term.
// Matches against project name, worktree branch name, tags, and category (case-insensitive).
func (m *Model) matchesFilter(category string, project config.Project, wt *worktree.Worktree) bool {
	if m.filterTerm == "" {
		return true
	}
	
	filterLower := strings.ToLower(m.filterTerm)
	
	// Match against category
	if strings.Contains(strings.ToLower(category), filterLower) {
		return true
	}
	
	// Match against project name
	if strings.Contains(strings.ToLower(project.Name), filterLower) {
		return true
	}
	
	// Match against project tags
	for _, tag := range project.Tags {
		if strings.Contains(strings.ToLower(tag), filterLower) {
			return true
		}
	}
	
	// If worktree is provided, match against branch name
	if wt != nil {
		if strings.Contains(strings.ToLower(wt.Branch), filterLower) {
			return true
		}
	}
	
	return false
}

// buildItems creates a flattened list of items for navigation.
func (m *Model) buildItems() {
	oldItemsCount := len(m.items)
	m.items = []Item{}
	
	// Group projects by category
	categoryProjects := make(map[string][]config.Project)
	var uncategorized []config.Project
	
	for _, project := range m.projects {
		if project.Category != "" {
			categoryProjects[project.Category] = append(categoryProjects[project.Category], project)
		} else {
			uncategorized = append(uncategorized, project)
		}
	}
	
	// Build items in order: categories (from config.categories), then uncategorized
	for _, category := range m.categories {
		projects := categoryProjects[category]
		if len(projects) == 0 {
			continue // Skip empty categories
		}
		
		// When filtering, check if category has any matches
		categoryHasMatches := false
		if m.filterTerm != "" {
			for _, project := range projects {
				if m.matchesFilter(category, project, nil) {
					categoryHasMatches = true
					break
				}
				// Check worktrees
				if wts, ok := m.worktrees[project.Path]; ok {
					for i := range wts {
						if m.matchesFilter(category, project, &wts[i]) {
							categoryHasMatches = true
							break
						}
					}
				}
			}
		} else {
			categoryHasMatches = true // No filter, show all
		}
		
		// Skip category if no matches when filtering
		if !categoryHasMatches {
			continue
		}
		
		// Add category item
		m.items = append(m.items, Item{
			Type:     ItemTypeCategory,
			Category: category,
		})
		
		// Add projects under this category (always expanded)
		for _, project := range projects {
			// Check if project matches filter
			projectMatches := m.filterTerm == "" || m.matchesFilter(category, project, nil)
			
			// Check if any worktrees match (check all when filtering, only expanded when not)
			hasMatchingWorktrees := false
			if m.filterTerm != "" {
				// When filtering, check all worktrees
				if wts, ok := m.worktrees[project.Path]; ok {
					for i := range wts {
						if m.matchesFilter(category, project, &wts[i]) {
							hasMatchingWorktrees = true
							break
						}
					}
				}
			} else if m.expandedProjects[project.Path] {
				// When not filtering, only check if expanded
				if wts, ok := m.worktrees[project.Path]; ok {
					hasMatchingWorktrees = len(wts) > 0
				}
			}
			
			// Skip project if neither it nor its worktrees match
			if !projectMatches && !hasMatchingWorktrees {
				continue
			}
			
			// Add project header
			m.items = append(m.items, Item{
				Type:        ItemTypeProject,
				Category:    category,
				ProjectName: project.Name,
				ProjectPath: project.Path,
				ProjectTags: project.Tags,
			})
			
			// Add worktrees: when filtering show all matches, when not filtering respect expansion
			if m.filterTerm != "" || m.expandedProjects[project.Path] {
				if wts, ok := m.worktrees[project.Path]; ok {
					for i := range wts {
						// Apply filter to worktrees if filtering
						if m.filterTerm != "" && !m.matchesFilter(category, project, &wts[i]) {
							continue
						}
						m.items = append(m.items, Item{
							Type:        ItemTypeWorktree,
							Category:    category,
							ProjectName: project.Name,
							ProjectPath: project.Path,
							ProjectTags: project.Tags,
							Worktree:    &wts[i],
						})
					}
				}
			}
		}
	}
	
	// Add uncategorized projects at the bottom
	if len(uncategorized) > 0 {
		// When filtering, check if uncategorized has any matches
		uncategorizedHasMatches := false
		if m.filterTerm != "" {
			for _, project := range uncategorized {
				if m.matchesFilter("Uncategorized", project, nil) {
					uncategorizedHasMatches = true
					break
				}
				// Check worktrees
				if wts, ok := m.worktrees[project.Path]; ok {
					for i := range wts {
						if m.matchesFilter("Uncategorized", project, &wts[i]) {
							uncategorizedHasMatches = true
							break
						}
					}
				}
			}
		} else {
			uncategorizedHasMatches = true // No filter, show all
		}
		
		// Skip uncategorized section if no matches when filtering
		if !uncategorizedHasMatches {
			goto skipUncategorized
		}
		
		// Add "Uncategorized" category item
		m.items = append(m.items, Item{
			Type:     ItemTypeCategory,
			Category: "Uncategorized",
		})
		
		// Add projects under uncategorized (always expanded)
		for _, project := range uncategorized {
			// Check if project matches filter
			projectMatches := m.filterTerm == "" || m.matchesFilter("Uncategorized", project, nil)
			
			// Check if any worktrees match (check all when filtering, only expanded when not)
			hasMatchingWorktrees := false
			if m.filterTerm != "" {
				// When filtering, check all worktrees
				if wts, ok := m.worktrees[project.Path]; ok {
					for i := range wts {
						if m.matchesFilter("Uncategorized", project, &wts[i]) {
							hasMatchingWorktrees = true
							break
						}
					}
				}
			} else if m.expandedProjects[project.Path] {
				// When not filtering, only check if expanded
				if wts, ok := m.worktrees[project.Path]; ok {
					hasMatchingWorktrees = len(wts) > 0
				}
			}
			
			// Skip project if neither it nor its worktrees match
			if !projectMatches && !hasMatchingWorktrees {
				continue
			}
			
			// Add project header
			m.items = append(m.items, Item{
				Type:        ItemTypeProject,
				Category:    "Uncategorized",
				ProjectName: project.Name,
				ProjectPath: project.Path,
				ProjectTags: project.Tags,
			})
			
			// Add worktrees: when filtering show all matches, when not filtering respect expansion
			if m.filterTerm != "" || m.expandedProjects[project.Path] {
				if wts, ok := m.worktrees[project.Path]; ok {
					for i := range wts {
						// Apply filter to worktrees if filtering
						if m.filterTerm != "" && !m.matchesFilter("Uncategorized", project, &wts[i]) {
							continue
						}
						m.items = append(m.items, Item{
							Type:        ItemTypeWorktree,
							Category:    "Uncategorized",
							ProjectName: project.Name,
							ProjectPath: project.Path,
							ProjectTags: project.Tags,
							Worktree:    &wts[i],
						})
					}
				}
			}
		}
	}
	
skipUncategorized:
	
	// Skip over category items in selection - ensure we're on a selectable item
	if len(m.items) > 0 {
		if m.selectedIndex >= len(m.items) {
			m.selectedIndex = len(m.items) - 1
		}
		// If current selection is a category, move to next non-category item
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.items) && m.items[m.selectedIndex].Type == ItemTypeCategory {
			// Find next non-category item
			for i := m.selectedIndex + 1; i < len(m.items); i++ {
				if m.items[i].Type != ItemTypeCategory {
					m.selectedIndex = i
					break
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

// categoryAssignedMsg is sent when a category is successfully assigned to a project.
type categoryAssignedMsg struct {
	projectPath string
	category    string
}

// tagsAssignedMsg is sent when tags are successfully assigned to a project.
type tagsAssignedMsg struct {
	projectPath string
	tags        []string
}

// workspaceCreatedMsg is sent when a workspace file is successfully created.
type workspaceCreatedMsg struct {
	workspacePath string
}

// devcontainerCreatedMsg is sent when a .devcontainer folder is successfully created.
type devcontainerCreatedMsg struct {
	targetPath string
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
		// Look up project subfolder from config
		var projectSubFolder string
		for _, project := range m.projects {
			if project.Path == item.ProjectPath {
				projectSubFolder = project.SubFolder
				break
			}
		}

		effectivePath := workspace.GetTargetPath(pathToOpen, projectSubFolder)

		if err := vscode.OpenInVSCode(effectivePath); err != nil {
			return vsCodeErrorMsg{err: err}
		}
		return nil
	}
}

// openConfigInVSCode opens the wt config file in VS Code.
func (m Model) openConfigInVSCode() tea.Cmd {
	return func() tea.Msg {
		configPath, err := config.GetConfigPath()
		if err != nil {
			return vsCodeErrorMsg{err: fmt.Errorf("failed to get config path: %w", err)}
		}
		
		if err := vscode.OpenInVSCode(configPath); err != nil {
			return vsCodeErrorMsg{err: fmt.Errorf("failed to open config file: %w", err)}
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

// createWorkspaceFile creates a workspace file for the selected item.
func (m Model) createWorkspaceFile() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.items) {
		return nil
	}
	
	item := m.items[m.selectedIndex]
	
	// Determine the target path based on item type
	var targetPath string
	switch item.Type {
	case ItemTypeProject:
		targetPath = item.ProjectPath
	case ItemTypeWorktree:
		if item.Worktree == nil {
			return nil
		}
		targetPath = item.Worktree.Path
	default:
		return nil
	}
	
	return func() tea.Msg {
		// Look up project color and subfolder from config
		var projectColor string
		var projectSubFolder string
		for _, project := range m.projects {
			if project.Path == item.ProjectPath {
				projectColor = project.Color
				projectSubFolder = project.SubFolder
				break
			}
		}

		effectivePath := workspace.GetTargetPath(targetPath, projectSubFolder)

		if err := workspace.CreateOrCopyWorkspaceFileWithColor(effectivePath, projectColor); err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("failed to create workspace file: %w", err)}
		}
		
		return workspaceCreatedMsg{workspacePath: targetPath}
	}
}

// createDevcontainer creates a .devcontainer folder for the selected item.
func (m Model) createDevcontainer() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.items) {
		return nil
	}

	item := m.items[m.selectedIndex]

	// Determine the target path based on item type
	var targetPath string
	switch item.Type {
	case ItemTypeProject:
		targetPath = item.ProjectPath
	case ItemTypeWorktree:
		if item.Worktree == nil {
			return nil
		}
		targetPath = item.Worktree.Path
	default:
		return nil
	}

	return func() tea.Msg {
		// Look up project color, subfolder, and name from config
		var projectColor string
		var projectSubFolder string
		var projectName string
		for _, project := range m.projects {
			if project.Path == item.ProjectPath {
				projectColor = project.Color
				projectSubFolder = project.SubFolder
				projectName = filepath.Base(project.Path)
				break
			}
		}

		effectivePath := workspace.GetTargetPath(targetPath, projectSubFolder)

		if err := devcontainer.CreateDevcontainerWithColor(effectivePath, projectColor, projectName); err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("failed to create devcontainer: %w", err)}
		}

		return devcontainerCreatedMsg{targetPath: targetPath}
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
		
		// Generate color for the project based on its path
		projectColor := workspace.GenerateColorFromPath(absPath)
		
		// Add new project
		cfg.Projects = append(cfg.Projects, config.Project{
			Name:  name,
			Path:  absPath,
			Color: projectColor,
		})
		
		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}
		
		return projectAddedMsg{projectPath: absPath}
	}
}

// assignCategory assigns a category to a project.
func (m Model) assignCategory(projectPath, categoryName string) tea.Cmd {
	return func() tea.Msg {
		// Load existing config
		cfg, err := config.LoadConfig()
		if err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("failed to load config: %w", err)}
		}
		
		// Find the project
		var project *config.Project
		for i := range cfg.Projects {
			if cfg.Projects[i].Path == projectPath {
				project = &cfg.Projects[i]
				break
			}
		}
		
		if project == nil {
			return worktreeErrorMsg{err: fmt.Errorf("project not found: %s", projectPath)}
		}
		
		// Add category to config if it doesn't exist
		cfg.AddCategory(categoryName)
		
		// Set project category
		project.SetCategory(categoryName)
		
		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}
		
		return categoryAssignedMsg{projectPath: projectPath, category: categoryName}
	}
}
// assignTags assigns tags to a project.
func (m Model) assignTags(projectPath, tagInput string) tea.Cmd {
	return func() tea.Msg {
		// Load existing config
		cfg, err := config.LoadConfig()
		if err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("failed to load config: %w", err)}
		}
		
		// Find the project
		var project *config.Project
		for i := range cfg.Projects {
			if cfg.Projects[i].Path == projectPath {
				project = &cfg.Projects[i]
				break
			}
		}
		
		if project == nil {
			return worktreeErrorMsg{err: fmt.Errorf("project not found: %s", projectPath)}
		}
		
		// Parse comma-separated tags
		tagStrs := strings.Split(tagInput, ",")
		var tags []string
		for _, tag := range tagStrs {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
		
		if len(tags) == 0 {
			return worktreeErrorMsg{err: fmt.Errorf("no valid tags provided")}
		}
		
		// Add tags to project (AddTags avoids duplicates)
		project.AddTags(tags...)
		
		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			return worktreeErrorMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}
		
		return tagsAssignedMsg{projectPath: projectPath, tags: tags}
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
