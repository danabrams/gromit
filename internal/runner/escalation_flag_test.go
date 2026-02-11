package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
)

// TestScopeCheckEscalation_SetsEscalatedFlag verifies that scope check
// auto-escalation routes through escalateModel() so the escalation is
// tracked in the iteration result.
func TestScopeCheckEscalation_SetsEscalatedFlag(t *testing.T) {
	tests := []struct {
		name            string
		complexity      string
		startModel      string
		wantModel       string
		wantEscalated   bool
		wantEscalatedTo string
	}{
		{
			name:            "high complexity escalates haiku and sets Escalated flag",
			complexity:      "high",
			startModel:      "haiku",
			wantModel:       "opus",
			wantEscalated:   true,
			wantEscalatedTo: "opus",
		},
		{
			name:            "high complexity escalates sonnet and sets Escalated flag",
			complexity:      "high",
			startModel:      "sonnet",
			wantModel:       "opus",
			wantEscalated:   true,
			wantEscalatedTo: "opus",
		},
		{
			name:            "medium complexity on sonnet does not escalate",
			complexity:      "medium",
			startModel:      "sonnet",
			wantModel:       "sonnet",
			wantEscalated:   false,
			wantEscalatedTo: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			mockClaude := &mockClaudeClient{
				RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
					return &claude.Result{
						Success: true,
						Output: fmt.Sprintf(
							`{"complexity":%q,"estimated_iterations":1,"can_complete_in_single_iteration":true,"blockers":[],"rationale":"test"}`,
							tt.complexity,
						),
					}, nil
				},
			}
			mockRend := &mockPromptRenderer{
				BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
					return &prompt.Context{
						Model:              model,
						ConfirmedLearnings: []learnings.Learning{},
						RecentLearnings:    []learnings.Learning{},
					}, nil
				},
			}

			r := &Runner{
				cfg: &config.Config{
					ScopeCheck: config.ScopeCheckConfig{
						Enabled: true,
						Model:   "haiku",
					},
				},
				claude:   mockClaude,
				beads:    &mockBeadClient{},
				renderer: mockRend,
				output:   &buf,
			}
			bc := &beadContext{
				bead:      &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}},
				model:     tt.startModel,
				result:    &IterationResult{Model: tt.startModel},
				promptCtx: &prompt.Context{Model: tt.startModel},
			}

			err := r.buildPromptForBead(context.Background(), bc, 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if bc.model != tt.wantModel {
				t.Errorf("expected model %q, got %q", tt.wantModel, bc.model)
			}
			if bc.result.Escalated != tt.wantEscalated {
				t.Errorf("expected result.Escalated=%v, got %v", tt.wantEscalated, bc.result.Escalated)
			}
			if bc.result.EscalatedTo != tt.wantEscalatedTo {
				t.Errorf("expected result.EscalatedTo=%q, got %q", tt.wantEscalatedTo, bc.result.EscalatedTo)
			}
		})
	}
}

// setupAcceptanceEscalation creates a Runner and beadContext configured for
// acceptance test escalation tests. The mock Claude fails the first 2 calls
// (haiku attempts) and succeeds on the 3rd (sonnet).
func setupAcceptanceEscalation(t *testing.T) (*Runner, *beadContext) {
	t.Helper()
	var buf strings.Builder
	callCount := 0
	r := &Runner{
		cfg: &config.Config{
			Claude: config.ClaudeConfig{
				StallTimeout:       30,
				StallTimeoutActive: 10,
			},
			Escalation: config.EscalationConfig{
				Enabled:            true,
				Chain:              []string{"haiku", "sonnet", "opus"},
				MaxRetriesPerModel: 1, // 1 retry = 2 attempts total per model
			},
		},
		claude: &mockClaudeClient{
			StreamRunFn: func(ctx context.Context, p string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
				callCount++
				if callCount <= 2 {
					return &claude.Result{Success: false, Output: "acceptance tests failed"}, nil
				}
				return &claude.Result{Success: true, Output: "acceptance tests passed"}, nil
			},
		},
		renderer: &mockPromptRenderer{},
		output:   &buf,
	}
	bc := &beadContext{
		bead:  &bead.Bead{ID: "test-1", Title: "Test"},
		model: "haiku",
		result: &IterationResult{
			Model: "haiku",
		},
		promptCtx: &prompt.Context{
			Model:              "haiku",
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}
	return r, bc
}

// TestAcceptanceTestEscalation_SetsEscalatedFlag verifies that when acceptance
// tests exhaust retries and escalate to a higher model, the escalation is
// tracked in the iteration result via escalateModel().
func TestAcceptanceTestEscalation_SetsEscalatedFlag(t *testing.T) {
	r, bc := setupAcceptanceEscalation(t)

	err := r.runAcceptanceTestsWithRetry(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.model != "sonnet" {
		t.Errorf("expected model to be escalated to 'sonnet', got %q", bc.model)
	}
	if bc.result.Escalated != true {
		t.Errorf("expected result.Escalated=true after acceptance test escalation, got false")
	}
	if bc.result.EscalatedTo != "sonnet" {
		t.Errorf("expected result.EscalatedTo='sonnet', got %q", bc.result.EscalatedTo)
	}
}

// TestAcceptanceTestEscalation_ResetsRetries verifies that when acceptance
// tests escalate to a higher model, retriesThisModel is reset to 0.
func TestAcceptanceTestEscalation_ResetsRetries(t *testing.T) {
	r, bc := setupAcceptanceEscalation(t)
	bc.retriesThisModel = 3 // non-zero starting value

	err := r.runAcceptanceTestsWithRetry(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.retriesThisModel != 0 {
		t.Errorf("expected retriesThisModel=0 after escalation, got %d", bc.retriesThisModel)
	}
}

// TestScopeCheckEscalation_ResetsRetries verifies that scope check
// auto-escalation resets retriesThisModel to 0 (as escalateModel() does).
func TestScopeCheckEscalation_ResetsRetries(t *testing.T) {
	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  `{"complexity":"high","estimated_iterations":1,"can_complete_in_single_iteration":true,"blockers":[],"rationale":"big task"}`,
			}, nil
		},
	}
	mockRend := &mockPromptRenderer{
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			return &prompt.Context{
				Model:              model,
				ConfirmedLearnings: []learnings.Learning{},
				RecentLearnings:    []learnings.Learning{},
			}, nil
		},
	}

	r := &Runner{
		cfg: &config.Config{
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
		},
		claude:   mockClaude,
		beads:    &mockBeadClient{},
		renderer: mockRend,
		output:   &buf,
	}
	bc := &beadContext{
		bead:             &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}},
		model:            "sonnet",
		retriesThisModel: 2, // non-zero starting value
		result:           &IterationResult{Model: "sonnet"},
		promptCtx:        &prompt.Context{Model: "sonnet"},
	}

	err := r.buildPromptForBead(context.Background(), bc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.model != "opus" {
		t.Errorf("expected model 'opus', got %q", bc.model)
	}
	if bc.retriesThisModel != 0 {
		t.Errorf("expected retriesThisModel=0 after scope check escalation, got %d", bc.retriesThisModel)
	}
}
