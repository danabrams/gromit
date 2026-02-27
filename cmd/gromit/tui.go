package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch a stub Bubble Tea interface",
	RunE:  runTui,
}

type stubModel struct{}

var tuiStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))

func runTui(_ *cobra.Command, _ []string) error {
	p := tea.NewProgram(stubModel{})
	_, err := p.Run()
	return err
}

func (stubModel) Init() tea.Cmd {
	return nil
}

func (stubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			return stubModel{}, tea.Quit
		}
	}
	return stubModel{}, nil
}

func (stubModel) View() string {
	title := tuiStyle.Render("Gromit TUI")
	body := "This stub Bubble Tea UI exits when you press q."
	return fmt.Sprintf("%s\n%s\n\nPress q to quit.\n", title, body)
}
