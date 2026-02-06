package tmux

import (
	"testing"
)

func TestSetTitle_DisablesAfterFirstError(t *testing.T) {
	m := &Manager{
		inTmux:   true,
		disabled: false,
	}

	// First call should return an error (setTmuxTitle will fail in test environment)
	err1 := m.SetTitle("title1")
	if err1 == nil {
		t.Fatal("expected first SetTitle to fail")
	}

	// After first error, disabled should be true
	if !m.disabled {
		t.Error("expected disabled to be true after first error")
	}

	// Second call should return nil without trying to set title
	err2 := m.SetTitle("title2")
	if err2 != nil {
		t.Errorf("expected second SetTitle to return nil, got %v", err2)
	}

	// Verify disabled is still true
	if !m.disabled {
		t.Error("expected disabled to remain true")
	}
}

func TestSetTitle_NoOpWhenDisabled(t *testing.T) {
	m := &Manager{
		inTmux:   true,
		disabled: true,
	}

	// Should return nil when already disabled
	err := m.SetTitle("title")
	if err != nil {
		t.Errorf("expected SetTitle to return nil when disabled, got %v", err)
	}
}

func TestSetTitle_NoOpWhenNotInTmux(t *testing.T) {
	m := &Manager{
		inTmux:   false,
		disabled: false,
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
		inTmux: false,
	}

	if m.disabled {
		t.Error("expected disabled to be false on new manager")
	}
}
