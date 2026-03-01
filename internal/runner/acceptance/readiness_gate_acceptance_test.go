//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/readiness"
	"github.com/danabrams/gromit/internal/runner"
)

func TestReadinessGateBlocksUnlabeledBead(t *testing.T) {
	t.Parallel()

	result := runReadinessGateAcceptanceTest(t, readinessGateAcceptanceOptions{
		Bead: &bead.Bead{ID: "unlabeled-1", Title: "Unlabeled bead"},
		Assessment: readiness.Assessment{
			Status: readiness.StatusNotReady,
			Reason: "criteria_missing",
		},
	})

	if result.BuildInvoked {
		t.Fatalf("build stage ran despite readiness block")
	}
	if result.GateBlockReason != "criteria_missing" {
		t.Fatalf("GateBlockReason = %q, want %q", result.GateBlockReason, "criteria_missing")
	}
}

func TestReadinessGateBlocksSpecLabeledBead(t *testing.T) {
	t.Parallel()

	result := runReadinessGateAcceptanceTest(t, readinessGateAcceptanceOptions{
		Bead: &bead.Bead{
			ID:     "spec-1",
			Title:  "Spec bead",
			Labels: []string{"spec:payments"},
		},
		Assessment: readiness.Assessment{
			Status: readiness.StatusNotReady,
			Reason: "criteria_missing",
		},
	})

	if !result.Blocked {
		t.Fatalf("expected spec bead to be blocked by readiness gate")
	}
	if result.GateBlockReason != "criteria_missing" {
		t.Fatalf("GateBlockReason = %q, want %q", result.GateBlockReason, "criteria_missing")
	}
}

func TestReadinessGateAllowsReadyBead(t *testing.T) {
	t.Parallel()

	result := runReadinessGateAcceptanceTest(t, readinessGateAcceptanceOptions{
		Bead: &bead.Bead{ID: "ready-1", Title: "Ready bead"},
		Assessment: readiness.Assessment{
			Status: readiness.StatusReady,
		},
	})

	if result.Blocked {
		t.Fatalf("ready bead should not be blocked")
	}
	if !result.Ready {
		t.Fatalf("expected ready bead to report ready state")
	}
}

type readinessGateAcceptanceOptions struct {
	Bead       *bead.Bead
	Assessment readiness.Assessment
}

type readinessGateAcceptanceResult struct {
	BuildInvoked    bool
	GateBlockReason string
	Blocked         bool
	Ready           bool
}

func runReadinessGateAcceptanceTest(t *testing.T, opts readinessGateAcceptanceOptions) readinessGateAcceptanceResult {
	t.Helper()

	beadToRun := opts.Bead
	if beadToRun == nil {
		beadToRun = &bead.Bead{ID: "readiness-gate-test", Title: "Readiness gate test bead"}
	}

	assessment := opts.Assessment
	if assessment.Status == "" {
		assessment.Status = readiness.StatusNotReady
	}

	buildCalled := false
	gateBlockReason := ""

	firstCall := true
	mockBeads := &mockBeadClient{
		ReadyFn: func(ctx context.Context) (*bead.Bead, error) {
			if !firstCall {
				return nil, nil
			}
			firstCall = false
			return beadToRun, nil
		},
	}

	gateStage := prepare.New(io.Discard).WithReadinessAssessor(&fakeReadinessAssessor{
		assessment: assessment,
	})

	buildStage := &readinessGateTestStage{
		fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			buildCalled = true
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		},
	}

	epilogueStage := &readinessGateTestStage{
		fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			if in.Result != nil {
				gateBlockReason = in.Result.GateBlockReason
			}
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		},
	}

	cfg := runner.OrchestratorConfig{
		Gate:     gateStage,
		Build:    buildStage,
		Validate: &noopStage{},
		Epilogue: epilogueStage,
		GetBead:  mockBeads.Ready,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := runner.NewOrchestrator(cfg)
	if err := orch.Run(context.Background(), 1, time.Time{}, nil); err != nil {
		t.Fatalf("Orchestrator.Run() failed: %v", err)
	}

	return readinessGateAcceptanceResult{
		BuildInvoked:    buildCalled,
		GateBlockReason: gateBlockReason,
		Blocked:         !buildCalled,
		Ready:           buildCalled,
	}
}

type readinessGateTestStage struct {
	fn func(ctx context.Context, in pipeline.Input) (pipeline.Output, error)
}

func (s *readinessGateTestStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if s == nil || s.fn == nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}
	return s.fn(ctx, in)
}

type fakeReadinessAssessor struct {
	assessment readiness.Assessment
}

func (f *fakeReadinessAssessor) Assess(ctx context.Context, _ *bead.Bead) (readiness.Assessment, error) {
	return f.assessment, nil
}
