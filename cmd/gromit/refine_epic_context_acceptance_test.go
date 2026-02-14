//go:build acceptance

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestResolveEpicsDir_ReturnsDefaultPath tests that resolveEpicsDir returns the default epics directory path
func TestResolveEpicsDir_ReturnsDefaultPath(t *testing.T) {
	// Expected failure: resolveEpicsDir function does not exist yet
	// Expected signature: func resolveEpicsDir(cfg *config.Config) string
	//
	// This test verifies that resolveEpicsDir returns .gromit/epics as the default path.

	result := resolveEpicsDir(nil)
	expected := ".gromit/epics"
	if result != expected {
		t.Errorf("resolveEpicsDir(nil) = %q, want %q", result, expected)
	}
}

// TestResolveEpicsDir_WithEmptyConfig tests that resolveEpicsDir returns default with empty config
func TestResolveEpicsDir_WithEmptyConfig(t *testing.T) {
	// Expected failure: resolveEpicsDir function does not exist yet
	//
	// This test verifies that when config exists but has no epics path configured,
	// resolveEpicsDir returns the default path.

	cfg := &config.Config{}
	result := resolveEpicsDir(cfg)
	expected := ".gromit/epics"
	if result != expected {
		t.Errorf("resolveEpicsDir(empty config) = %q, want %q", result, expected)
	}
}

