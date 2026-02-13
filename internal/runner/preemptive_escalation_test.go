package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
)

// TestSetupBeadContext_PreemptiveEscalationFromSonnetToOpus verifies that
// when setupBeadContext receives a scopeEstimate with high complexity and
// the initial model is sonnet, it preemptively escalates to opus before
// the first invocation.
func TestSetupBeadContext_PreemptiveEscalationFromSonnetToOpus(t *testing.T) {
	// Expected failure: setupBeadContext does not check scopeEstimate.Complexity
	// or call escalateModel before returning
	var buf strings.Builder
	r := &Runner{
		cfg: &config.Config{
			Models: config.ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			},
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
			Claude: config.ClaudeConfig{
				BeadTimeout: 300,
			},
			Escalation: config.EscalationConfig{
				MaxRetriesPerModel: 2,
				MaxRetriesPerBead:  5,
			},
		},
		beads:    &mockBeadClient{},
		renderer: &mockPromptRenderer{},
		router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		analyzer: &mockFailureAnalyzer{},
		output:   &buf,
	}

	b := &bead.Bead{
		ID:       "test-1",
		Title:    "Complex Task",
		Priority: 1, // P1 = sonnet
		Labels:   []string{},
	}

	scopeEstimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          2,
		CanCompleteInSingleIteration: false,
		Rationale:                    "This task touches multiple subsystems",
		Blockers:                     []string{},
	}

	bc, beadCtx, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, scopeEstimate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify that model was escalated from sonnet to opus
	if bc.Model != "opus" {
		t.Errorf("expected model to be escalated to 'opus', got %q", bc.Model)
	}

	// Verify that escalation was tracked in the result
	if !bc.Result.Escalated {
		t.Error("expected result.Escalated to be true after preemptive escalation")
	}

	if bc.Result.EscalatedTo != "opus" {
		t.Errorf("expected result.EscalatedTo='opus', got %q", bc.Result.EscalatedTo)
	}

	// Verify that the result model was updated
	if bc.Result.Model != "opus" {
		t.Errorf("expected result.Model='opus', got %q", bc.Result.Model)
	}

	// Verify that retriesThisModel was reset
	if bc.RetriesThisModel != 0 {
		t.Errorf("expected retriesThisModel=0 after escalation, got %d", bc.RetriesThisModel)
	}

	// Verify logging indicates preemptive escalation
	if !strings.Contains(buf.String(), "preemptive") && !strings.Contains(buf.String(), "Scope check") {
		t.Error("expected log to indicate preemptive escalation based on scope check")
	}

	if beadCtx == nil {
		t.Error("beadCtx should not be nil")
	}
}

// TestSetupBeadContext_NoPreemptiveEscalationWhenComplexityMedium verifies that
// preemptive escalation does NOT occur when scope complexity is medium (not high).
func TestSetupBeadContext_NoPreemptiveEscalationWhenComplexityMedium(t *testing.T) {
	// Expected failure: setupBeadContext does not check scopeEstimate.Complexity,
	// so it cannot distinguish between high and medium complexity
	var buf strings.Builder
	r := &Runner{
		cfg: &config.Config{
			Models: config.ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			},
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
			Claude: config.ClaudeConfig{
				BeadTimeout: 300,
			},
			Escalation: config.EscalationConfig{
				MaxRetriesPerModel: 2,
				MaxRetriesPerBead:  5,
			},
		},
		beads:    &mockBeadClient{},
		renderer: &mockPromptRenderer{},
		router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		analyzer: &mockFailureAnalyzer{},
		output:   &buf,
	}

	b := &bead.Bead{
		ID:       "test-2",
		Title:    "Medium Task",
		Priority: 1, // P1 = sonnet
		Labels:   []string{},
	}

	scopeEstimate := &prompt.ScopeEstimate{
		Complexity:                   "medium",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Rationale:                    "This task is straightforward",
		Blockers:                     []string{},
	}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, scopeEstimate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify that model was NOT escalated - should remain sonnet
	if bc.Model != "sonnet" {
		t.Errorf("expected model to remain 'sonnet', got %q", bc.Model)
	}

	// Verify that escalation flags are false
	if bc.Result.Escalated {
		t.Error("expected result.Escalated to be false when complexity is medium")
	}

	if bc.Result.EscalatedTo != "" {
		t.Errorf("expected result.EscalatedTo to be empty, got %q", bc.Result.EscalatedTo)
	}
}

