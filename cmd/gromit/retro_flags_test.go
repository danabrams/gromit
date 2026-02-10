package main

import (
	"testing"
)

// TestRetroCmd_HasSpecFlag verifies that the retro command has a --spec flag
func TestRetroCmd_HasSpecFlag(t *testing.T) {
	// Get the --spec flag
	specFlag := retroCmd.Flags().Lookup("spec")
	if specFlag == nil {
		t.Fatal("retro command should have --spec flag")
	}

	// Verify flag type is string
	if specFlag.Value.Type() != "string" {
		t.Errorf("--spec flag should be string type, got %s", specFlag.Value.Type())
	}
}

// TestRetroCmd_HasEpicFlag verifies that the retro command has an --epic flag
func TestRetroCmd_HasEpicFlag(t *testing.T) {
	// Get the --epic flag
	epicFlag := retroCmd.Flags().Lookup("epic")
	if epicFlag == nil {
		t.Fatal("retro command should have --epic flag")
	}

	// Verify flag type is string
	if epicFlag.Value.Type() != "string" {
		t.Errorf("--epic flag should be string type, got %s", epicFlag.Value.Type())
	}
}
