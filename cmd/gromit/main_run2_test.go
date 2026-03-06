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

func hasSubcommand(parent *cobra.Command, name string) bool {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return true
		}
	}
	return false
}
