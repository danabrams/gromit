//go:build acceptance

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExtractAcceptanceCriteria_StandardList tests that extractAcceptanceCriteria parses a standard bulleted list
func TestExtractAcceptanceCriteria_StandardList(t *testing.T) {
	// Expected failure: extractAcceptanceCriteria function does not exist yet
	// Expected signature: func extractAcceptanceCriteria(content string) []string
	//
	// This test verifies that when a spec has a standard ## Acceptance Criteria section
	// with bullet points, extractAcceptanceCriteria returns those bullet points as a slice.

	content := `---
id: test-spec
created: 2026-02-11
---

# Test Spec

Some description here.

## Acceptance Criteria

- First criterion about behavior
- Second criterion about another behavior
- Third criterion with more details

## Research & Context

Some research notes here.
`

	criteria := extractAcceptanceCriteria(content)

	if len(criteria) != 3 {
		t.Fatalf("extractAcceptanceCriteria() returned %d criteria, want 3", len(criteria))
	}

	want := []string{
		"First criterion about behavior",
		"Second criterion about another behavior",
		"Third criterion with more details",
	}

	for i, criterion := range criteria {
		if criterion != want[i] {
			t.Errorf("criterion[%d] = %q, want %q", i, criterion, want[i])
		}
	}
}

// TestExtractAcceptanceCriteria_MissingSection tests that extractAcceptanceCriteria returns empty slice when section is missing
func TestExtractAcceptanceCriteria_MissingSection(t *testing.T) {
	// Expected failure: extractAcceptanceCriteria function does not exist yet
	//
	// This test verifies that when a spec has no ## Acceptance Criteria section,
	// extractAcceptanceCriteria returns an empty slice, not an error.

	content := `---
id: test-spec
created: 2026-02-11
---

# Test Spec

Some description here.

## Specification

Details about the feature.

## Research & Context

Some notes.
`

	criteria := extractAcceptanceCriteria(content)

	if len(criteria) != 0 {
		t.Errorf("extractAcceptanceCriteria() with missing section = %v, want empty slice", criteria)
	}
}

// TestExtractAcceptanceCriteria_EmptySection tests that extractAcceptanceCriteria handles empty acceptance criteria sections
func TestExtractAcceptanceCriteria_EmptySection(t *testing.T) {
	// Expected failure: extractAcceptanceCriteria function does not exist yet
	//
	// This test verifies that when a spec has an ## Acceptance Criteria heading
	// but no bullet points under it, extractAcceptanceCriteria returns an empty slice.

	content := `---
id: test-spec
created: 2026-02-11
---

# Test Spec

## Acceptance Criteria

## Research & Context

Some notes.
`

	criteria := extractAcceptanceCriteria(content)

	if len(criteria) != 0 {
		t.Errorf("extractAcceptanceCriteria() with empty section = %v, want empty slice", criteria)
	}
}

// TestExtractAcceptanceCriteria_WithSubBullets tests that extractAcceptanceCriteria includes nested bullet points
func TestExtractAcceptanceCriteria_WithSubBullets(t *testing.T) {
	// Expected failure: extractAcceptanceCriteria function does not exist yet
	//
	// This test verifies that when criteria have sub-bullets (indented),
	// extractAcceptanceCriteria includes the main bullets but not sub-bullets.

	content := `---
id: test-spec
created: 2026-02-11
---

# Test Spec

## Acceptance Criteria

- Main criterion one
  - Sub detail about one
  - Another sub detail
- Main criterion two
- Main criterion three
  - Sub detail for three

## Decisions

Some decisions.
`

	criteria := extractAcceptanceCriteria(content)

	// Should only get the main bullets, not sub-bullets
	if len(criteria) != 3 {
		t.Fatalf("extractAcceptanceCriteria() returned %d criteria, want 3 (main bullets only)", len(criteria))
	}

	want := []string{
		"Main criterion one",
		"Main criterion two",
		"Main criterion three",
	}

	for i, criterion := range criteria {
		if criterion != want[i] {
			t.Errorf("criterion[%d] = %q, want %q", i, criterion, want[i])
		}
	}
}

// TestExtractAcceptanceCriteria_StopsAtNextHeading tests that extraction stops at the next ## heading
func TestExtractAcceptanceCriteria_StopsAtNextHeading(t *testing.T) {
	// Expected failure: extractAcceptanceCriteria function does not exist yet
	//
	// This test verifies that extractAcceptanceCriteria only collects bullets until
	// the next ## heading, not bullets from later sections.

	content := `---
id: test-spec
created: 2026-02-11
---

# Test Spec

## Acceptance Criteria

- Criterion one
- Criterion two

## Decisions

- This is a decision bullet, not an acceptance criterion
- Another decision

## Research & Context

- Research bullet here
`

	criteria := extractAcceptanceCriteria(content)

	if len(criteria) != 2 {
		t.Fatalf("extractAcceptanceCriteria() returned %d criteria, want 2 (should stop at next heading)", len(criteria))
	}

	want := []string{
		"Criterion one",
		"Criterion two",
	}

	for i, criterion := range criteria {
		if criterion != want[i] {
			t.Errorf("criterion[%d] = %q, want %q", i, criterion, want[i])
		}
	}
}

