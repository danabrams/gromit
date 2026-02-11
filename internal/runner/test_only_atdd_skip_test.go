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
//
// Expected failure: processBead does not yet check bead.IsTestOnlyBead() to
// skip the ATDD phase for test-only beads. Currently, a test-only bead with
// ATDD globally enabled will enter the acceptance test phase, causing
// StreamRun to be called for acceptance tests. After implementation, it should
// skip directly to the build phase with only 1 StreamRun call.
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
					Claude:   mockClaude,
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

			result := r.processBead(context.Background(), b, 1, time.Time{})

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
//
// Expected failure: processBead does not yet detect test-only beads or log
// "Skipping ATDD: bead is test-only". After implementation, this log message
// should appear in the output buffer.
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
			Claude:   mockClaude,
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

	r.processBead(context.Background(), b, 1, time.Time{})

	output := buf.String()
	if !strings.Contains(output, "Skipping ATDD: bead is test-only") {
		t.Errorf("expected log message 'Skipping ATDD: bead is test-only' in output, got:\n%s", output)
	}
}
