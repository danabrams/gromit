package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
)

// TestProcessBead_SkipsATDDForTestOnlyBead verifies that when ATDD is globally
// enabled but the bead's title indicates it IS a test deliverable, the ATDD
// pre-pass is skipped automatically.
func TestProcessBead_SkipsATDDForTestOnlyBead(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{name: "add tests for", title: "Add tests for bead validation"},
		{name: "add unit tests for", title: "Add unit tests for config loading"},
		{name: "write tests for", title: "Write tests for runner loop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClaude := &mockClaudeClient{
				StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
					return &claude.Result{Success: true, Output: "done"}, nil
				},
			}

			var buf strings.Builder
			r, err := NewRunnerWithDeps(
				&config.Config{
					Methodology: config.MethodologyConfig{ATDD: true},
					Claude:      config.ClaudeConfig{BeadTimeout: 60},
				},
				&buf, t.TempDir(),
				Deps{
					Beads:    &mockBeadClient{},
					Router:   newMockRouterFromClaudeClient(mockClaude),
					Analyzer: &mockFailureAnalyzer{},
					Renderer: &mockPromptRenderer{},
					Logger:   &mockIterationLogger{},
				},
			)
			if err != nil {
				t.Fatalf("NewRunnerWithDeps: %v", err)
			}

			b := &bead.Bead{
				ID:              "test-only-1",
				Title:           tt.title,
				Priority:        1,
				Labels:          []string{}, // No atdd:false label — relies on heuristic
				ExpectedOutputs: []string{},
			}

			result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

			if !result.Success {
				t.Fatalf("expected success, got error: %v", result.Error)
			}

			// With ATDD skipped, only 1 StreamRun call should be made (the build).
			// If ATDD ran, there would be 2+ calls (acceptance tests + build).
			mockClaude.mu.Lock()
			callCount := len(mockClaude.StreamRunCalls)
			mockClaude.mu.Unlock()
			if callCount != 1 {
				t.Errorf("expected 1 StreamRun call (build only, ATDD skipped), got %d", callCount)
			}
		})
	}
}

// TestProcessBead_SkipsATDDForTestOnlyBead_LogsReason verifies that when ATDD
// is skipped for a test-only bead, the runner logs the reason.
func TestProcessBead_SkipsATDDForTestOnlyBead_LogsReason(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Methodology: config.MethodologyConfig{ATDD: true},
			Claude:      config.ClaudeConfig{BeadTimeout: 60},
		},
		&buf, t.TempDir(),
		Deps{
			Beads:    &mockBeadClient{},
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	b := &bead.Bead{
		ID:              "test-only-log",
		Title:           "Add tests for prompt rendering",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	r.processBead(context.Background(), b, 1, time.Time{}, nil)

	output := buf.String()
	if !strings.Contains(output, "Skipping ATDD: bead is test-only") {
		t.Errorf("expected log message 'Skipping ATDD: bead is test-only' in output, got:\n%s", output)
	}
}

// TestProcessBead_FileCreationBeadNotMarkedAlreadyDone verifies that ATDD
// no longer uses a verify-fail phase and therefore does not mark structural
// file-creation beads as already done before implementation.
func TestProcessBead_FileCreationBeadNotMarkedAlreadyDone(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
	}

	var buf strings.Builder
	noopCmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if strings.Contains(command, "-tags acceptance") {
			return "expected pre-build acceptance failure", "", 1, nil
		}
		return "VALIDATION_PASSED", "", 0, nil
	}

	r, err := NewRunnerWithDeps(
		&config.Config{
			Methodology: config.MethodologyConfig{ATDD: true},
			Claude:      config.ClaudeConfig{BeadTimeout: 60},
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"go test ./..."},
			},
		},
		&buf, t.TempDir(),
		Deps{
			Beads:     &mockBeadClient{},
			Router:    newMockRouterFromClaudeClient(mockClaude),
			Analyzer:  &mockFailureAnalyzer{},
			Renderer:  &mockPromptRenderer{},
			Logger:    &mockIterationLogger{},
			CmdRunner: noopCmdRunner,
		},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	b := &bead.Bead{
		ID:       "file-create-1",
		Title:    "Split runner.go 1/4: extract adapters.go and callbacks.go",
		Priority: 1,
		Labels:   []string{},
		// Description mentions creating files that don't exist
		Description: `1. Create internal/runner/adapters.go with these moved from runner.go:
   - routerAdapter struct
2. Create internal/runner/callbacks.go with callback methods`,
		ExpectedOutputs: []string{},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	// The bead should succeed — not be falsely marked "already done"
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if result.AlreadyDone {
		t.Error("bead should NOT be marked as already done for file-creation beads")
	}

	output := buf.String()
	// ATDD should still be active (acceptance tests written).
	if !strings.Contains(output, "ATDD enabled") {
		t.Errorf("expected ATDD to still be active, got:\n%s", output)
	}
}