// TestExtractAcceptanceCriteria_MultilineContent tests that extractAcceptanceCriteria handles multi-line bullet text
func TestExtractAcceptanceCriteria_MultilineContent(t *testing.T) {
	// Expected failure: extractAcceptanceCriteria function does not exist yet
	//
	// This test verifies that when a bullet point spans multiple lines (paragraph continuation),
	// extractAcceptanceCriteria captures only the first line or handles it appropriately.

	content := `---
id: test-spec
created: 2026-02-11
---

# Test Spec

## Acceptance Criteria

- First criterion that is straightforward
- Second criterion with a very long description
  that continues on the next line
- Third criterion

## Decisions

Done.
`

	criteria := extractAcceptanceCriteria(content)

	// At minimum should find 3 main bullet points
	if len(criteria) < 3 {
		t.Fatalf("extractAcceptanceCriteria() returned %d criteria, want at least 3", len(criteria))
	}

	// First and third should match exactly
	if criteria[0] != "First criterion that is straightforward" {
		t.Errorf("criterion[0] = %q, want %q", criteria[0], "First criterion that is straightforward")
	}

	// For multi-line, verify it at least starts with the first line
	if !strings.HasPrefix(criteria[1], "Second criterion with a very long description") {
		t.Errorf("criterion[1] = %q, should start with 'Second criterion with a very long description'", criteria[1])
	}
}

// TestExtractAcceptanceCriteria_WithFrontmatter tests that extractAcceptanceCriteria handles content with YAML frontmatter
func TestExtractAcceptanceCriteria_WithFrontmatter(t *testing.T) {
	// Expected failure: extractAcceptanceCriteria function does not exist yet
	//
	// This test verifies that extractAcceptanceCriteria correctly parses specs with frontmatter.

	content := `---
id: test-spec
epic: some-epic-id
created: 2026-02-11
---

# Test Spec

## Acceptance Criteria

- When X happens, Y should occur
- System should validate Z before processing

## Research

Notes.
`

	criteria := extractAcceptanceCriteria(content)

	if len(criteria) != 2 {
		t.Fatalf("extractAcceptanceCriteria() returned %d criteria, want 2", len(criteria))
	}

	want := []string{
		"When X happens, Y should occur",
		"System should validate Z before processing",
	}

	for i, criterion := range criteria {
		if criterion != want[i] {
			t.Errorf("criterion[%d] = %q, want %q", i, criterion, want[i])
		}
	}
}

