package main

import (
	"os"
	"strings"
	"testing"
)

// TestCLAUDEMD_TrimmedToEssentials verifies that CLAUDE.md contains only the
// header, Architecture section, and Key Principles section, and is under 1,500 chars.
func TestCLAUDEMD_TrimmedToEssentials(t *testing.T) {
	content, err := os.ReadFile("../../CLAUDE.md")
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}

	text := string(content)

	// Must be under 1,500 characters
	if len(text) > 1500 {
		t.Errorf("CLAUDE.md should be under 1,500 chars, got %d", len(text))
	}

	// Must contain header and essential sections
	if !strings.Contains(text, "# Gromit") {
		t.Error("CLAUDE.md should contain the '# Gromit' header")
	}
	if !strings.Contains(text, "## Architecture") {
		t.Error("CLAUDE.md should contain the '## Architecture' section")
	}
	if !strings.Contains(text, "## Key Principles") {
		t.Error("CLAUDE.md should contain the '## Key Principles' section")
	}

	// Must NOT contain removed sections
	removedSections := []string{
		"## Quick Start",
		"## Project Structure",
		"## Development Commands",
		"## Bead Sizing",
		"## Capturing Ideas",
		"## bd Integration",
		"## Model Selection",
		"## Configuration",
		"## Keeping Docs Current",
	}

	for _, section := range removedSections {
		if strings.Contains(text, section) {
			t.Errorf("CLAUDE.md should NOT contain removed section: %q", section)
		}
	}
}

// TestRULESMD_ProcessSection verifies that RULES.md process section is updated
// from "more than 2 files" to "6+ files across unrelated packages".
func TestRULESMD_ProcessSection(t *testing.T) {
	// Expected failure: RULES.md still says "Beads that touch more than 2 files should be split"

	content, err := os.ReadFile("../../.gromit/RULES.md")
	if err != nil {
		t.Fatalf("reading .gromit/RULES.md: %v", err)
	}

	text := string(content)

	// Verify new language is present
	if !strings.Contains(text, "6+ files across unrelated packages") {
		t.Error("RULES.md should contain updated language about '6+ files across unrelated packages'")
	}

	// Verify old language is removed
	if strings.Contains(text, "more than 2 files should be split") {
		t.Error("RULES.md should NOT contain old 'more than 2 files should be split' language")
	}

	// Verify the context about grouping is mentioned
	if !strings.Contains(text, "Interface + implementation + mock") ||
		!strings.Contains(text, "implementation + tests") {
		t.Error("RULES.md should reference grouping patterns (e.g., interface+impl+mock, implementation+tests)")
	}
}

// TestSKILLMD_SizingRulesUpdated verifies that the decompose skill's sizing rules
// are updated to reflect the behavior-based philosophy instead of file-count rules.
func TestSKILLMD_SizingRulesUpdated(t *testing.T) {
	// Expected failure: skills/gromit-decompose/SKILL.md still contains "One concern per bead", "Max 2 files touched"

	content, err := os.ReadFile("../../skills/gromit-decompose/SKILL.md")
	if err != nil {
		t.Fatalf("reading skills/gromit-decompose/SKILL.md: %v", err)
	}

	text := string(content)

	// Verify new behavior-based rules are present
	if !strings.Contains(text, "One deliverable behavior per bead") {
		t.Error("SKILL.md should contain 'One deliverable behavior per bead' instead of 'One concern per bead'")
	}

	if !strings.Contains(text, "Soft file limit of 4-5") {
		t.Error("SKILL.md should contain 'Soft file limit of 4-5' instead of 'Max 2 files touched'")
	}

	// Verify old rules are removed
	if strings.Contains(text, "Max 2 files touched") {
		t.Error("SKILL.md should NOT contain 'Max 2 files touched'")
	}

	if strings.Contains(text, "A single file or two tightly coupled files") {
		t.Error("SKILL.md should NOT contain 'A single file or two tightly coupled files'")
	}
}

