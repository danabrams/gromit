package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcceptancePackageHasDocumentation(t *testing.T) {
	// Verify that the acceptance package has a doc.go file documenting
	// its purpose as a proper Go package.
	repoRoot := findRepoRoot(t)
	acceptanceDir := filepath.Join(repoRoot, "internal", "runner", "acceptance")
	docPath := filepath.Join(acceptanceDir, "doc.go")

	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("doc.go not found in acceptance package: %v", err)
	}

	docContent := string(content)
	if !strings.Contains(docContent, "package acceptance") {
		t.Errorf("doc.go must declare 'package acceptance', got:\n%s", docContent)
	}

	if !strings.Contains(docContent, "acceptance") || !strings.Contains(docContent, "test") {
		t.Errorf("doc.go should document the acceptance test package purpose")
	}
}
