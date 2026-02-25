package tui

import (
	"testing"

	"github.com/bmingles/wt/pkg/config"
)

func TestBuildItemsWithCategories(t *testing.T) {
	// Test data
	categories := []string{"work", "personal"}
	projects := []config.Project{
		{
			Name:     "project-a",
			Path:     "/path/to/project-a",
			Category: "work",
			Tags:     []string{"backend"},
		},
		{
			Name:     "project-b",
			Path:     "/path/to/project-b",
			Category: "work",
			Tags:     []string{"frontend"},
		},
		{
			Name:     "project-c",
			Path:     "/path/to/project-c",
			Category: "personal",
			Tags:     []string{},
		},
		{
			Name:     "project-d",
			Path:     "/path/to/project-d",
			Category: "",
			Tags:     []string{},
		},
	}

	m := NewModel(projects, categories)

	// Categories are always expanded, so we should see:
	// work (category), project-a, project-b, personal (category), project-c, Uncategorized (category), project-d
	expectedCount := 7
	if len(m.items) != expectedCount {
		t.Errorf("Expected %d items (categories always expanded), got %d", expectedCount, len(m.items))
	}

	// Verify first item is "work" category
	if m.items[0].Type != ItemTypeCategory || m.items[0].Category != "work" {
		t.Errorf("Expected first item to be work category, got %v", m.items[0])
	}

	// Verify next two items are projects under work
	if m.items[1].Type != ItemTypeProject || m.items[1].ProjectName != "project-a" {
		t.Errorf("Expected second item to be project-a, got %+v", m.items[1])
	}
	if m.items[2].Type != ItemTypeProject || m.items[2].ProjectName != "project-b" {
		t.Errorf("Expected third item to be project-b, got %+v", m.items[2])
	}

	// Verify "personal" category
	if m.items[3].Type != ItemTypeCategory || m.items[3].Category != "personal" {
		t.Errorf("Expected fourth item to be personal category, got %v", m.items[3])
	}

	// Verify project-c under personal
	if m.items[4].Type != ItemTypeProject || m.items[4].ProjectName != "project-c" {
		t.Errorf("Expected fifth item to be project-c, got %+v", m.items[4])
	}

	// Verify "Uncategorized" category
	if m.items[5].Type != ItemTypeCategory || m.items[5].Category != "Uncategorized" {
		t.Errorf("Expected sixth item to be Uncategorized category, got %v", m.items[5])
	}

	// Verify project-d under uncategorized
	if m.items[6].Type != ItemTypeProject || m.items[6].ProjectName != "project-d" {
		t.Errorf("Expected seventh item to be project-d, got %+v", m.items[6])
	}

	// Test that selectedIndex skips over categories
	m.selectedIndex = 0 // Start at work category
	m.buildItems()      // This should move selection to first non-category item
	if m.items[m.selectedIndex].Type == ItemTypeCategory {
		t.Errorf("selectedIndex should skip category items, but is at category: %v", m.items[m.selectedIndex])
	}
}

func TestBuildItemsWithoutCategories(t *testing.T) {
	// Test with no categories defined
	categories := []string{}
	projects := []config.Project{
		{
			Name:     "project-a",
			Path:     "/path/to/project-a",
			Category: "",
			Tags:     []string{},
		},
	}

	m := NewModel(projects, categories)

	// Should have Uncategorized category + project-a (always expanded)
	expectedCount := 2
	if len(m.items) != expectedCount {
		t.Errorf("Expected %d items (Uncategorized category + project), got %d", expectedCount, len(m.items))
	}

	if m.items[0].Type != ItemTypeCategory || m.items[0].Category != "Uncategorized" {
		t.Errorf("Expected first item to be Uncategorized category, got %+v", m.items[0])
	}

	if m.items[1].Type != ItemTypeProject || m.items[1].ProjectName != "project-a" {
		t.Errorf("Expected second item to be project-a, got %+v", m.items[1])
	}
}

