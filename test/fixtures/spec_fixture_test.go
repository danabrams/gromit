package fixtures_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// extractSpecTitle is a test-local copy of the function from cmd/gromit/refine.go
// to verify the spec fixture format
func extractSpecTitle(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	firstLine := true

	for scanner.Scan() {
		line := scanner.Text()

		// Check for frontmatter start/end on first line or after frontmatter start
		if firstLine && line == "---" {
			inFrontmatter = true
			firstLine = false
			continue
		}
		firstLine = false

		if inFrontmatter {
			if line == "---" {
				inFrontmatter = false
			}
			continue
		}

		// Look for level-1 heading
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}

	return ""
}

// TestRefineSpecFixture verifies the refine_spec.md fixture has the correct format.
func TestRefineSpecFixture(t *testing.T) {
	// Get the path to the fixture
	fixturePath := filepath.Join(".", "refine_spec.md")

	// Verify file exists
	if _, err := os.Stat(fixturePath); err != nil {
		t.Fatalf("Fixture file not found: %v", err)
	}

	// Extract the title
	title := extractSpecTitle(fixturePath)
	if title == "" {
		t.Fatal("Expected fixture to have a level-1 heading, but extractSpecTitle returned empty string")
	}

	// Verify title is "Blank Idea Title" as specified in the task
	expectedTitle := "Blank Idea Title"
	if title != expectedTitle {
		t.Errorf("Expected title %q, got %q", expectedTitle, title)
	}

	// Read the file to verify frontmatter format
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	contentStr := string(content)

	// Verify frontmatter is present
	if !strings.HasPrefix(contentStr, "---\n") {
		t.Error("Expected fixture to start with frontmatter (---)")
	}

	// Verify required frontmatter fields
	requiredFields := []string{"id:", "source_ideas:", "created:"}
	for _, field := range requiredFields {
		if !strings.Contains(contentStr, field) {
			t.Errorf("Expected frontmatter to contain %q", field)
		}
	}

	// Verify heading is present
	if !strings.Contains(contentStr, "# Blank Idea Title") {
		t.Error("Expected fixture to contain '# Blank Idea Title' heading")
	}
}
