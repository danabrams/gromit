package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestREADMEDocumentsLanesAndFixtures(t *testing.T) {
	readmePath := filepath.Join("..", "..", "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}

	checks := []string{
		"Default lane (fast)",
		"Smoke lane (real CLI)",
		"Fixture refresh workflow",
	}

	body := string(content)
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Fatalf("README.md missing %q", want)
		}
	}
}
