package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmingles/wt/pkg/config"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage wt configuration",
	Long:  `Manage wt configuration including projects and settings.`,
}

// configInitCmd creates a default config file
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize config file",
	Long:  `Create a default config file at ~/.config/wt/config.yaml with an empty projects list.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Set custom config path if provided
		if cfgFile != "" {
			config.SetConfigPath(cfgFile)
		}

		if err := config.InitConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
			os.Exit(1)
		}

		configPath, _ := config.GetConfigPath()
		fmt.Printf("Config file created at: %s\n", configPath)
	},
}

// configAddCmd adds a project to the config
var configAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Add a project to the config",
	Long:  `Add a new project with the specified name and path to the configuration.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		path := args[1]

		// Set custom config path if provided
		if cfgFile != "" {
			config.SetConfigPath(cfgFile)
		}

		// Make path absolute
		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
			os.Exit(1)
		}

		// Check if path exists
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: path does not exist: %s\n", absPath)
			os.Exit(1)
		}

		// Load existing config
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Check if project with this name already exists
		for _, p := range cfg.Projects {
			if p.Name == name {
				fmt.Fprintf(os.Stderr, "Error: project with name '%s' already exists\n", name)
				os.Exit(1)
			}
		}

		// Add new project
		cfg.Projects = append(cfg.Projects, config.Project{
			Name: name,
			Path: absPath,
		})

		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Added project '%s' at path: %s\n", name, absPath)
	},
}

// configListCmd lists all configured projects
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured projects",
	Long:  `Display all projects currently configured in the config file.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Set custom config path if provided
		if cfgFile != "" {
			config.SetConfigPath(cfgFile)
		}

		// Load config
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		if len(cfg.Projects) == 0 {
			fmt.Println("No projects configured.")
			fmt.Println("Use 'wt config add <name> <path>' to add a project.")
			return
		}

		fmt.Println("Configured projects:")
		for _, p := range cfg.Projects {
			fmt.Printf("  %s: %s\n", p.Name, p.Path)
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configAddCmd)
	configCmd.AddCommand(configListCmd)
}
