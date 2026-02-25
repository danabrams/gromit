package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestNoAcceptanceTestsInRunnerRootTests(t *testing.T) {
	// Verify that runner_test.go does not exist in internal/runner/.
	// All test files in internal/runner/ root must be unit tests only.
	// Acceptance tests must be in internal/runner/acceptance/ only.
	runnerDir := filepath.Join(findRepoRoot(t), "internal", "runner")

	entries, err := os.ReadDir(runnerDir)
	if err != nil {
		t.Fatalf("failed to read internal/runner directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip directories
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			continue // Skip non-test files
		}

		testPath := filepath.Join(runnerDir, entry.Name())
		content, err := os.ReadFile(testPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", testPath, err)
		}

		contentStr := string(content)
		lines := strings.Split(contentStr, "\n")
		// Only check first 5 lines for build tags (standard Go convention)
		for i := 0; i < len(lines) && i < 5; i++ {
			line := strings.TrimSpace(lines[i])
			if strings.Contains(line, "//go:build acceptance") || strings.Contains(line, "// +build acceptance") {
				t.Fatalf("%s should not have acceptance build tag; acceptance tests must be in internal/runner/acceptance/", entry.Name())
			}
		}
	}

	// Specifically verify runner_test.go doesn't exist
	runnerTestPath := filepath.Join(runnerDir, "runner_test.go")
	_, err = os.Stat(runnerTestPath)
	if err == nil {
		t.Fatalf("runner_test.go should not exist at %s; acceptance tests must be isolated in internal/runner/acceptance/", runnerTestPath)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking for runner_test.go: %v", err)
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
