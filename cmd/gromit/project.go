package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// projectCmd is the parent command for project management subcommands.
// These commands operate on the new workspace-based architecture (internal/next/).
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects in the Gromit workspace (next-gen)",
}

var projectAttachCmd = &cobra.Command{
	Use:   "attach [repo-path]",
	Short: "Attach a repository as a project in the workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("gromit project attach: not implemented yet")
		fmt.Println("This will register a repo as a project cell in the Gromit workspace.")
		return nil
	},
}

var projectInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect a project and generate architecture/source-map artifacts",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("gromit project inspect: not implemented yet")
		fmt.Println("This will analyze the repo and produce architecture.json, source-map.json, and glossary.json.")
		return nil
	},
}

var projectGuideCmd = &cobra.Command{
	Use:   "guide",
	Short: "Generate the agent guide for a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("gromit project guide: not implemented yet")
		fmt.Println("This will render agent-guide.md from inspection artifacts and doctrine.")
		return nil
	},
}

func init() {
	projectCmd.AddCommand(projectAttachCmd)
	projectCmd.AddCommand(projectInspectCmd)
	projectCmd.AddCommand(projectGuideCmd)
	rootCmd.AddCommand(projectCmd)
}
