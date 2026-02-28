package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/danabrams/gromit/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch Gromit TUI",
	RunE:  runTui,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTui(_ *cobra.Command, _ []string) error {
	// Create store for UI state
	store := &tui.Store{}

	// Create the real Model with the store
	model := tui.NewModel(store)

	// Launch the TUI with the real model
	p := tea.NewProgram(model)
	_, err := p.Run()
	return err
}
