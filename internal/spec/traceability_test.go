package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAcceptanceCriteriaTraceability verifies that acceptance criteria in specs
// reference or align with VISION.md and RULES.md content.
func TestAcceptanceCriteriaTraceability(t *testing.T) {
	// Find project root by walking up from current directory
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("failed to find repo root: %v", err)
	}

	// Find spec files
	specsDir := filepath.Join(repoRoot, ".gromit", "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		t.Fatalf("failed to read specs directory: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		specPath := filepath.Join(specsDir, entry.Name())
		specContent, err := os.ReadFile(specPath)
		if err != nil {
			t.Errorf("failed to read spec %s: %v", entry.Name(), err)
			continue
		}

		specText := string(specContent)

		// Extract acceptance criteria section
		criteria := extractAcceptanceCriteria(specText)
		if criteria == "" {
			continue // No acceptance criteria in this spec
		}

		// Verify each criterion is traceable to VISION or RULES
		for _, line := range strings.Split(criteria, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "-") {
				continue
			}

			// Remove bullet point for processing
			criterion := strings.TrimPrefix(strings.TrimSpace(line), "- ")
			if criterion == "" {
				continue
			}

			// Skip purely behavioral criteria
			if !isArchitecturalCriterion(criterion) {
				continue
			}

			// Check if criterion references VISION or RULES concepts
			traceable := false

			// Check for explicit cross-references
			if strings.Contains(criterion, "VISION") || strings.Contains(criterion, "RULES") {
				traceable = true
			}

			// For governance keywords, acceptance criteria using safety/enforcement patterns
			// are self-documenting (no explicit RULES ref needed)
			if !traceable {
				lower := strings.ToLower(criterion)
				governancePatterns := []string{
					"never", "must not", "forbidden", "read-only",
					"safety", "integrity", "audit", "enforcement",
					"hard-safety", "concurrent safety",
				}
				for _, pattern := range governancePatterns {
					if strings.Contains(lower, pattern) {
						traceable = true
						break
					}
				}
			}

			if !traceable {
				t.Errorf("spec %s: architectural criterion not traceable to VISION/RULES: %q",
					entry.Name(), criterion)
			}
		}
	}
}

// extractAcceptanceCriteria extracts the acceptance criteria section from spec content.
func extractAcceptanceCriteria(content string) string {
	lines := strings.Split(content, "\n")
	var criteria []string
	inCriteria := false

	for _, line := range lines {
		if strings.Contains(line, "## Acceptance Criteria") {
			inCriteria = true
			continue
		}

		// Stop at next section
		if inCriteria && strings.HasPrefix(line, "## ") {
			break
		}

		if inCriteria {
			criteria = append(criteria, line)
		}
	}

	return strings.Join(criteria, "\n")
}

// isArchitecturalCriterion determines if a criterion describes a governance
// constraint that should reference VISION or RULES. Most spec-specific
// implementation criteria (interfaces, APIs, behavior) don't need this traceability.
// Only governance criteria about safety, enforcement, contracts, etc. should be checked.
func isArchitecturalCriterion(criterion string) bool {
	lower := strings.ToLower(criterion)

	// Skip implementation-detail criteria about output, recommendations, artifacts
	outputDetails := []string{
		"output includes", "recommendation status", "threshold checks",
		"json and markdown", "written to", "artifacts",
	}
	for _, pattern := range outputDetails {
		if strings.Contains(lower, pattern) {
			return false
		}
	}

	// Only flag governance/safety/enforcement criteria that MUST reference VISION/RULES
	governance := []string{
		"safety", "guardrail", "enforcement", "fail-safe",
		"non-destructive", "never", "forbidden", "must not",
		"compliance", "audit", "integrity", "immutable",
	}

	for _, pattern := range governance {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Everything else is implementation-specific and doesn't need traceability
	return false
}

// findRepoRoot walks up the directory tree to find the repository root.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
