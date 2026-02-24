package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmingles/wt/pkg/config"
	"github.com/spf13/cobra"
)

var (
	// configAddCmd flags
	addTags []string
	// configListCmd flags
	filterTag string
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
		newProject := config.Project{
			Name: name,
			Path: absPath,
			Tags: addTags,
		}
		cfg.Projects = append(cfg.Projects, newProject)

		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Added project '%s' at path: %s\n", name, absPath)
		if len(addTags) > 0 {
			fmt.Printf("Tags: %s\n", strings.Join(addTags, ", "))
		}
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

		// Filter projects by tag if specified
		projects := cfg.Projects
		if filterTag != "" {
			projects = cfg.FilterProjectsByTag(filterTag)
			if len(projects) == 0 {
				fmt.Printf("No projects found with tag '%s'.\n", filterTag)
				return
			}
			fmt.Printf("Projects with tag '%s':\n", filterTag)
		} else {
			fmt.Println("Configured projects:")
		}

		// Display projects
		for _, p := range projects {
			fmt.Printf("  %s: %s", p.Name, p.Path)
			if len(p.Tags) > 0 {
				fmt.Printf(" [%s]", strings.Join(p.Tags, ", "))
			}
			fmt.Println()
		}
	},
}

// configTagCmd manages tags for projects
var configTagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage project tags",
	Long:  `Add, remove, or list tags for projects.`,
}

// configTagAddCmd adds tags to a project
var configTagAddCmd = &cobra.Command{
	Use:   "add <project-name> <tag1> [tag2...]",
	Short: "Add tags to a project",
	Long:  `Add one or more tags to an existing project.`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := args[0]
		tags := args[1:]

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

		// Find project
		project, err := cfg.FindProject(projectName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Add tags
		if project.AddTags(tags...) {
			// Save config
			if err := config.SaveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Added tags to project '%s': %s\n", projectName, strings.Join(tags, ", "))
		} else {
			fmt.Printf("All tags already exist on project '%s'\n", projectName)
		}
	},
}

// configTagRemoveCmd removes tags from a project
var configTagRemoveCmd = &cobra.Command{
	Use:   "remove <project-name> <tag1> [tag2...]",
	Short: "Remove tags from a project",
	Long:  `Remove one or more tags from an existing project.`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := args[0]
		tags := args[1:]

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

		// Find project
		project, err := cfg.FindProject(projectName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Remove tags
		if project.RemoveTags(tags...) {
			// Save config
			if err := config.SaveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Removed tags from project '%s': %s\n", projectName, strings.Join(tags, ", "))
		} else {
			fmt.Printf("None of the specified tags were found on project '%s'\n", projectName)
		}
	},
}

// configTagListCmd lists all tags or tags for a specific project
var configTagListCmd = &cobra.Command{
	Use:   "list [project-name]",
	Short: "List tags",
	Long:  `List all unique tags across all projects, or tags for a specific project.`,
	Args:  cobra.MaximumNArgs(1),
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

		// If project name provided, list tags for that project
		if len(args) > 0 {
			projectName := args[0]
			project, err := cfg.FindProject(projectName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(project.Tags) == 0 {
				fmt.Printf("Project '%s' has no tags.\n", projectName)
			} else {
				fmt.Printf("Tags for project '%s':\n", projectName)
				for _, tag := range project.Tags {
					fmt.Printf("  - %s\n", tag)
				}
			}
		} else {
			// List all unique tags
			tags := cfg.GetAllTags()
			if len(tags) == 0 {
				fmt.Println("No tags configured.")
			} else {
				fmt.Println("All tags:")
				for _, tag := range tags {
					fmt.Printf("  - %s\n", tag)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configAddCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configTagCmd)
	
	// Add tag subcommands
	configTagCmd.AddCommand(configTagAddCmd)
	configTagCmd.AddCommand(configTagRemoveCmd)
	configTagCmd.AddCommand(configTagListCmd)
	
	// Add flags
	configAddCmd.Flags().StringSliceVarP(&addTags, "tags", "t", []string{}, "Tags for the project (comma-separated)")
	configListCmd.Flags().StringVar(&filterTag, "tag", "", "Filter projects by tag")
}
