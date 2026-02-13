package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// TestRunValidationAppendsFailureSummary verifies that after a validation command
// fails, extractValidationSummary is called on the failure output and the result
// is appended to r.validationFailures.
func TestRunValidationAppendsFailureSummary(t *testing.T) {
	tests := []struct {
		name           string
		stdout         string
		stderr         string
		wantContains   string // expected substring in the appended summary
		wantFailureLen int    // expected length of validationFailures after call
	}{
		{
			name: "go test failure appends summary with FAIL line",
			stdout: `=== RUN   TestFoo
--- FAIL: TestFoo (0.01s)
    foo_test.go:15: expected 1, got 2
FAIL	github.com/example/pkg	0.023s`,
			stderr:         "",
			wantContains:   "--- FAIL: TestFoo",
			wantFailureLen: 1,
		},
		{
			name:   "go vet failure appends summary with diagnostic line",
			stdout: "",
			stderr: `# github.com/example/pkg
./main.go:10:6: x declared and not used`,
			wantContains:   "x declared and not used",
			wantFailureLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Validation: config.ValidationConfig{
					Enabled:  true,
					Commands: []string{"go test ./..."},
				},
				Claude: config.ClaudeConfig{
					BeadTimeout:     300,
					AnalysisTimeout: 60,
				},
			}
			cfg.SetDefaults()
			cfg.NormalizeNilFields()

			cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
				return tt.stdout, tt.stderr, 1, nil // exit code 1 = failure
			}

			var buf strings.Builder
			r := &Runner{
				cfg:              cfg,
				output:           &buf,
				analyzer:         &mockFailureAnalyzer{},
				validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
			}

			bc := &runtypes.BeadContext{
				Bead:      &bead.Bead{ID: "test-1", Title: "Test bead", Labels: []string{}, ExpectedOutputs: []string{}},
				Result:    &IterationResult{},
				PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
			}

			// runValidation should fail but append the summary
			_ = r.runValidation(context.Background(), bc)

			if len(r.validationFailures) != tt.wantFailureLen {
				t.Fatalf("validationFailures length = %d, want %d", len(r.validationFailures), tt.wantFailureLen)
			}
			if tt.wantFailureLen > 0 && !strings.Contains(r.validationFailures[0], tt.wantContains) {
				t.Errorf("validationFailures[0] = %q, want it to contain %q", r.validationFailures[0], tt.wantContains)
			}
		})
	}
}

// TestRunValidationAccumulatesMultipleFailures verifies that successive validation
// failures from different beads accumulate in r.validationFailures, not overwrite.
func TestRunValidationAccumulatesMultipleFailures(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:     300,
			AnalysisTimeout: 60,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	callCount := 0
	failureOutputs := []string{
		"--- FAIL: TestFirst (0.01s)\nFAIL\tpkg/first\t0.01s",
		"--- FAIL: TestSecond (0.02s)\nFAIL\tpkg/second\t0.02s",
	}

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		idx := callCount
		callCount++
		return failureOutputs[idx], "", 1, nil
	}

	var buf strings.Builder
	r := &Runner{
		cfg:              cfg,
		output:           &buf,
		analyzer:         &mockFailureAnalyzer{},
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}

	workDir := t.TempDir()

	// Simulate two consecutive validation failures
	for i := 0; i < 2; i++ {
		bc := &runtypes.BeadContext{
			Bead:      &bead.Bead{ID: "test-" + string(rune('1'+i)), Title: "Bead", Labels: []string{}, ExpectedOutputs: []string{}},
			Result:    &IterationResult{},
			PromptCtx: &prompt.Context{WorkDir: workDir},
		}
		_ = r.runValidation(context.Background(), bc)
	}

	if len(r.validationFailures) != 2 {
		t.Fatalf("validationFailures length = %d, want 2", len(r.validationFailures))
	}
	if !strings.Contains(r.validationFailures[0], "TestFirst") {
		t.Errorf("validationFailures[0] = %q, want it to contain 'TestFirst'", r.validationFailures[0])
	}
	if !strings.Contains(r.validationFailures[1], "TestSecond") {
		t.Errorf("validationFailures[1] = %q, want it to contain 'TestSecond'", r.validationFailures[1])
	}
}

