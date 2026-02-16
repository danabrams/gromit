//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

func setupReviewSpecSmokeProject(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("Rules"), 0644); err != nil {
		t.Fatalf("write RULES.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("Context"), 0644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "gromit.yaml"), []byte("paths:\n  gromit_dir: .gromit\n"), 0644); err != nil {
		t.Fatalf("write gromit.yaml: %v", err)
	}

	specBody := "---\nid: existing-spec\n---\n# Existing Spec\n"
	if err := os.WriteFile(filepath.Join(specsDir, "existing-spec.md"), []byte(specBody), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	return tmpDir
}

// smoke-matrix: keep | rationale: Retains high-value failure-path E2E coverage for strict spec validation and suggestion output. | destination: cmd/gromit/review_spec_validation_acceptance_test.go:TestCmdSmoke_ReviewSpecValidationEndToEnd
func TestCmdSmoke_ReviewSpecValidationEndToEnd(t *testing.T) {
	tmpDir := setupReviewSpecSmokeProject(t)

	_, stderr, exitCode, err := testutil.RunGromitWithStdin(
		binaryPath,
		tmpDir,
		nil,
		"",
		"review", "--spec", "nonexistent-spec",
	)
	if err != nil {
		t.Fatalf("run gromit review --spec: %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code, got 0; stderr=%s", stderr)
	}

	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "not found") {
		t.Fatalf("expected not-found validation error, got: %s", stderr)
	}
	if !strings.Contains(stderr, "existing-spec") {
		t.Fatalf("expected available spec suggestion in stderr, got: %s", stderr)
	}
}
