package runner

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunnerSmokeSuiteReclassified_AcceptanceFileSet(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	matches, err := filepath.Glob(filepath.Join(projectRoot, "internal/runner/*_acceptance_test.go"))
	if err != nil {
		t.Fatalf("glob runner acceptance files: %v", err)
	}

	allowedFiles := map[string]bool{
		filepath.Join(projectRoot, "internal/runner/validation_extraction_acceptance_test.go"): true,
		filepath.Join(projectRoot, "internal/runner/invocation_timeout_acceptance_test.go"):    true,
		filepath.Join(projectRoot, "internal/runner/worktree_merge_acceptance_test.go"):        true,
		filepath.Join(projectRoot, "internal/runner/runner_pipeline_acceptance_test.go"):       true,
		filepath.Join(projectRoot, "internal/runner/status_integration_acceptance_test.go"):    true,
	}

	for _, abs := range matches {
		if !allowedFiles[abs] {
			t.Fatalf("unexpected runner acceptance file in smoke suite: %s", abs)
		}
	}
}
