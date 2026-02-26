package gromit

import (
	"os"
	"strings"
	"testing"
)

// TestCLAUDEMDDocumentsNilFieldNormalizationConvention validates that CLAUDE.md
// includes documentation of the nil-field normalization visibility convention.
func TestCLAUDEMDDocumentsNilFieldNormalizationConvention(t *testing.T) {
	data, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}
	content := string(data)

	// Check for Code Patterns section
	if !strings.Contains(content, "Code Patterns") {
		t.Error("CLAUDE.md missing 'Code Patterns' section")
	}

	// Check for nil-field normalization documentation
	if !strings.Contains(content, "NormalizeNilFields") || !strings.Contains(content, "normalizeNilFields") {
		t.Error("CLAUDE.md missing nil-field normalization convention documentation")
	}

	// Check for visibility distinction (exported vs unexported)
	if !strings.Contains(content, "Exported") || !strings.Contains(content, "unexported") {
		t.Error("CLAUDE.md missing visibility distinction in nil-field documentation")
	}
}