func TestSearchFilter(t *testing.T) {
	// Test data with various categories, projects, and tags
	categories := []string{"frontend", "backend"}
	projects := []config.Project{
		{
			Name:     "react-app",
			Path:     "/path/to/react-app",
			Category: "frontend",
			Tags:     []string{"typescript", "web"},
		},
		{
			Name:     "vue-app",
			Path:     "/path/to/vue-app",
			Category: "frontend",
			Tags:     []string{"javascript", "web"},
		},
		{
			Name:     "api-server",
			Path:     "/path/to/api-server",
			Category: "backend",
			Tags:     []string{"golang", "api"},
		},
	}

	m := NewModel(projects, categories)

	// Test 1: Filter by project name
	m.filterTerm = "react"
	m.buildItems()
	
	// Should show: frontend (category), react-app (project)
	// Note: vue-app and backend category should be hidden
	if len(m.items) != 2 {
		t.Errorf("Expected 2 items when filtering by 'react', got %d", len(m.items))
	}
	if len(m.items) >= 2 {
		if m.items[0].Type != ItemTypeCategory || m.items[0].Category != "frontend" {
			t.Errorf("Expected first item to be frontend category, got %+v", m.items[0])
		}
		if m.items[1].Type != ItemTypeProject || m.items[1].ProjectName != "react-app" {
			t.Errorf("Expected second item to be react-app, got %+v", m.items[1])
		}
	}

	// Test 2: Filter by tag
	m.filterTerm = "golang"
	m.buildItems()
	
	// Should show: backend (category), api-server (project)
	if len(m.items) != 2 {
		t.Errorf("Expected 2 items when filtering by 'golang' tag, got %d", len(m.items))
	}
	if len(m.items) >= 2 {
		if m.items[0].Type != ItemTypeCategory || m.items[0].Category != "backend" {
			t.Errorf("Expected first item to be backend category, got %+v", m.items[0])
		}
		if m.items[1].Type != ItemTypeProject || m.items[1].ProjectName != "api-server" {
			t.Errorf("Expected second item to be api-server, got %+v", m.items[1])
		}
	}

	// Test 3: Filter by category name
	m.filterTerm = "front"
	m.buildItems()
	
	// Should show: frontend (category), react-app, vue-app
	if len(m.items) != 3 {
		t.Errorf("Expected 3 items when filtering by 'front' (category), got %d", len(m.items))
	}

	// Test 4: Filter with no matches
	m.filterTerm = "nonexistent"
	m.buildItems()
	
	// Should show no items
	if len(m.items) != 0 {
		t.Errorf("Expected 0 items when filtering with no matches, got %d", len(m.items))
	}

	// Test 5: Clear filter
	m.filterTerm = ""
	m.buildItems()
	
	// Should show all items: frontend (category), react-app, vue-app, backend (category), api-server
	if len(m.items) != 5 {
		t.Errorf("Expected 5 items with no filter, got %d", len(m.items))
	}

	// Test 6: Case-insensitive matching
	m.filterTerm = "REACT"
	m.buildItems()
	
	// Should show: frontend (category), react-app
	if len(m.items) != 2 {
		t.Errorf("Expected 2 items when filtering by 'REACT' (case-insensitive), got %d", len(m.items))
	}

	// Test 7: Filtering shows worktrees even when project is collapsed
	// Start with all projects collapsed
	for _, p := range projects {
		m.expandedProjects[p.Path] = false
	}
	m.filterTerm = "" // Clear filter first
	m.buildItems()
	
	// Now apply a filter - should show matching worktrees even though projects are collapsed
	m.filterTerm = "web"  // Matches tags on react-app and vue-app
	m.buildItems()
	
	// Should have more items than just categories and projects
	// Even though projects are collapsed, filtering should show worktrees
	// At minimum: frontend (category), react-app (project), vue-app (project) = 3 items
	if len(m.items) < 3 {
		t.Errorf("Expected at least 3 items when filtering with collapsed projects, got %d", len(m.items))
	}
}
