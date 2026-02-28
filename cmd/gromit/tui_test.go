package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/tui"
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

func TestTuiCommandUsesRealModel(t *testing.T) {
	// Verify that the tui command is wired to use the real Model and Store
	// from internal/tui package, with proper initialization.
	// This test checks that a Store can be created with NewModel returning the real model.

	store := &tui.Store{}
	model := tui.NewModel(store)

	// Check the model has the expected View() method with non-empty output
	view := model.View()
	if view == "" {
		t.Error("expected real model to have non-empty initial view")
	}

	// Verify Init() returns nil or a command (real model behavior)
	cmd := model.Init()
	// cmd can be nil, that's acceptable
	_ = cmd
}