// TestSetupBeadContext_NoPreemptiveEscalationWhenScopeCheckDisabled verifies that
// preemptive escalation does NOT occur when scope check is disabled, even with high complexity.
func TestSetupBeadContext_NoPreemptiveEscalationWhenScopeCheckDisabled(t *testing.T) {
	// Expected failure: setupBeadContext does not check cfg.ScopeCheck.Enabled
	// before applying preemptive escalation logic
	var buf strings.Builder
	r := &Runner{
		cfg: &config.Config{
			Models: config.ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			},
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: false, // DISABLED
				Model:   "haiku",
			},
			Claude: config.ClaudeConfig{
				BeadTimeout: 300,
			},
			Escalation: config.EscalationConfig{
				MaxRetriesPerModel: 2,
				MaxRetriesPerBead:  5,
			},
		},
		beads:    &mockBeadClient{},
		renderer: &mockPromptRenderer{},
		router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		analyzer: &mockFailureAnalyzer{},
		output:   &buf,
	}

	b := &bead.Bead{
		ID:       "test-3",
		Title:    "Complex Task",
		Priority: 1, // P1 = sonnet
		Labels:   []string{},
	}

	scopeEstimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          2,
		CanCompleteInSingleIteration: false,
		Rationale:                    "This task touches multiple subsystems",
		Blockers:                     []string{},
	}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, scopeEstimate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify that model was NOT escalated when scope check is disabled
	if bc.Model != "sonnet" {
		t.Errorf("expected model to remain 'sonnet' when scope check disabled, got %q", bc.Model)
	}

	// Verify that escalation flags are false
	if bc.Result.Escalated {
		t.Error("expected result.Escalated to be false when scope check is disabled")
	}
}

// TestSetupBeadContext_NoPreemptiveEscalationFromHaiku verifies that
// preemptive escalation does NOT occur when the initial model is haiku (P2),
// even with high complexity, because haiku beads are expected to be simple.
func TestSetupBeadContext_NoPreemptiveEscalationFromHaiku(t *testing.T) {
	// Expected failure: setupBeadContext does not check the initial model
	// before applying preemptive escalation logic
	var buf strings.Builder
	r := &Runner{
		cfg: &config.Config{
			Models: config.ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			},
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
			Claude: config.ClaudeConfig{
				BeadTimeout: 300,
			},
			Escalation: config.EscalationConfig{
				MaxRetriesPerModel: 2,
				MaxRetriesPerBead:  5,
			},
		},
		beads:    &mockBeadClient{},
		renderer: &mockPromptRenderer{},
		router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		analyzer: &mockFailureAnalyzer{},
		output:   &buf,
	}

	b := &bead.Bead{
		ID:       "test-4",
		Title:    "Simple Task",
		Priority: 2, // P2 = haiku
		Labels:   []string{},
	}

	scopeEstimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Rationale:                    "False positive - actually simple",
		Blockers:                     []string{},
	}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, scopeEstimate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify that model was NOT escalated from haiku
	if bc.Model != "haiku" {
		t.Errorf("expected model to remain 'haiku', got %q", bc.Model)
	}

	// Verify that escalation flags are false
	if bc.Result.Escalated {
		t.Error("expected result.Escalated to be false when initial model is haiku")
	}
}

