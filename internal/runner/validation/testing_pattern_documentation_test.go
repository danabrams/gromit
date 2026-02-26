package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationErrorPatternDocumented(t *testing.T) {
	docPath := filepath.Join("..", "..", "..", "docs", "testing-patterns.md")
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("unable to read %s: %v", docPath, err)
	}
	body := string(data)

	wants := []string{
		"TestWrapRefactorValidationError",
		"nil check",
		"message content",
		"errors.Is",
	}

	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("documentation missing %q", want)
		}
	}
}
