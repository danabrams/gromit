package debug

import (
	"bytes"
	"errors"
	"log"
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

func TestEnforceSystemicChangeGuardrails_UsesInteractiveApprovalPrompt(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/RULES.md b/RULES.md",
		"--- a/RULES.md",
		"+++ b/RULES.md",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	called := false
	var prompt string
	err := EnforceSystemicChangeGuardrails(patch, false, func(p string) bool {
		called = true
		prompt = p
		return true
	})
	if err != nil {
		t.Fatalf("EnforceSystemicChangeGuardrails() error = %v, want nil", err)
	}
	if !called {
		t.Fatal("interactive approval prompt was not called")
	}
	lowerPrompt := strings.ToLower(prompt)
	if !strings.Contains(lowerPrompt, "process rule") {
		t.Fatalf("prompt = %q, want process rule guidance", prompt)
	}
	if !strings.Contains(prompt, "--approve") {
		t.Fatalf("prompt = %q, want --approve guidance", prompt)
	}
}

func TestEnforceSystemicChangeGuardrails_BlocksPromptFragmentYAMLWithoutApproval(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/.gromit/fragments/build.yaml b/.gromit/fragments/build.yaml",
		"--- a/.gromit/fragments/build.yaml",
		"+++ b/.gromit/fragments/build.yaml",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	err := EnforceSystemicChangeGuardrails(patch, false, nil)
	if !errors.Is(err, ErrSystemicChangeApprovalRequired) {
		t.Fatalf("EnforceSystemicChangeGuardrails() error = %v, want approval-required error", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "prompt fragment") {
		t.Fatalf("error = %q, want prompt fragment category", err.Error())
	}
}

func TestEnforceSystemicChangeGuardrails_LogsBlockedChangesForAudit(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(orig)
	})

	patch := strings.Join([]string{
		"diff --git a/.gromit/fragments/build.md b/.gromit/fragments/build.md",
		"--- a/.gromit/fragments/build.md",
		"+++ b/.gromit/fragments/build.md",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	err := EnforceSystemicChangeGuardrails(patch, false, nil)
	if !errors.Is(err, ErrSystemicChangeApprovalRequired) {
		t.Fatalf("EnforceSystemicChangeGuardrails() error = %v, want approval-required error", err)
	}

	logContents := buf.String()
	if !strings.Contains(strings.ToLower(logContents), "blocked systemic change") {
		t.Fatalf("log = %q, want audit entry for blocked systemic change", logContents)
	}
	if !strings.Contains(logContents, ".gromit/fragments/build.md") {
		t.Fatalf("log = %q, want audit entry to mention modified path", logContents)
	}
}
