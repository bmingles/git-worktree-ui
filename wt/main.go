package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bmingles/wt/pkg/config"
	"github.com/bmingles/wt/pkg/tui"
)

func main() {
	// Initialize config
	if err := config.InitConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		os.Exit(1)
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check if there are any projects
	if len(cfg.Projects) == 0 {
		fmt.Println("No projects configured.")
		fmt.Println("Add projects to ~/.config/wt/config.yaml")
		os.Exit(0)
	}

	// Create and run TUI
	model := tui.NewModel(cfg.Projects)
	p := tea.NewProgram(model, tea.WithAltScreen())
	
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
