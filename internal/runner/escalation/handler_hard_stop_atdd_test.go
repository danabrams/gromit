package escalation

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// Expected failure: hard-stop approval state is not tracked yet, so
// AnalyzeAndHandleFailure can still continue autonomous execution for hard-stop
// categories without explicit approval.
func TestAnalyzeAndHandleFailure_HardStopBlocksAutonomousPathWithoutApproval(t *testing.T) {
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, _ *bead.Bead, _ string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryHardStopAction,
				Recoverable: false,
				RootCause:   "bulk delete outside allowlist",
				Suggestion:  "request explicit approval before escalation",
			}, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.HardStopApproval = runtypes.HardStopApprovalState{Approved: false}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, &provider.Result{Output: "dangerous action requested"})
	if continueLoop {
		t.Fatal("expected hard-stop guardrail to halt autonomous execution when approval is missing")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected approval-required error for hard-stop action")
	}
	if !strings.Contains(strings.ToLower(bc.Result.Error.Error()), "approval") {
		t.Fatalf("expected error to mention approval requirement, got: %v", bc.Result.Error)
	}
	if !bc.Result.HardStopPendingApproval {
		t.Fatal("expected HardStopPendingApproval=true when hard-stop approval is missing")
	}
}

// Expected failure: hard-stop approval fields and guardrail routing are not
// implemented yet, so approved hard-stop actions do not follow the explicit
// escalation path.
func TestAnalyzeAndHandleFailure_HardStopApprovedCanProceedViaEscalation(t *testing.T) {
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, _ *bead.Bead, _ string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryHardStopAction,
				Recoverable: false,
				RootCause:   "credential rotation requested",
				Suggestion:  "run with explicit approval",
			}, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.HardStopApproval = runtypes.HardStopApprovalState{
		Approved:   true,
		ApprovedBy: "human-reviewer",
	}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, &provider.Result{Output: "credential mutation requested"})
	if !continueLoop {
		t.Fatal("expected approved hard-stop action to continue via escalation path")
	}
	if !bc.Result.Escalated {
		t.Fatal("expected escalation for approved hard-stop action")
	}
	if bc.Result.HardStopPendingApproval {
		t.Fatal("expected HardStopPendingApproval=false after explicit approval")
	}
}