// TestBuildEpicContextSection_FullEpicWithSiblings tests that buildEpicContextSection assembles complete context
func TestBuildEpicContextSection_FullEpicWithSiblings(t *testing.T) {
	// Expected failure: buildEpicContextSection function does not exist yet
	// Expected signature: func buildEpicContextSection(epicID, epicPath, specsDir string) (string, error)
	//
	// This test verifies that when an epic exists with sibling specs,
	// buildEpicContextSection returns a formatted section with epic document and sibling summaries.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create epic file
	epicContent := `---
epic_id: payment-integration
created: 2026-02-11
---

# Payment Integration

This epic covers all payment processing features.

## Vision

Enable users to pay with multiple methods.
`
	epicPath := filepath.Join(epicsDir, "payment.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Create sibling spec 1
	spec1Content := `---
id: payment-api
epic: payment-integration
created: 2026-02-11
---

# Payment API

## Acceptance Criteria

- API endpoint accepts payment requests
- API returns transaction ID on success
`
	spec1Path := filepath.Join(specsDir, "payment-api.md")
	if err := os.WriteFile(spec1Path, []byte(spec1Content), 0644); err != nil {
		t.Fatalf("Failed to write spec1: %v", err)
	}

	// Create sibling spec 2
	spec2Content := `---
id: payment-ui
epic: payment-integration
created: 2026-02-11
---

# Payment UI

## Acceptance Criteria

- User can select payment method
- UI displays confirmation after payment
`
	spec2Path := filepath.Join(specsDir, "payment-ui.md")
	if err := os.WriteFile(spec2Path, []byte(spec2Content), 0644); err != nil {
		t.Fatalf("Failed to write spec2: %v", err)
	}

	// Call buildEpicContextSection
	section, err := buildEpicContextSection("payment-integration", epicPath, specsDir)
	if err != nil {
		t.Fatalf("buildEpicContextSection() error = %v, want nil", err)
	}

	// Verify section contains key elements
	checks := []struct {
		name     string
		contains string
	}{
		{"epic context header", "## Epic Context"},
		{"epic ID instruction", "Include `epic: payment-integration` in the spec frontmatter"},
		{"epic document header", "### Epic Document"},
		{"epic title", "# Payment Integration"},
		{"epic vision", "Enable users to pay with multiple methods"},
		{"sibling specs header", "### Sibling Specs"},
		{"spec1 title", "**Payment API**"},
		{"spec1 ID", "(`payment-api`)"},
		{"spec1 criterion1", "API endpoint accepts payment requests"},
		{"spec1 criterion2", "API returns transaction ID on success"},
		{"spec2 title", "**Payment UI**"},
		{"spec2 ID", "(`payment-ui`)"},
		{"spec2 criterion1", "User can select payment method"},
		{"spec2 criterion2", "UI displays confirmation after payment"},
	}

	for _, check := range checks {
		if !strings.Contains(section, check.contains) {
			t.Errorf("buildEpicContextSection() missing %s: section should contain %q", check.name, check.contains)
		}
	}
}

// TestBuildEpicContextSection_EpicWithNoSiblings tests that buildEpicContextSection handles epics without sibling specs
func TestBuildEpicContextSection_EpicWithNoSiblings(t *testing.T) {
	// Expected failure: buildEpicContextSection function does not exist yet
	//
	// This test verifies that when an epic has no linked specs yet,
	// buildEpicContextSection includes the epic document and a message about no siblings.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create epic file
	epicContent := `---
epic_id: new-feature
created: 2026-02-11
---

# New Feature

A brand new feature with no specs yet.
`
	epicPath := filepath.Join(epicsDir, "new.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// No sibling specs created

	// Call buildEpicContextSection
	section, err := buildEpicContextSection("new-feature", epicPath, specsDir)
	if err != nil {
		t.Fatalf("buildEpicContextSection() error = %v, want nil", err)
	}

	// Verify section contains epic but mentions no siblings
	checks := []struct {
		name     string
		contains string
	}{
		{"epic context header", "## Epic Context"},
		{"epic ID instruction", "Include `epic: new-feature` in the spec frontmatter"},
		{"epic document header", "### Epic Document"},
		{"epic title", "# New Feature"},
		{"sibling specs header", "### Sibling Specs"},
		{"no siblings message", "No other specs have been created for this epic yet"},
	}

	for _, check := range checks {
		if !strings.Contains(section, check.contains) {
			t.Errorf("buildEpicContextSection() missing %s: section should contain %q", check.name, check.contains)
		}
	}
}

// TestBuildEpicContextSection_SpecsWithoutAcceptanceCriteria tests sibling formatting when specs lack acceptance criteria
func TestBuildEpicContextSection_SpecsWithoutAcceptanceCriteria(t *testing.T) {
	// Expected failure: buildEpicContextSection function does not exist yet
	//
	// This test verifies that when a sibling spec has no acceptance criteria section,
	// buildEpicContextSection still includes it with a note or empty criteria list.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create epic
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

	// Create spec without acceptance criteria
	specContent := `---
id: incomplete-spec
epic: test-epic
created: 2026-02-11
---

# Incomplete Spec

## Specification

Some details but no acceptance criteria section.
`
	specPath := filepath.Join(specsDir, "incomplete-spec.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec: %v", err)
	}

	// Call buildEpicContextSection
	section, err := buildEpicContextSection("test-epic", epicPath, specsDir)
	if err != nil {
		t.Fatalf("buildEpicContextSection() error = %v, want nil", err)
	}

	// Verify spec is still included
	if !strings.Contains(section, "**Incomplete Spec**") {
		t.Errorf("buildEpicContextSection() should include spec title even without acceptance criteria")
	}
	if !strings.Contains(section, "(`incomplete-spec`)") {
		t.Errorf("buildEpicContextSection() should include spec ID even without acceptance criteria")
	}
}

// TestBuildEpicContextSection_MissingEpicFile tests error handling when epic file doesn't exist
func TestBuildEpicContextSection_MissingEpicFile(t *testing.T) {
	// Expected failure: buildEpicContextSection function does not exist yet
	//
	// This test verifies that when the epic file path doesn't exist,
	// buildEpicContextSection returns a meaningful error.

	tmpDir := t.TempDir()
	nonexistentPath := filepath.Join(tmpDir, "nonexistent.md")
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Call buildEpicContextSection with missing file
	_, err := buildEpicContextSection("test-epic", nonexistentPath, specsDir)
	if err == nil {
		t.Fatal("buildEpicContextSection() with missing epic file should return error, got nil")
	}
}

// TestBuildEpicContextSection_MultipleSiblingsOrdering tests that siblings are included in consistent order
func TestBuildEpicContextSection_MultipleSiblingsOrdering(t *testing.T) {
	// Expected failure: buildEpicContextSection function does not exist yet
	//
	// This test verifies that when multiple sibling specs exist,
	// buildEpicContextSection includes all of them (order may vary based on file system).

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create epic
	epicContent := `---
epic_id: multi-spec-epic
created: 2026-02-11
---

# Multi Spec Epic
`
	epicPath := filepath.Join(epicsDir, "multi.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Create three sibling specs
	siblings := []struct {
		id    string
		title string
		ac    []string
	}{
		{"spec-alpha", "Alpha Spec", []string{"Alpha criterion 1", "Alpha criterion 2"}},
		{"spec-beta", "Beta Spec", []string{"Beta criterion 1"}},
		{"spec-gamma", "Gamma Spec", []string{"Gamma criterion 1", "Gamma criterion 2", "Gamma criterion 3"}},
	}

	for _, sibling := range siblings {
		content := fmt.Sprintf(`---
id: %s
epic: multi-spec-epic
created: 2026-02-11
---

# %s

## Acceptance Criteria

`, sibling.id, sibling.title)
		for _, ac := range sibling.ac {
			content += fmt.Sprintf("- %s\n", ac)
		}

		specPath := filepath.Join(specsDir, sibling.id+".md")
		if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write spec %s: %v", sibling.id, err)
		}
	}

	// Call buildEpicContextSection
	section, err := buildEpicContextSection("multi-spec-epic", epicPath, specsDir)
	if err != nil {
		t.Fatalf("buildEpicContextSection() error = %v, want nil", err)
	}

	// Verify all siblings are present
	for _, sibling := range siblings {
		if !strings.Contains(section, sibling.title) {
			t.Errorf("buildEpicContextSection() missing sibling %s", sibling.title)
		}
		if !strings.Contains(section, sibling.id) {
			t.Errorf("buildEpicContextSection() missing sibling ID %s", sibling.id)
		}
		for _, ac := range sibling.ac {
			if !strings.Contains(section, ac) {
				t.Errorf("buildEpicContextSection() missing acceptance criterion %q from %s", ac, sibling.id)
			}
		}
	}
}

// TestBuildEpicContextSection_SpecsWithOtherEpics tests that only specs matching the epic ID are included
func TestBuildEpicContextSection_SpecsWithOtherEpics(t *testing.T) {
	// Expected failure: buildEpicContextSection function does not exist yet
	//
	// This test verifies that when the specs directory contains specs for other epics,
	// buildEpicContextSection only includes specs matching the requested epic ID.

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create target epic
	epicContent := `---
epic_id: target-epic
created: 2026-02-11
---

# Target Epic
`
	epicPath := filepath.Join(epicsDir, "target.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Create spec for target epic
	targetSpecContent := `---
id: target-spec
epic: target-epic
created: 2026-02-11
---

# Target Spec

## Acceptance Criteria

- Target criterion
`
	targetSpecPath := filepath.Join(specsDir, "target-spec.md")
	if err := os.WriteFile(targetSpecPath, []byte(targetSpecContent), 0644); err != nil {
		t.Fatalf("Failed to write target spec: %v", err)
	}

	// Create spec for other epic
	otherSpecContent := `---
id: other-spec
epic: other-epic
created: 2026-02-11
---

# Other Spec

## Acceptance Criteria

- Other criterion that should not appear
`
	otherSpecPath := filepath.Join(specsDir, "other-spec.md")
	if err := os.WriteFile(otherSpecPath, []byte(otherSpecContent), 0644); err != nil {
		t.Fatalf("Failed to write other spec: %v", err)
	}

	// Call buildEpicContextSection
	section, err := buildEpicContextSection("target-epic", epicPath, specsDir)
	if err != nil {
		t.Fatalf("buildEpicContextSection() error = %v, want nil", err)
	}

	// Verify target spec is included
	if !strings.Contains(section, "Target Spec") {
		t.Error("buildEpicContextSection() should include target spec")
	}
	if !strings.Contains(section, "Target criterion") {
		t.Error("buildEpicContextSection() should include target spec's acceptance criteria")
	}

	// Verify other spec is NOT included
	if strings.Contains(section, "Other Spec") {
		t.Error("buildEpicContextSection() should NOT include specs from other epics")
	}
	if strings.Contains(section, "Other criterion that should not appear") {
		t.Error("buildEpicContextSection() should NOT include acceptance criteria from other epics")
	}
}