// TestSetupBeadContext_PreemptiveEscalationFromOpusIsNoop verifies that
// when the initial model is already opus, no escalation occurs (can't escalate
// from the highest tier).
func TestSetupBeadContext_PreemptiveEscalationFromOpusIsNoop(t *testing.T) {
	// Expected failure: setupBeadContext does not check if model is already opus
	// before attempting preemptive escalation
	var buf strings.Builder
	r := &Runner{
		cfg: &config.Config{
			Models: config.ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			},
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
			Claude: config.ClaudeConfig{
				BeadTimeout: 300,
			},
			Escalation: config.EscalationConfig{
				MaxRetriesPerModel: 2,
				MaxRetriesPerBead:  5,
			},
		},
		beads:    &mockBeadClient{},
		renderer: &mockPromptRenderer{},
		router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		analyzer: &mockFailureAnalyzer{},
		output:   &buf,
	}

	b := &bead.Bead{
		ID:       "test-5",
		Title:    "Critical Task",
		Priority: 0, // P0 = opus
		Labels:   []string{},
	}

	scopeEstimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          3,
		CanCompleteInSingleIteration: false,
		Rationale:                    "Very complex task",
		Blockers:                     []string{},
	}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, scopeEstimate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify that model remains opus (no change)
	if bc.Model != "opus" {
		t.Errorf("expected model to remain 'opus', got %q", bc.Model)
	}

	// Verify that escalation flags are NOT set (no escalation occurred)
	if bc.Result.Escalated {
		t.Error("expected result.Escalated to be false when model is already opus")
	}

	if bc.Result.EscalatedTo != "" {
		t.Errorf("expected result.EscalatedTo to be empty when no escalation occurs, got %q", bc.Result.EscalatedTo)
	}
}

// TestSetupBeadContext_NilScopeEstimateDoesNotCausePreemptiveEscalation verifies that
// when scopeEstimate is nil (no scope gate ran), preemptive escalation does not occur.
func TestSetupBeadContext_NilScopeEstimateDoesNotCausePreemptiveEscalation(t *testing.T) {
	// Expected failure: setupBeadContext does not handle nil scopeEstimate correctly
	// in the preemptive escalation logic
	var buf strings.Builder
	r := &Runner{
		cfg: &config.Config{
			Models: config.ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			},
			ScopeCheck: config.ScopeCheckConfig{
				Enabled: true,
				Model:   "haiku",
			},
			Claude: config.ClaudeConfig{
				BeadTimeout: 300,
			},
			Escalation: config.EscalationConfig{
				MaxRetriesPerModel: 2,
				MaxRetriesPerBead:  5,
			},
		},
		beads:    &mockBeadClient{},
		renderer: &mockPromptRenderer{},
		router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		analyzer: &mockFailureAnalyzer{},
		output:   &buf,
	}

	b := &bead.Bead{
		ID:       "test-6",
		Title:    "Task without scope estimate",
		Priority: 1, // P1 = sonnet
		Labels:   []string{},
	}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify that model was NOT escalated when scopeEstimate is nil
	if bc.Model != "sonnet" {
		t.Errorf("expected model to remain 'sonnet' when scopeEstimate is nil, got %q", bc.Model)
	}

	// Verify that escalation flags are false
	if bc.Result.Escalated {
		t.Error("expected result.Escalated to be false when scopeEstimate is nil")
	}
}

