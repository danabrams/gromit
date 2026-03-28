package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestMainRegistersRun2Command(t *testing.T) {
	if !hasSubcommand(rootCmd, "run2") {
		t.Fatalf("root command is missing run2 subcommand")
	}
}

func TestRegisterRootCommandsRegistersDebug(t *testing.T) {
	t.Parallel()
	root := &cobra.Command{}
	registerRootCommands(root)
	if !hasSubcommand(root, debugCmd.Name()) {
		t.Fatalf("registerRootCommands is missing the debug subcommand")
	}
}

func hasSubcommand(parent *cobra.Command, name string) bool {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return true
		}
	}
	return false
}
