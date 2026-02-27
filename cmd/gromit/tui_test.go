package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterRootCommandsAddsTui(t *testing.T) {
	root := &cobra.Command{}
	registerRootCommands(root)

	cmd, _, err := root.Find([]string{"tui"})
	if err != nil {
		t.Fatalf("unexpected error finding tui command: %v", err)
	}
	if cmd == nil || cmd.Use != "tui" {
		t.Fatalf("tui command not registered")
	}
}