// TestDetectEpicFromContext_ExactMatch tests that detectEpicFromContext finds an exact epic ID match
func TestDetectEpicFromContext_ExactMatch(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	// Expected signature: func detectEpicFromContext(context string, epicsDir string) (string, error)
	//
	// This test verifies that when context contains an exact epic ID, detectEpicFromContext returns it.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create epic file
	epicContent := `---
epic_id: multi-interface-architecture
created: 2026-02-11
---

# Multi-Interface Architecture

Design for supporting multiple interfaces.
`
	epicPath := filepath.Join(epicsDir, "architecture.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Context with exact epic ID
	context := "multi-interface-architecture"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	if epicID != "multi-interface-architecture" {
		t.Errorf("detectEpicFromContext() = %q, want %q", epicID, "multi-interface-architecture")
	}
}

// TestDetectEpicFromContext_SubstringMatch tests that detectEpicFromContext finds epic ID as substring
func TestDetectEpicFromContext_SubstringMatch(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when context contains epic ID as part of a larger string,
	// detectEpicFromContext still finds it.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create epic file
	epicContent := `---
epic_id: payment-integration
created: 2026-02-11
---

# Payment Integration

Payment processing features.
`
	epicPath := filepath.Join(epicsDir, "payment.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Context with epic ID embedded in sentence
	context := "Part of payment-integration epic"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	if epicID != "payment-integration" {
		t.Errorf("detectEpicFromContext() = %q, want %q", epicID, "payment-integration")
	}
}

// TestDetectEpicFromContext_TruncatedContext tests that detectEpicFromContext handles truncated context strings
func TestDetectEpicFromContext_TruncatedContext(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when context is truncated mid-epic-ID (like "f multi-interface-arch"),
	// detectEpicFromContext still finds the match using the complete epic ID portion.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create epic file
	epicContent := `---
epic_id: multi-interface-architecture
created: 2026-02-11
---

# Multi-Interface Architecture
`
	epicPath := filepath.Join(epicsDir, "architecture.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Truncated context - "multi-interface-arch" is a substring of "multi-interface-architecture"
	context := "f multi-interface-arch"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	if epicID != "multi-interface-architecture" {
		t.Errorf("detectEpicFromContext() = %q, want %q", epicID, "multi-interface-architecture")
	}
}

// TestDetectEpicFromContext_NoMatch tests that detectEpicFromContext returns empty string when no epic matches
func TestDetectEpicFromContext_NoMatch(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when context doesn't contain any known epic ID,
	// detectEpicFromContext returns empty string and no error.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create epic file
	epicContent := `---
epic_id: payment-integration
created: 2026-02-11
---

# Payment Integration
`
	epicPath := filepath.Join(epicsDir, "payment.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Context with no epic ID
	context := "Some random idea text about features"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	if epicID != "" {
		t.Errorf("detectEpicFromContext() = %q, want empty string", epicID)
	}
}

// TestDetectEpicFromContext_EmptyContext tests that detectEpicFromContext handles empty context string
func TestDetectEpicFromContext_EmptyContext(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when context is empty, detectEpicFromContext returns empty string.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create epic file
	epicContent := `---
epic_id: test-epic
created: 2026-02-11
---

# Test Epic
`
	epicPath := filepath.Join(epicsDir, "test.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Empty context
	context := ""
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	if epicID != "" {
		t.Errorf("detectEpicFromContext() = %q, want empty string", epicID)
	}
}

// TestDetectEpicFromContext_EmptyEpicsDir tests that detectEpicFromContext handles empty epics directory
func TestDetectEpicFromContext_EmptyEpicsDir(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when epics directory is empty (no epic files),
	// detectEpicFromContext returns empty string.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// No epic files created

	context := "Part of some-epic-id epic"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	if epicID != "" {
		t.Errorf("detectEpicFromContext() = %q, want empty string", epicID)
	}
}

// TestDetectEpicFromContext_LongestMatchWins tests that detectEpicFromContext returns the longest matching epic ID
func TestDetectEpicFromContext_LongestMatchWins(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when multiple epic IDs match as substrings,
	// detectEpicFromContext returns the longest one to avoid false positives.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create epics with overlapping IDs
	epics := []struct {
		id       string
		filename string
	}{
		{"api", "api.md"},
		{"api-design", "api-design.md"},
		{"api-design-patterns", "api-design-patterns.md"},
	}

	for _, epic := range epics {
		content := fmt.Sprintf(`---
epic_id: %s
created: 2026-02-11
---

# Epic
`, epic.id)
		epicPath := filepath.Join(epicsDir, epic.filename)
		if err := os.WriteFile(epicPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write epic file %s: %v", epic.filename, err)
		}
	}

	// Context that matches all three IDs as substrings
	context := "Part of api-design-patterns epic"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	// Should return longest match
	if epicID != "api-design-patterns" {
		t.Errorf("detectEpicFromContext() = %q, want %q (longest match)", epicID, "api-design-patterns")
	}
}

// TestDetectEpicFromContext_InvalidFrontmatter tests that detectEpicFromContext skips epics with invalid frontmatter
func TestDetectEpicFromContext_InvalidFrontmatter(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when an epic file has invalid frontmatter,
	// detectEpicFromContext skips it and continues checking other epics.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create epic with invalid frontmatter
	invalidEpic := `---
epic_id: invalid-epic
created: [this is not valid yaml
---

# Invalid Epic
`
	invalidPath := filepath.Join(epicsDir, "invalid.md")
	if err := os.WriteFile(invalidPath, []byte(invalidEpic), 0644); err != nil {
		t.Fatalf("Failed to write invalid epic file: %v", err)
	}

	// Create valid epic
	validEpic := `---
epic_id: valid-epic
created: 2026-02-11
---

# Valid Epic
`
	validPath := filepath.Join(epicsDir, "valid.md")
	if err := os.WriteFile(validPath, []byte(validEpic), 0644); err != nil {
		t.Fatalf("Failed to write valid epic file: %v", err)
	}

	// Context referencing valid epic
	context := "Part of valid-epic"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v", err)
	}
	if epicID != "valid-epic" {
		t.Errorf("detectEpicFromContext() = %q, want %q", epicID, "valid-epic")
	}

	// Context referencing invalid epic should return empty
	context2 := "Part of invalid-epic"
	epicID2, err := detectEpicFromContext(context2, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	if epicID2 != "" {
		t.Errorf("detectEpicFromContext() with invalid frontmatter = %q, want empty string", epicID2)
	}
}

// TestDetectEpicFromContext_MissingEpicIDField tests that detectEpicFromContext skips epics without epic_id field
func TestDetectEpicFromContext_MissingEpicIDField(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when an epic file's frontmatter lacks the epic_id field,
	// detectEpicFromContext skips it.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create epic without epic_id field
	noIDEpic := `---
created: 2026-02-11
title: No ID Epic
---

# No ID Epic
`
	noIDPath := filepath.Join(epicsDir, "no-id.md")
	if err := os.WriteFile(noIDPath, []byte(noIDEpic), 0644); err != nil {
		t.Fatalf("Failed to write no-id epic file: %v", err)
	}

	// Create valid epic
	validEpic := `---
epic_id: proper-epic
created: 2026-02-11
---

# Proper Epic
`
	validPath := filepath.Join(epicsDir, "proper.md")
	if err := os.WriteFile(validPath, []byte(validEpic), 0644); err != nil {
		t.Fatalf("Failed to write valid epic file: %v", err)
	}

	// Context that would match if no-id epic had an ID
	context := "Some context text"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	// Should not find no-id epic
	if epicID != "" {
		t.Errorf("detectEpicFromContext() = %q, want empty string (epic without epic_id should be skipped)", epicID)
	}
}

// TestDetectEpicFromContext_NonStringEpicID tests that detectEpicFromContext skips epics with non-string epic_id
func TestDetectEpicFromContext_NonStringEpicID(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when an epic file's epic_id field is not a string,
	// detectEpicFromContext skips it gracefully without panicking.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create epic with numeric epic_id
	numericIDEpic := `---
epic_id: 12345
created: 2026-02-11
---

# Numeric ID Epic
`
	numericPath := filepath.Join(epicsDir, "numeric.md")
	if err := os.WriteFile(numericPath, []byte(numericIDEpic), 0644); err != nil {
		t.Fatalf("Failed to write numeric-id epic file: %v", err)
	}

	// Create valid epic
	validEpic := `---
epic_id: string-epic
created: 2026-02-11
---

# String Epic
`
	validPath := filepath.Join(epicsDir, "string.md")
	if err := os.WriteFile(validPath, []byte(validEpic), 0644); err != nil {
		t.Fatalf("Failed to write valid epic file: %v", err)
	}

	// Context referencing the numeric ID
	context := "Part of 12345 epic"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	// Should not match numeric ID
	if epicID != "" {
		t.Errorf("detectEpicFromContext() = %q, want empty string (non-string epic_id should be skipped)", epicID)
	}
}

// TestDetectEpicFromContext_MissingEpicsDirectory tests that detectEpicFromContext handles missing epics directory gracefully
func TestDetectEpicFromContext_MissingEpicsDirectory(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when the epics directory doesn't exist,
	// detectEpicFromContext returns empty string without error.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "nonexistent", "epics")
	// Don't create the directory

	context := "Part of some-epic"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	if epicID != "" {
		t.Errorf("detectEpicFromContext() with missing dir = %q, want empty string", epicID)
	}
}

// TestDetectEpicFromContext_MultipleMatches tests longest match selection with multiple overlapping IDs
func TestDetectEpicFromContext_MultipleMatches(t *testing.T) {
	// Expected failure: detectEpicFromContext function does not exist yet
	//
	// This test verifies that when context contains multiple epic IDs,
	// detectEpicFromContext returns the longest one.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}

	// Create multiple epics
	epics := []struct {
		id       string
		filename string
	}{
		{"auth", "auth.md"},
		{"user-management", "user.md"},
		{"api-versioning", "api.md"},
	}

	for _, epic := range epics {
		content := fmt.Sprintf(`---
epic_id: %s
created: 2026-02-11
---

# Epic %s
`, epic.id, epic.id)
		epicPath := filepath.Join(epicsDir, epic.filename)
		if err := os.WriteFile(epicPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write epic file %s: %v", epic.filename, err)
		}
	}

	// Context mentioning multiple epics - should return longest
	context := "This involves auth and user-management features"
	epicID, err := detectEpicFromContext(context, epicsDir)
	if err != nil {
		t.Fatalf("detectEpicFromContext() error = %v, want nil", err)
	}
	// user-management is longer than auth
	if epicID != "user-management" {
		t.Errorf("detectEpicFromContext() = %q, want %q (longest match among multiple)", epicID, "user-management")
	}
}
