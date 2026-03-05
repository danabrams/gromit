package main

import (
	"fmt"
	"os"

	"github.com/danabrams/gromit/internal/config"
	"github.com/spf13/cobra"
)

var configPath string

var rootCmd = &cobra.Command{
	Use:           "gromit",
	Short:         "Gromit - Execute the Gromit loop correctly",
	SilenceErrors: true,
	Long: `Gromit executes AI coding tasks with fresh context on each iteration.

It integrates with bd (beads) for task management and uses model escalation
for handling failures efficiently.`,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "gromit.yaml", "Path to config file")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if !commandRequiresRepoRoot(cmd) {
			return nil
		}
		return ensureRepoRoot()
	}

	registerRootCommands(rootCmd)
}

func registerRootCommands(root *cobra.Command) {
	registerRunCommand(root)
	registerStatusCommand(root)
	registerRetroCommand(root)
	root.AddCommand(validatePRMetadataCmd)
	registerBenchmarkCommands(root)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func commandRequiresRepoRoot(cmd *cobra.Command) bool {
	if isInitCommand(cmd) {
		return false
	}
	if isBenchmarkCommand(cmd) {
		return false
	}
	if isValidatePRMetadataCommand(cmd) {
		return false
	}
	return true
}

func isValidatePRMetadataCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd == validatePRMetadataCmd {
		return true
	}
	if cmd.Name() == validatePRMetadataCmd.Name() {
		return true
	}
	if cmd.Use == validatePRMetadataCmd.Use {
		return true
	}
	return false
}

func isInitCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd == initCmd {
		return true
	}
	if cmd.Name() == initCmd.Name() {
		return true
	}
	if cmd.Use == initCmd.Use {
		return true
	}
	return false
}

func isBenchmarkCommand(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current == benchmarkCmd {
			return true
		}
	}
	return false
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		if os.IsNotExist(err) && configPath == "gromit.yaml" {
			return nil, fmt.Errorf("gromit.yaml not found - run 'gromit init' to set up this project")
		}
		return nil, err
	}
	return cfg, nil
}