// TestBuildPromptForBeadPopulatesRecentValidationFailures verifies that
// buildPromptForBead sets Context.RecentValidationFailures to the last 3
// entries from r.validationFailures.
func TestBuildPromptForBeadPopulatesRecentValidationFailures(t *testing.T) {
	tests := []struct {
		name      string
		failures  []string
		wantCount int
		wantFirst string // first element of the populated slice
		wantLast  string // last element of the populated slice
	}{
		{
			name:      "1 failure is passed through",
			failures:  []string{"--- FAIL: TestA"},
			wantCount: 1,
			wantFirst: "--- FAIL: TestA",
			wantLast:  "--- FAIL: TestA",
		},
		{
			name:      "3 failures are all included",
			failures:  []string{"--- FAIL: TestA", "--- FAIL: TestB", "--- FAIL: TestC"},
			wantCount: 3,
			wantFirst: "--- FAIL: TestA",
			wantLast:  "--- FAIL: TestC",
		},
		{
			name:      "5 failures capped to last 3",
			failures:  []string{"F1", "F2", "F3", "F4", "F5"},
			wantCount: 3,
			wantFirst: "F3",
			wantLast:  "F5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedCtx *prompt.Context
			renderer := &mockPromptRenderer{
				BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
					return &prompt.Context{
						Bead:               b,
						ParentBead:         parent,
						Iteration:          iteration,
						Model:              model,
						WorkDir:            t.TempDir(),
						ConfirmedLearnings: []learnings.Learning{},
						RecentLearnings:    []learnings.Learning{},
					}, nil
				},
				RenderBuildFn: func(ctx *prompt.Context) (string, error) {
					capturedCtx = ctx
					return "build prompt", nil
				},
			}

			cfg := &config.Config{
				Claude: config.ClaudeConfig{BeadTimeout: 300},
			}
			cfg.SetDefaults()
			cfg.NormalizeNilFields()

			r := &Runner{
				cfg:                cfg,
				renderer:           renderer,
				validationFailures: tt.failures,
			}

			bc := &runtypes.BeadContext{
				Bead:      &bead.Bead{ID: "test-1", Title: "Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
				Model:     "test-model",
				Iteration: 1,
			}

			err := r.buildPromptForBead(context.Background(), bc, 1)
			if err != nil {
				t.Fatalf("buildPromptForBead() error = %v", err)
			}

			if capturedCtx == nil {
				t.Fatal("RenderBuild was not called")
			}

			got := capturedCtx.RecentValidationFailures
			if len(got) != tt.wantCount {
				t.Fatalf("RecentValidationFailures length = %d, want %d; got %v", len(got), tt.wantCount, got)
			}

			if tt.wantCount > 0 {
				if got[0] != tt.wantFirst {
					t.Errorf("RecentValidationFailures[0] = %q, want %q", got[0], tt.wantFirst)
				}
				if got[len(got)-1] != tt.wantLast {
					t.Errorf("RecentValidationFailures[last] = %q, want %q", got[len(got)-1], tt.wantLast)
				}
			}
		})
	}
}

// TestRunResetsValidationFailures verifies that Runner.validationFailures is
// reset to an empty slice at the start of each Run() call, so failures from
// a previous run don't leak into the next run's build prompts.
func TestRunResetsValidationFailures(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{BeadTimeout: 300},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return nil, nil // no beads available, Run exits immediately
		},
	}

	mockLog := &mockIterationLogger{}
	var buf strings.Builder

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    beads,
		Router:   newMockRouter(),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   mockLog,
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	// Pre-populate validationFailures as if from a previous run
	r.validationFailures = []string{"old failure 1", "old failure 2"}

	// Run with no beads — should reset validationFailures and exit immediately
	err = r.Run(context.Background(), 1, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(r.validationFailures) != 0 {
		t.Errorf("validationFailures after Run() = %v, want empty slice", r.validationFailures)
	}
}
