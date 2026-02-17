//go:build acceptance

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/test/testutil"
)

// smoke-matrix: keep | rationale: Retains high-value failure-path E2E coverage for strict spec validation and suggestion output. | destination: cmd/gromit/review_spec_validation_acceptance_test.go:TestCmdSmoke_ReviewSpecValidationEndToEnd
func TestCmdSmoke_ReviewSpecValidationEndToEnd(t *testing.T) {
	tmpDir := setupReviewSpecSmokeProject(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
	_, stderr, exitCode, err := testutil.RunGromitHelperProcessWithStdin(
		ctx,
		binaryPath,
		tmpDir,
		nil,
		"",
		"review", "--spec", "nonexistent-spec",
	)
	if err != nil {
		t.Fatalf("run gromit review --spec: %v", err)
	}
	if exitCode == 0 { t.Fatalf("expected non-zero exit code, got 0; stderr=%s", stderr) }
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "not found") {
		t.Fatalf("expected not-found validation error, got: %s", stderr)
	}
	if !strings.Contains(stderr, "existing-spec") {
		t.Fatalf("expected available spec suggestion in stderr, got: %s", stderr)
	}
}
