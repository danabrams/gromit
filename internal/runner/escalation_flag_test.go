package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
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

			// Create a mock provider that returns the same output as mockClaude
			mockProvider := &mockProviderWithRouterTracking{
				runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
					// Call the mockClaude.RunFn to get the same behavior
					claudeResult, err := mockClaude.RunFn(ctx, prompt, "haiku")
					if err != nil {
						return nil, err
					}
					return &provider.Result{
						Success:  claudeResult.Success,
						ExitCode: claudeResult.ExitCode,
						Output:   claudeResult.Output,
					}, nil
				},
			}
			mockRouter := provider.NewSingleProviderRouter(mockProvider)

			r := &Runner{
				cfg: &config.Config{
					ScopeCheck: config.ScopeCheckConfig{
						Enabled: true,
						Model:   "haiku",
					},
				},
				router:   mockRouter,
				beads:    &mockBeadClient{},
				renderer: mockRend,
				output:   &buf,
			}
			bc := &runtypes.BeadContext{
				Bead:      &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}},
				Model:     tt.startModel,
				Result:    &IterationResult{Model: tt.startModel},
				PromptCtx: &prompt.Context{Model: tt.startModel},
			}

			err := r.buildPromptForBead(context.Background(), bc, 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if bc.Model != tt.wantModel {
				t.Errorf("expected model %q, got %q", tt.wantModel, bc.Model)
			}
			if bc.Result.Escalated != tt.wantEscalated {
				t.Errorf("expected result.Escalated=%v, got %v", tt.wantEscalated, bc.Result.Escalated)
			}
			if bc.Result.EscalatedTo != tt.wantEscalatedTo {
				t.Errorf("expected result.EscalatedTo=%q, got %q", tt.wantEscalatedTo, bc.Result.EscalatedTo)
			}
		})
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

	// Create a mock provider that returns the same output as mockClaude
	mockProvider := &mockProviderWithRouterTracking{
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			// Call the mockClaude.RunFn to get the same behavior
			claudeResult, err := mockClaude.RunFn(ctx, prompt, "haiku")
			if err != nil {
				return nil, err
			}
			return &provider.Result{
				Success:  claudeResult.Success,
				ExitCode: claudeResult.ExitCode,
				Output:   claudeResult.Output,
			}, nil
		},
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg: &config.Config{
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
		},
		router:   mockRouter,
		beads:    &mockBeadClient{},
		renderer: mockRend,
		output:   &buf,
	}
	bc := &runtypes.BeadContext{
		Bead:             &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}},
		Model:            "sonnet",
		RetriesThisModel: 2, // non-zero starting value
		Result:           &IterationResult{Model: "sonnet"},
		PromptCtx:        &prompt.Context{Model: "sonnet"},
	}

	err := r.buildPromptForBead(context.Background(), bc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.Model != "opus" {
		t.Errorf("expected model 'opus', got %q", bc.Model)
	}
	if bc.RetriesThisModel != 0 {
		t.Errorf("expected retriesThisModel=0 after scope check escalation, got %d", bc.RetriesThisModel)
	}
}