// TestSKILLMD_SplittingLogicUpdated verifies that the decompose skill's
// splitting logic is updated to remove "separate beads for implementation and tests"
// and to add grouping rules.
func TestSKILLMD_SplittingLogicUpdated(t *testing.T) {
	// Expected failure: SKILL.md still says "If a task has both implementation and tests → consider separate beads"

	content, err := os.ReadFile("../../skills/gromit-decompose/SKILL.md")
	if err != nil {
		t.Fatalf("reading skills/gromit-decompose/SKILL.md: %v", err)
	}

	text := string(content)

	// Verify old splitting guidance is removed
	if strings.Contains(text, "consider separate beads (implementation first, then tests)") {
		t.Error("SKILL.md should NOT suggest separate beads for implementation and tests")
	}

	if strings.Contains(text, "If a task touches 3+ files → split by file") {
		t.Error("SKILL.md should NOT suggest splitting by file for 3+ files")
	}

	// Verify new splitting guidance is present
	if !strings.Contains(text, "6+ files across unrelated packages") {
		t.Error("SKILL.md should mention splitting at '6+ files across unrelated packages'")
	}

	// Verify grouping rules are documented
	groupingRules := []string{
		"Interface + implementation + mock",
		"Implementation + its tests",
		"Companion methods",
		"Command flags",
		"Template + its registration",
	}

	foundGroupingRules := 0
	for _, rule := range groupingRules {
		if strings.Contains(text, rule) {
			foundGroupingRules++
		}
	}

	if foundGroupingRules < 3 {
		t.Errorf("SKILL.md should document at least 3 of the 5 grouping rules (never-split patterns), found %d", foundGroupingRules)
	}
}

// TestSKILLMD_ExamplesUpdated verifies that the decompose skill's examples
// show coarser beads (e.g., interface + impl + tests as one bead).
func TestSKILLMD_ExamplesUpdated(t *testing.T) {
	// Expected failure: SKILL.md examples still show fine-grained splitting with separate implementation and test beads

	content, err := os.ReadFile("../../skills/gromit-decompose/SKILL.md")
	if err != nil {
		t.Fatalf("reading skills/gromit-decompose/SKILL.md: %v", err)
	}

	text := string(content)

	// This is a softer requirement - we're checking that examples are updated
	// to demonstrate the new philosophy. We can't check for specific example text
	// without being too prescriptive, but we can verify that the old pattern
	// of "separate test beads" isn't the primary example.

	// Count occurrences of test-related example text
	testBeadCount := strings.Count(text, "Add tests for")
	testBeadCount += strings.Count(text, "Write tests for")
	testBeadCount += strings.Count(text, "Test ")

	implPlusTestCount := strings.Count(text, "implementation + tests")
	implPlusTestCount += strings.Count(text, "Implementation + its tests")

	// If there are multiple test-only bead examples but no impl+test examples,
	// the examples haven't been updated
	if testBeadCount >= 3 && implPlusTestCount == 0 {
		t.Error("SKILL.md examples should demonstrate implementation + tests together, not separate test beads")
	}
}

// TestPROMPTDecompose_GuidelinesUpdated verifies that the decompose template's
// guidelines section reflects the new sizing philosophy and grouping rules.
func TestPROMPTDecompose_GuidelinesUpdated(t *testing.T) {
	// Expected failure: .gromit/templates/PROMPT_decompose.md guidelines still reflect old file-count philosophy

	content, err := os.ReadFile("../../.gromit/templates/PROMPT_decompose.md")
	if err != nil {
		t.Fatalf("reading .gromit/templates/PROMPT_decompose.md: %v", err)
	}

	text := string(content)

	// Verify new sizing philosophy is mentioned in guidelines
	if !strings.Contains(text, "deliverable behavior") && !strings.Contains(text, "natural implementation unit") {
		t.Error("PROMPT_decompose.md should reference 'deliverable behavior' or 'natural implementation unit' in guidelines")
	}

	// Verify grouping rules are documented
	if !strings.Contains(text, "Interface + implementation + mock") {
		t.Error("PROMPT_decompose.md should document the interface+impl+mock grouping pattern")
	}

	if !strings.Contains(text, "Implementation + its tests") && !strings.Contains(text, "implementation + tests") {
		t.Error("PROMPT_decompose.md should document that implementation and tests belong together")
	}

	// Verify the soft file limit is mentioned
	if !strings.Contains(text, "4-5") && !strings.Contains(text, "6+") {
		t.Error("PROMPT_decompose.md should mention the soft file limit (4-5 files, split at 6+)")
	}
}

// TestPROMPTDecompose_PreservesAntiOverlapGuidance verifies that the existing
// anti-overlap guidance and ATDD test-only suppression remain intact.
func TestPROMPTDecompose_PreservesAntiOverlapGuidance(t *testing.T) {
	// Expected failure: This should pass if the changes are made correctly, but we're verifying preservation

	content, err := os.ReadFile("../../.gromit/templates/PROMPT_decompose.md")
	if err != nil {
		t.Fatalf("reading .gromit/templates/PROMPT_decompose.md: %v", err)
	}

	text := string(content)

	// Verify anti-overlap guidance is preserved
	if !strings.Contains(text, "Avoiding Sibling Overlap") {
		t.Error("PROMPT_decompose.md should preserve the 'Avoiding Sibling Overlap' section")
	}

	if !strings.Contains(text, "acceptance criteria must be **unique to that task**") {
		t.Error("PROMPT_decompose.md should preserve the anti-overlap cross-check guidance")
	}

	// Verify ATDD test-only suppression is preserved
	if !strings.Contains(text, "ATDD Active") {
		t.Error("PROMPT_decompose.md should preserve the 'ATDD Active' conditional section")
	}

	if !strings.Contains(text, "No Test-Only Beads") {
		t.Error("PROMPT_decompose.md should preserve the 'No Test-Only Beads' guidance")
	}
}

