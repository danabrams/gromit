package debug

import (
	"errors"
	"strings"
	"testing"
)

func TestEnforceSystemicChangeGuardrails_RequiresApprovalForSystemicPatch(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/.gromit/fragments/build.md b/.gromit/fragments/build.md",
		"--- a/.gromit/fragments/build.md",
		"+++ b/.gromit/fragments/build.md",
		"@@ -1 +1 @@",
		"-old",
		"+new",
		"diff --git a/internal/v2/pipeline/guardrail_gate.go b/internal/v2/pipeline/guardrail_gate.go",
		"--- a/internal/v2/pipeline/guardrail_gate.go",
		"+++ b/internal/v2/pipeline/guardrail_gate.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
		"diff --git a/RULES.md b/RULES.md",
		"--- a/RULES.md",
		"+++ b/RULES.md",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	err := EnforceSystemicChangeGuardrails(patch, false, nil)
	if !errors.Is(err, ErrSystemicChangeApprovalRequired) {
		t.Fatalf("EnforceSystemicChangeGuardrails() error = %v, want approval-required error", err)
	}
	if !strings.Contains(err.Error(), "--approve") {
		t.Fatalf("error = %q, want --approve guidance", err.Error())
	}

	lower := strings.ToLower(err.Error())
	for _, category := range []string{"prompt fragment", "guard", "process rule"} {
		if !strings.Contains(lower, category) {
			t.Fatalf("error = %q, want category %q", err.Error(), category)
		}
	}
}
