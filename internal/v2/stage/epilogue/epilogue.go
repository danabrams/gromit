package epilogue

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/event"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

// Stage handles the final bookkeeping for a completed bead iteration.
type Stage struct {
	name    string
	tracker tasktracker.TaskTracker
}

// EpilogueArtifacts exposes outcome metadata for downstream consumers.
type EpilogueArtifacts struct {
	Success       bool
	FailureReason string
}

// New constructs an epilogue stage backed by the provided adapter set.
func New(cfg *config.Config, tracker tasktracker.TaskTracker) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if tracker == nil {
		return nil, fmt.Errorf("task tracker required")
	}
	return &Stage{name: stagedesc.Describe("epilogue", cfg), tracker: tracker}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the canonical stage identifier.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run executes the epilogue responsibilities for the current bead.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}

	beadID := strings.TrimSpace(req.Bead.ID)
	if beadID == "" {
		return nil, fmt.Errorf("bead ID required")
	}

	if isFailurePath(req) {
		return &stagepkg.Result{
			Decision: stagepkg.DecisionProceed,
			Artifacts: &EpilogueArtifacts{
				Success:       false,
				FailureReason: failureSummary(req),
			},
			Events: []event.TypedEvent{},
		}, nil
	}

	closeResp, err := s.tracker.CloseBead(ctx, tasktracker.CloseBeadRequest{BeadID: beadID})
	if err != nil {
		return nil, fmt.Errorf("close bead %s: %w", beadID, err)
	}
	if closeResp == nil || !closeResp.Closed {
		return nil, fmt.Errorf("close bead %s: unexpected response %#v", beadID, closeResp)
	}

	return &stagepkg.Result{
		Decision: stagepkg.DecisionProceed,
		Artifacts: &EpilogueArtifacts{
			Success: true,
		},
		Events: []event.TypedEvent{},
	}, nil
}

func failureSummary(req *stagepkg.Request) string {
	if req == nil || req.RetryContext == nil {
		return "failure"
	}
	if len(req.RetryContext.PriorFailures) == 0 {
		return fmt.Sprintf("iteration %d failed", req.Iteration)
	}
	return strings.Join(req.RetryContext.PriorFailures, "; ")
}

func isFailurePath(req *stagepkg.Request) bool {
	return req != nil && req.RetryContext != nil
}