// TestSetupBeadContext_PreemptiveEscalationTableDriven verifies multiple scenarios
// for preemptive model escalation based on scope complexity and initial model.
func TestSetupBeadContext_PreemptiveEscalationTableDriven(t *testing.T) {
	// Expected failure: setupBeadContext does not implement preemptive escalation
	// logic, so all expectations will fail
	tests := []struct {
		name             string
		scopeEnabled     bool
		initialPriority  int
		complexity       string
		wantModel        string
		wantEscalated    bool
		wantEscalatedTo  string
		wantLogContains  string
		scopeEstimateNil bool
	}{
		{
			name:            "sonnet + high complexity → opus",
			scopeEnabled:    true,
			initialPriority: 1, // P1 = sonnet
			complexity:      "high",
			wantModel:       "opus",
			wantEscalated:   true,
			wantEscalatedTo: "opus",
			wantLogContains: "escalat",
		},
		{
			name:            "sonnet + medium complexity → sonnet (no escalation)",
			scopeEnabled:    true,
			initialPriority: 1,
			complexity:      "medium",
			wantModel:       "sonnet",
			wantEscalated:   false,
			wantEscalatedTo: "",
		},
		{
			name:            "sonnet + low complexity → sonnet (no escalation)",
			scopeEnabled:    true,
			initialPriority: 1,
			complexity:      "low",
			wantModel:       "sonnet",
			wantEscalated:   false,
			wantEscalatedTo: "",
		},
		{
			name:            "haiku + high complexity → haiku (no escalation from haiku)",
			scopeEnabled:    true,
			initialPriority: 2, // P2 = haiku
			complexity:      "high",
			wantModel:       "haiku",
			wantEscalated:   false,
			wantEscalatedTo: "",
		},
		{
			name:            "opus + high complexity → opus (already at top tier)",
			scopeEnabled:    true,
			initialPriority: 0, // P0 = opus
			complexity:      "high",
			wantModel:       "opus",
			wantEscalated:   false,
			wantEscalatedTo: "",
		},
		{
			name:            "sonnet + high complexity but scope disabled → sonnet",
			scopeEnabled:    false,
			initialPriority: 1,
			complexity:      "high",
			wantModel:       "sonnet",
			wantEscalated:   false,
			wantEscalatedTo: "",
		},
		{
			name:             "sonnet + nil scope estimate → sonnet (no escalation)",
			scopeEnabled:     true,
			initialPriority:  1,
			scopeEstimateNil: true,
			wantModel:        "sonnet",
			wantEscalated:    false,
			wantEscalatedTo:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			r := &Runner{
				cfg: &config.Config{
					Models: config.ModelsConfig{
						P0: "opus",
						P1: "sonnet",
						P2: "haiku",
					},
					ScopeCheck: config.ScopeCheckConfig{
						Enabled: tt.scopeEnabled,
						Model:   "haiku",
					},
					Claude: config.ClaudeConfig{
						BeadTimeout: 300,
					},
					Escalation: config.EscalationConfig{
						MaxRetriesPerModel: 2,
						MaxRetriesPerBead:  5,
					},
				},
				beads:    &mockBeadClient{},
				renderer: &mockPromptRenderer{},
				router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
				analyzer: &mockFailureAnalyzer{},
				output:   &buf,
			}

			b := &bead.Bead{
				ID:       "test-table-" + tt.name,
				Title:    tt.name,
				Priority: tt.initialPriority,
				Labels:   []string{},
			}

			var scopeEstimate *prompt.ScopeEstimate
			if !tt.scopeEstimateNil {
				scopeEstimate = &prompt.ScopeEstimate{
					Complexity:                   tt.complexity,
					EstimatedIterations:          1,
					CanCompleteInSingleIteration: true,
					Rationale:                    "test rationale",
					Blockers:                     []string{},
				}
			}

			bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, scopeEstimate)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer cancel()

			if bc.Model != tt.wantModel {
				t.Errorf("expected model=%q, got %q", tt.wantModel, bc.Model)
			}

			if bc.Result.Escalated != tt.wantEscalated {
				t.Errorf("expected result.Escalated=%v, got %v", tt.wantEscalated, bc.Result.Escalated)
			}

			if bc.Result.EscalatedTo != tt.wantEscalatedTo {
				t.Errorf("expected result.EscalatedTo=%q, got %q", tt.wantEscalatedTo, bc.Result.EscalatedTo)
			}

			if tt.wantLogContains != "" && !strings.Contains(strings.ToLower(buf.String()), tt.wantLogContains) {
				t.Errorf("expected log to contain %q, got: %s", tt.wantLogContains, buf.String())
			}
		})
	}
}