// TestAllDocuments_ConsistentFileLimits verifies that documents with sizing rules
// use consistent file limit language (4-5 soft limit, 6+ for splitting).
// CLAUDE.md is excluded — it was trimmed to only Architecture and Key Principles.
func TestAllDocuments_ConsistentFileLimits(t *testing.T) {
	docs := map[string]string{
		".gromit/RULES.md":                      "../../.gromit/RULES.md",
		"skills/gromit-decompose/SKILL.md":      "../../skills/gromit-decompose/SKILL.md",
		".gromit/templates/PROMPT_decompose.md": "../../.gromit/templates/PROMPT_decompose.md",
	}

	fileCountMentions := make(map[string][]string)

	for docName, path := range docs {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		text := string(content)

		// Check for old "2 files" language
		if strings.Contains(text, "2 files") && strings.Contains(text, "more than 2 files") {
			fileCountMentions[docName] = append(fileCountMentions[docName], "OLD: references '2 files' limit")
		}

		// Check for new "4-5" or "6+" language
		hasSoftLimit := strings.Contains(text, "4-5") || strings.Contains(text, "4 or 5")
		hasSplitLimit := strings.Contains(text, "6+") || strings.Contains(text, "6 or more") || strings.Contains(text, "six or more")

		if !hasSoftLimit && !hasSplitLimit {
			fileCountMentions[docName] = append(fileCountMentions[docName], "MISSING: no mention of new file limits")
		} else {
			if hasSoftLimit {
				fileCountMentions[docName] = append(fileCountMentions[docName], "NEW: mentions 4-5 file soft limit")
			}
			if hasSplitLimit {
				fileCountMentions[docName] = append(fileCountMentions[docName], "NEW: mentions 6+ file split threshold")
			}
		}
	}

	// Report inconsistencies
	for docName, mentions := range fileCountMentions {
		for _, mention := range mentions {
			if strings.HasPrefix(mention, "OLD:") || strings.HasPrefix(mention, "MISSING:") {
				t.Errorf("%s: %s", docName, mention)
			}
		}
	}
}

// TestAllDocuments_ConsistentGroupingRules verifies that documents with splitting
// logic also document the five never-split grouping patterns.
// CLAUDE.md is excluded — it was trimmed to only Architecture and Key Principles.
func TestAllDocuments_ConsistentGroupingRules(t *testing.T) {
	docsWithSplitting := map[string]string{
		".gromit/RULES.md":                      "../../.gromit/RULES.md",
		"skills/gromit-decompose/SKILL.md":      "../../skills/gromit-decompose/SKILL.md",
		".gromit/templates/PROMPT_decompose.md": "../../.gromit/templates/PROMPT_decompose.md",
	}

	coreGroupingPatterns := []string{
		"Interface + implementation + mock",
		"Implementation + its tests",
	}

	for docName, path := range docsWithSplitting {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		text := string(content)

		// Each document should mention at least the two core grouping patterns
		missingPatterns := []string{}
		for _, pattern := range coreGroupingPatterns {
			// Allow some variation in phrasing
			normalized := strings.ToLower(strings.ReplaceAll(pattern, "+", ""))
			textLower := strings.ToLower(text)

			// Check if the pattern is present in some form
			hasInterfaceImplMock := strings.Contains(textLower, "interface") &&
				strings.Contains(textLower, "implementation") &&
				strings.Contains(textLower, "mock")
			hasImplTests := strings.Contains(textLower, "implementation") &&
				strings.Contains(textLower, "tests") &&
				(strings.Contains(textLower, "together") || strings.Contains(textLower, "same bead"))

			if strings.Contains(normalized, "interface") && !hasInterfaceImplMock {
				missingPatterns = append(missingPatterns, pattern)
			} else if strings.Contains(normalized, "implementation + its tests") && !hasImplTests {
				missingPatterns = append(missingPatterns, pattern)
			}
		}

		if len(missingPatterns) > 0 {
			t.Errorf("%s should document grouping patterns: %v", docName, missingPatterns)
		}
	}
}
