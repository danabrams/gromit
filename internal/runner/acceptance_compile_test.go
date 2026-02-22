package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAcceptanceTestsCompile(t *testing.T) {
	// Verify that all acceptance tests compile when the acceptance build tag
	// is active. This catches type errors, unused imports, and other
	// compilation failures that only surface with -tags acceptance.
	repoRoot := findRepoRoot(t)

	cmd := exec.Command("go", "test", "-tags", "acceptance", "-run", "^$", "./internal/runner/acceptance/")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("acceptance tests failed to compile:\n%s", out)
	}
}

func findRepoRoot(t *testing.T) string {
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
