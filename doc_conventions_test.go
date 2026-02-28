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

func TestReadmeDocumentsCanonicalCycleRecordContract(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "canonical cycle record contract") {
		t.Error("README.md must describe the canonical cycle record contract")
	}
	if !strings.Contains(content, "internal/visionmetrics/contract.go") {
		t.Error("README.md must reference internal/visionmetrics/contract.go as the canonical contract")
	}
}
