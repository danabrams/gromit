package main

import (
	"strings"
	"testing"
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
