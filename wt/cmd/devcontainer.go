package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmingles/wt/pkg/devcontainer"
	"github.com/spf13/cobra"
)

var (
	devcontainerColor string
	devcontainerName  string
)

var devcontainerCmd = &cobra.Command{
	Use:   "devc [path]",
	Short: "Add a .devcontainer to a directory",
	Long:  `Create a .devcontainer folder in the specified directory. Defaults to the current directory if no path is given.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
			os.Exit(1)
		}

		name := devcontainerName
		if name == "" {
			name = filepath.Base(absPath)
		}

		if err := devcontainer.CreateDevcontainerWithColor(absPath, devcontainerColor, name); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating devcontainer: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created .devcontainer in: %s\n", absPath)
	},
}

func init() {
	rootCmd.AddCommand(devcontainerCmd)

	devcontainerCmd.Flags().StringVar(&devcontainerColor, "color", "", "Custom hex color for the devcontainer (e.g. d37cef)")
	devcontainerCmd.Flags().StringVar(&devcontainerName, "name", "", "Project name for scoping .tasks mount (defaults to directory name)")
}
