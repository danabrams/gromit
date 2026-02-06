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
	titleMethod   titleMethod // Cache which title method works
}

// titleMethod represents the tmux title-setting method to use
type titleMethod int

const (
	methodUnknown  titleMethod = iota // Not yet determined
	methodSetOpt                      // tmux set-option (3.0+)
	methodSelectPane                  // tmux select-pane -T (2.6+)
)

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

	if err := m.setTmuxTitle(title); err != nil {
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

	return m.setTmuxTitle(m.originalTitle)
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
// It tries the newer set-option method first (tmux 3.0+), then falls back to
// select-pane (tmux 2.6+) if the first method fails.
func (m *Manager) setTmuxTitle(title string) error {
	// If we know which method works, use it directly
	if m.titleMethod != methodUnknown {
		return m.tryTitleMethod(m.titleMethod, title)
	}

	// Try set-option first (tmux 3.0+)
	if err := m.tryTitleMethod(methodSetOpt, title); err == nil {
		m.titleMethod = methodSetOpt
		return nil
	}

	// Fall back to select-pane (tmux 2.6+)
	if err := m.tryTitleMethod(methodSelectPane, title); err == nil {
		m.titleMethod = methodSelectPane
		return nil
	}

	// Both methods failed
	return fmt.Errorf("failed to set tmux title: neither set-option nor select-pane method worked")
}

// tryTitleMethod attempts to set the title using the specified method
func (m *Manager) tryTitleMethod(method titleMethod, title string) error {
	var cmd *exec.Cmd
	switch method {
	case methodSetOpt:
		cmd = exec.Command("tmux", "set-option", "-p", "pane-title", title)
	case methodSelectPane:
		cmd = exec.Command("tmux", "select-pane", "-T", title)
	default:
		return fmt.Errorf("unknown title method")
	}

	return cmd.Run()
}

// FormatIterationTitle formats a title string with iteration info
func FormatIterationTitle(iteration int, beadID string, model string) string {
	return fmt.Sprintf("Gromit: iter %d - %s (%s)", iteration, beadID, model)
}
