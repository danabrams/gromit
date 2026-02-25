package repohygiene

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCIWorkflowRunsBeadsIssuesPolicyGuard(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "ci.yml")

	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read CI workflow: %v", err)
	}
	if !strings.Contains(string(content), "beads-issues-policy-guard") {
		t.Fatalf("expected CI workflow to run beads issues policy guard, got:\n%s", string(content))
	}
}
