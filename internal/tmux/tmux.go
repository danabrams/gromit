package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Manager handles tmux pane title management
type Manager struct {
	inTmux        bool
	originalTitle string
	disabled      bool
}

// NewManager creates a new tmux manager and saves the current pane title if in tmux
func NewManager() (*Manager, error) {
	m := &Manager{
		inTmux: InTmux(),
	}

	if m.inTmux {
		m.originalTitle = getOriginalTitle()
	}

	return m, nil
}

// InTmux returns true if running inside a tmux session
func InTmux() bool {
	return os.Getenv("TMUX") != ""
}

// SetTitle sets the tmux pane title
func (m *Manager) SetTitle(title string) error {
	if !m.inTmux || m.disabled {
		return nil // No-op if not in tmux or disabled
	}

	if err := setTmuxTitle(title); err != nil {
		m.disabled = true // Disable after first error
		return err
	}

	return nil
}

// RestoreTitle restores the original pane title
func (m *Manager) RestoreTitle() error {
	if !m.inTmux || m.originalTitle == "" {
		return nil // No-op if not in tmux or no original title saved
	}

	return setTmuxTitle(m.originalTitle)
}

// getOriginalTitle retrieves the current tmux pane title
func getOriginalTitle() string {
	cmd := exec.Command("tmux", "display-message", "-p", "#{pane_title}")
	output, err := cmd.Output()
	if err != nil {
		return "" // Return empty string on error
	}
	return strings.TrimSpace(string(output))
}

// setTmuxTitle sets the tmux pane title using the appropriate escape sequence
func setTmuxTitle(title string) error {
	cmd := exec.Command("tmux", "set-option", "-p", "pane-title", title)
	return cmd.Run()
}

// FormatIterationTitle formats a title string with iteration info
func FormatIterationTitle(iteration int, beadID string, model string) string {
	return fmt.Sprintf("Gromit: iter %d - %s (%s)", iteration, beadID, model)
}
