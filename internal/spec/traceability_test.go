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

	// Load VISION.md and RULES.md content
	visionPath := filepath.Join(repoRoot, "VISION.md")
	rulesPath := filepath.Join(repoRoot, "RULES.md")

	visionContent, err := os.ReadFile(visionPath)
	if err != nil {
		t.Fatalf("failed to load VISION.md: %v", err)
	}

	rulesContent, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("failed to load RULES.md: %v", err)
	}

	visionText := string(visionContent)
	rulesText := string(rulesContent)

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
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
				continue
			}

			// Skip meta criteria
			if strings.Contains(line, "Calling") || strings.Contains(line, "When") ||
				strings.Contains(line, "If") || strings.Contains(line, "Selecting") ||
				strings.Contains(line, "option") {
				// These are behavioral criteria, not policy criteria
				continue
			}

			// Check if criterion references VISION or RULES concepts
			traceable := false

			// Check for explicit cross-references
			if strings.Contains(line, "VISION") || strings.Contains(line, "RULES") {
				traceable = true
			}

			// Check for keyword alignment
			keywords := []string{"acceptance", "vision", "guardrail", "safety", "contract", "architecture"}
			for _, keyword := range keywords {
				if strings.Contains(strings.ToLower(line), keyword) {
					if strings.Contains(strings.ToLower(visionText), keyword) ||
						strings.Contains(strings.ToLower(rulesText), keyword) {
						traceable = true
						break
					}
				}
			}

			if !traceable && isSignificantCriterion(line) {
				t.Errorf("spec %s: criterion not traceable to VISION/RULES: %q",
					entry.Name(), line)
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

// isSignificantCriterion filters out meta criteria that don't require traceability.
func isSignificantCriterion(line string) bool {
	insignificant := []string{
		"Calling", "When", "If", "Selecting", "option",
		"shows", "displays", "prints", "exits",
		"exists", "does not exist", "can select",
	}

	for _, pattern := range insignificant {
		if strings.Contains(line, pattern) {
			return false
		}
	}

	return len(line) > 10
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
