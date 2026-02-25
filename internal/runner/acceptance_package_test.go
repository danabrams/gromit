package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findRepoRootForDocs(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}

func TestPreparePackageHasDocumentation(t *testing.T) {
	// Verify that the prepare package has a doc.go file documenting
	// its purpose as Stage 1 of the pipeline.
	repoRoot := findRepoRootForDocs(t)
	prepareDir := filepath.Join(repoRoot, "internal", "pipeline", "prepare")
	docPath := filepath.Join(prepareDir, "doc.go")

	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("doc.go not found in prepare package: %v", err)
	}

	docContent := string(content)
	if !strings.Contains(docContent, "package prepare") {
		t.Errorf("doc.go must declare 'package prepare', got:\n%s", docContent)
	}

	if !strings.Contains(docContent, "Stage") {
		t.Errorf("doc.go should document the prepare stage package purpose")
	}
}

func TestExecutePackageHasDocumentation(t *testing.T) {
	// Verify that the execute package has a doc.go file documenting
	// its purpose as Stage 2 of the pipeline.
	repoRoot := findRepoRootForDocs(t)
	executeDir := filepath.Join(repoRoot, "internal", "pipeline", "execute")
	docPath := filepath.Join(executeDir, "doc.go")

	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("doc.go not found in execute package: %v", err)
	}

	docContent := string(content)
	if !strings.Contains(docContent, "package execute") {
		t.Errorf("doc.go must declare 'package execute', got:\n%s", docContent)
	}

	if !strings.Contains(docContent, "Stage") {
		t.Errorf("doc.go should document the execute stage package purpose")
	}
}

func TestAcceptancePackageHasDocumentation(t *testing.T) {
	// Verify that the acceptance package has a doc.go file documenting
	// its purpose as a proper Go package.
	repoRoot := findRepoRootForDocs(t)
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
