package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bmingles/wt/pkg/config"
	"github.com/bmingles/wt/pkg/tui"
	"github.com/spf13/cobra"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "Git worktree manager with TUI interface",
	Long:  `wt is a command-line tool for managing Git worktrees with an interactive TUI.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Launch TUI when no subcommand is provided
		launchTUI()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Persistent flag for custom config path
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/wt/config.yaml)")
}

// launchTUI initializes and launches the TUI
func launchTUI() {
	// Set custom config path if provided
	if cfgFile != "" {
		config.SetConfigPath(cfgFile)
	}

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
		fmt.Println("Run 'wt config init' to create a config file, then 'wt config add <name> <path>' to add projects.")
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
