package tui

import (
	"testing"
)

func TestKeymap_HasAllRequiredBindings(t *testing.T) {
	km := DefaultKeymap()

	// Check that the keymap has all the required bindings
	if km == nil {
		t.Error("expected keymap to be non-nil")
	}

	// Verify some basic key bindings exist
	if km.Quit == "" {
		t.Error("expected Quit binding to be non-empty")
	}

	if km.FocusNext == "" {
		t.Error("expected FocusNext binding to be non-empty")
	}

	if km.FocusPrev == "" {
		t.Error("expected FocusPrev binding to be non-empty")
	}

	if km.ScrollUp == "" {
		t.Error("expected ScrollUp binding to be non-empty")
	}

	if km.ScrollDown == "" {
		t.Error("expected ScrollDown binding to be non-empty")
	}

	if km.SwitchView == "" {
		t.Error("expected SwitchView binding to be non-empty")
	}
}

func TestKeymap_DefaultBindingsAreReasonable(t *testing.T) {
	km := DefaultKeymap()

	// Verify that default bindings are reasonable
	if km.Quit != "q" && km.Quit != "ctrl+c" {
		t.Errorf("expected Quit to be 'q' or 'ctrl+c', got %q", km.Quit)
	}

	if km.FocusNext != "tab" {
		t.Errorf("expected FocusNext to be 'tab', got %q", km.FocusNext)
	}

	if km.FocusPrev != "shift+tab" {
		t.Errorf("expected FocusPrev to be 'shift+tab', got %q", km.FocusPrev)
	}
}
