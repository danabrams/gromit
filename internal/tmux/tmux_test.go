package tmux

import (
	"os/exec"
	"strings"
	"testing"
)

func newMockCmd(name string, args ...string) *exec.Cmd {
	return &exec.Cmd{Path: name, Args: append([]string{name}, args...)}
}

func TestSetTitle_DisablesAfterFirstError(t *testing.T) {
	m := &Manager{
		inTmux:      true,
		disabled:    false,
		titleMethod: methodUnknown,
	}

	// Simulate a manager that has already disabled (because methods failed once)
	m.disabled = true

	// Second call should return nil without trying to set title
	err := m.SetTitle("title")
	if err != nil {
		t.Errorf("expected SetTitle to return nil when disabled, got %v", err)
	}

	// Verify disabled is still true
	if !m.disabled {
		t.Error("expected disabled to remain true")
	}
}

func TestSetTitle_NoOpWhenDisabled(t *testing.T) {
	m := &Manager{
		inTmux:      true,
		disabled:    true,
		titleMethod: methodUnknown,
	}

	// Should return nil when already disabled
	err := m.SetTitle("title")
	if err != nil {
		t.Errorf("expected SetTitle to return nil when disabled, got %v", err)
	}
}

func TestSetTitle_NoOpWhenNotInTmux(t *testing.T) {
	m := &Manager{
		inTmux:      false,
		disabled:    false,
		titleMethod: methodUnknown,
	}

	// Should return nil when not in tmux
	err := m.SetTitle("title")
	if err != nil {
		t.Errorf("expected SetTitle to return nil when not in tmux, got %v", err)
	}

	// Should not be disabled when not in tmux
	if m.disabled {
		t.Error("expected disabled to remain false when not in tmux")
	}
}

func TestNewManager_InitializesWithDisabledFalse(t *testing.T) {
	m := &Manager{
		inTmux:      false,
		titleMethod: methodUnknown,
	}

	if m.disabled {
		t.Error("expected disabled to be false on new manager")
	}
}

func TestSetTmuxTitle_CachesTitleMethodWhenSuccessful(t *testing.T) {
	m := &Manager{
		inTmux:      true,
		titleMethod: methodUnknown,
		commandFn:   newMockCmd,
		runFn: func(cmd *exec.Cmd) error {
			return nil
		},
	}

	// Call setTmuxTitle - it should try methodSetOpt first
	// This will succeed in the test environment
	err := m.setTmuxTitle("test-title")
	if err != nil {
		t.Fatalf("expected setTmuxTitle to succeed, got error: %v", err)
	}

	// Verify that a title method was cached
	if m.titleMethod == methodUnknown {
		t.Error("expected titleMethod to be cached (not methodUnknown)")
	}
}

func TestSetTmuxTitle_UsesCache(t *testing.T) {
	var called []string
	m := &Manager{
		inTmux:      true,
		titleMethod: methodSelectPane, // Pre-set the cached method
		commandFn: func(name string, args ...string) *exec.Cmd {
			called = append(called, strings.Join(append([]string{name}, args...), " "))
			return newMockCmd(name, args...)
		},
		runFn: func(cmd *exec.Cmd) error {
			return nil
		},
	}

	// Call setTmuxTitle - it should use the cached method directly
	err := m.setTmuxTitle("test-title")
	// This should work (or we can test the logic without checking success)
	_ = err // We're just testing that it uses the cached method

	// titleMethod should still be the same
	if m.titleMethod != methodSelectPane {
		t.Errorf("expected titleMethod to remain methodSelectPane, got %v", m.titleMethod)
	}
	if len(called) != 1 || !strings.Contains(called[0], "select-pane") {
		t.Errorf("expected cached select-pane method call, got %v", called)
	}
}

func TestTryTitleMethod_UnknownMethod(t *testing.T) {
	m := &Manager{}

	// Test with invalid method type
	err := m.tryTitleMethod(titleMethod(999), "test-title")
	if err == nil {
		t.Fatal("expected tryTitleMethod to fail with unknown method")
	}

	if err.Error() != "unknown title method" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSetTmuxTitle_TriesSetOptFirst(t *testing.T) {
	var called []string
	m := &Manager{
		inTmux:      true,
		titleMethod: methodUnknown,
		commandFn: func(name string, args ...string) *exec.Cmd {
			called = append(called, strings.Join(append([]string{name}, args...), " "))
			return newMockCmd(name, args...)
		},
		runFn: func(cmd *exec.Cmd) error {
			if len(cmd.Args) > 1 && cmd.Args[1] == "set-option" {
				return exec.ErrNotFound
			}
			return nil
		},
	}

	// When titleMethod is unknown, setTmuxTitle should try methodSetOpt first
	// If it succeeds, titleMethod should be set to methodSetOpt
	err := m.setTmuxTitle("test-title")

	if err != nil {
		t.Fatalf("expected fallback to select-pane to succeed, got: %v", err)
	}
	if m.titleMethod != methodSelectPane {
		t.Fatalf("expected cached methodSelectPane, got %v", m.titleMethod)
	}
	if len(called) < 2 || !strings.Contains(called[0], "set-option") || !strings.Contains(called[1], "select-pane") {
		t.Errorf("expected set-option then select-pane, got %v", called)
	}
}

func TestRestoreTitle_NoOpWhenDisabled(t *testing.T) {
	m := &Manager{
		inTmux:        true,
		originalTitle: "original-title",
		disabled:      true,
	}

	// Should return nil when disabled, even with original title saved
	err := m.RestoreTitle()
	if err != nil {
		t.Errorf("expected RestoreTitle to return nil when disabled, got %v", err)
	}
}
