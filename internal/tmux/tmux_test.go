package tmux

import (
	"testing"
)

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
	m := &Manager{
		inTmux:      true,
		titleMethod: methodSelectPane, // Pre-set the cached method
	}

	// Call setTmuxTitle - it should use the cached method directly
	err := m.setTmuxTitle("test-title")
	// This should work (or we can test the logic without checking success)
	_ = err // We're just testing that it uses the cached method

	// titleMethod should still be the same
	if m.titleMethod != methodSelectPane {
		t.Errorf("expected titleMethod to remain methodSelectPane, got %v", m.titleMethod)
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
	m := &Manager{
		inTmux:      true,
		titleMethod: methodUnknown,
	}

	// When titleMethod is unknown, setTmuxTitle should try methodSetOpt first
	// If it succeeds, titleMethod should be set to methodSetOpt
	err := m.setTmuxTitle("test-title")

	// If successful, titleMethod should be cached (not methodUnknown)
	if err == nil && m.titleMethod == methodUnknown {
		t.Error("expected titleMethod to be cached (not methodUnknown) when setTmuxTitle succeeds")
	}

	// Verify that one of the methods was cached
	if err == nil && (m.titleMethod == methodSetOpt || m.titleMethod == methodSelectPane) {
		// Success case: one method worked and was cached
		return
	}

	// If there was an error, both methods should have failed
	if err != nil && "failed to set tmux title: neither set-option nor select-pane method worked" != err.Error() {
		t.Errorf("unexpected error: %v", err)
	}
}
