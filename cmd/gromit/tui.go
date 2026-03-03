package main

import (
	"errors"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danabrams/gromit/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch Gromit TUI",
	RunE:  runTui,
}

var osExecutable = os.Executable

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTui(_ *cobra.Command, _ []string) error {
	// Create store for UI state
	store := &tui.Store{}

	for {
		// Create the real Model with the store
		model := tui.NewModel(store)

		// Launch the TUI with the real model
		p := tea.NewProgram(model)
		if _, err := p.Run(); err != nil {
			return err
		}

		pending := model.PendingAction()
		if pending == nil {
			return nil
		}

		cmd, err := buildPendingActionCommand(pending)
		if err != nil {
			return err
		}

		if err := cmd.Run(); err != nil {
			return err
		}
	}
}

func buildPendingActionCommand(action *tui.PendingAction) (*exec.Cmd, error) {
	if action == nil || action.Command == "" {
		return nil, errors.New("pending action missing command")
	}

	executable, err := osExecutable()
	if err != nil {
		return nil, err
	}

	args := append([]string{action.Command}, action.Args...)
	cmd := exec.Command(executable, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, nil
}
