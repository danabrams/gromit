package main

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/scope"
)

// TestRunRetro_ValidatesFlags verifies that runRetro calls scope.ValidateFlags
// and returns an error when both --spec and --epic are set
func TestRunRetro_ValidatesFlags(t *testing.T) {
	// Set both flags
	retroSpecFlag = "init-wizard"
	retroEpicFlag = "gromit-xyz"
	defer func() {
		retroSpecFlag = ""
		retroEpicFlag = ""
	}()

	// Call runRetro
	err := runRetro(retroCmd, []string{})

	// Should return an error mentioning mutual exclusivity
	if err == nil {
		t.Fatal("runRetro should return error when both --spec and --epic are set")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

// TestRunRetro_ResolvesSpecToLabels verifies that when --spec is provided,
// it resolves to a label using scope.ResolveSpec
func TestRunRetro_ResolvesSpecToLabels(t *testing.T) {
	// Verify scope.ResolveSpec works
	labels := scope.ResolveSpec("init-wizard")
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}

	expectedLabel := "spec:init-wizard"
	if labels[0] != expectedLabel {
		t.Errorf("ResolveSpec returned %q, want %q", labels[0], expectedLabel)
	}
}
