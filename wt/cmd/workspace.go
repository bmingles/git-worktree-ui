package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmingles/wt/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceColor string

var workspaceCmd = &cobra.Command{
	Use:   "wksp [path]",
	Short: "Add a local.code-workspace file to a directory",
	Long:  `Create a local.code-workspace file in the specified directory. Defaults to the current directory if no path is given.`,
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

		if err := workspace.CreateOrCopyWorkspaceFileWithColor(absPath, workspaceColor); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating workspace file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created local.code-workspace in: %s\n", absPath)
	},
}

func init() {
	rootCmd.AddCommand(workspaceCmd)

	workspaceCmd.Flags().StringVar(&workspaceColor, "color", "", "Custom hex color for the workspace (e.g. d37cef)")
}
