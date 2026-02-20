package runner

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runbook"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestShouldCaptureRunbookEntry(t *testing.T) {
	tests := []struct {
		name   string
		result *IterationResult
		want   bool
	}{
		{name: "nil", result: nil, want: false},
		{name: "success", result: &IterationResult{Success: true}, want: false},
		{name: "decomposed", result: &IterationResult{Success: false, Decomposed: true}, want: false},
		{name: "already_done", result: &IterationResult{Success: false, AlreadyDone: true}, want: false},
		{name: "exhausted_failure", result: &IterationResult{Success: false}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCaptureRunbookEntry(tt.result); got != tt.want {
				t.Fatalf("shouldCaptureRunbookEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCaptureRunbookEntry_AppendsEntryWithMappedFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Validation.FastCommands = []string{"go test ./..."}
	cfg.Escalation.Chain = []string{"haiku", "sonnet", "opus"}

	gromitDir := t.TempDir()
	r := &Runner{
		cfg:       cfg,
		gromitDir: gromitDir,
		gitHeadFn: func() (string, error) { return "failure-commit", nil },
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-123", Title: "Fix flaky test"},
		StartCommit: "start-commit",
		BuildPrompt: "build prompt body",
		PromptCtx: &prompt.Context{
			ScopedTestCommand: "go test ./internal/runner/...",
		},
	}

	result := &IterationResult{
		BeadID:                  "bead-123",
		BeadTitle:               "Fix flaky test",
		SpecID:                  "demo-spec",
		Success:                 false,
		Output:                  "model failed output",
		AcceptanceFailureOutput: "validation failed output",
		FailureCategory:         "rate_limited",
		Error:                   errors.New("build failed"),
	}

	r.captureRunbookEntry(bc, result)

	entries, err := runbook.List(gromitDir, 0)
	if err != nil {
		t.Fatalf("runbook.List() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	entry := entries[0]
	if entry.BeadID != "bead-123" {
		t.Fatalf("entry.BeadID = %q, want %q", entry.BeadID, "bead-123")
	}
	if entry.BeadTitle != "Fix flaky test" {
		t.Fatalf("entry.BeadTitle = %q, want %q", entry.BeadTitle, "Fix flaky test")
	}
	if entry.SpecID != "demo-spec" {
		t.Fatalf("entry.SpecID = %q, want %q", entry.SpecID, "demo-spec")
	}
	if entry.StartCommit != "start-commit" {
		t.Fatalf("entry.StartCommit = %q, want %q", entry.StartCommit, "start-commit")
	}
	if entry.FailureCommit != "failure-commit" {
		t.Fatalf("entry.FailureCommit = %q, want %q", entry.FailureCommit, "failure-commit")
	}
	if entry.Prompt != "build prompt body" {
		t.Fatalf("entry.Prompt = %q, want %q", entry.Prompt, "build prompt body")
	}
	if len(entry.ValidationCommands) != 1 || entry.ValidationCommands[0] != "go test ./internal/runner/..." {
		t.Fatalf("entry.ValidationCommands = %#v, want scoped command", entry.ValidationCommands)
	}
	if entry.FailureOutput != "validation failed output" {
		t.Fatalf("entry.FailureOutput = %q, want acceptance output", entry.FailureOutput)
	}
	if entry.FailureCategory != "rate_limited" {
		t.Fatalf("entry.FailureCategory = %q, want %q", entry.FailureCategory, "rate_limited")
	}
	if got, want := strings.Join(entry.EscalationChain, ","), "haiku,sonnet,opus"; got != want {
		t.Fatalf("entry.EscalationChain = %q, want %q", got, want)
	}
	if entry.Env.GoVersion != runtime.Version() {
		t.Fatalf("entry.Env.GoVersion = %q, want %q", entry.Env.GoVersion, runtime.Version())
	}
	if entry.Env.OS != runtime.GOOS {
		t.Fatalf("entry.Env.OS = %q, want %q", entry.Env.OS, runtime.GOOS)
	}
	if entry.Env.Arch != runtime.GOARCH {
		t.Fatalf("entry.Env.Arch = %q, want %q", entry.Env.Arch, runtime.GOARCH)
	}
}

func TestCaptureRunbookEntry_BestEffortOnAppendFailure(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	notADir := filepath.Join(t.TempDir(), "runbooks.jsonl")
	if err := os.WriteFile(notADir, []byte("existing-file"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	var output strings.Builder
	r := &Runner{
		cfg:       cfg,
		gromitDir: notADir,
		output:    &output,
		gitHeadFn: func() (string, error) { return "head", nil },
	}

	r.captureRunbookEntry(&runtypes.BeadContext{}, &IterationResult{
		BeadID:  "bead-1",
		Success: false,
		Error:   errors.New("failed"),
	})

	if !strings.Contains(output.String(), "Warning: failed to append runbook entry for bead bead-1") {
		t.Fatalf("expected append warning in output, got %q", output.String())
	}
}
