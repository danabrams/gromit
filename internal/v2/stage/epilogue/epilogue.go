package epilogue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagesdesc "github.com/danabrams/gromit/internal/v2/stages/epilogue"
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
	return &Stage{name: stagesdesc.Describe(cfg), tracker: tracker}, nil
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
			Events: []events.Event{s.failureEvent(req)},
		}, nil
	}

	if err := s.tracker.CloseBead(ctx, beadID); err != nil {
		return nil, fmt.Errorf("close bead %s: %w", beadID, err)
	}

	return &stagepkg.Result{
		Decision: stagepkg.DecisionProceed,
		Artifacts: &EpilogueArtifacts{
			Success: true,
		},
		Events: []events.Event{s.successEvent(req)},
	}, nil
}

func (s *Stage) successEvent(req *stagepkg.Request) events.Event {
	summary := req.Telemetry
	event := &events.BeadCompleteEvent{
		BeadID:       req.Bead.ID,
		Model:        req.Model,
		Duration:     0,
		InputTokens:  0,
		OutputTokens: 0,
		CostUSD:      0,
		TimeMixin:    events.TimeMixin{Time: time.Now()},
	}
	if summary != nil {
		if summary.Model != "" {
			event.Model = summary.Model
		}
		event.Duration = summary.Duration
		event.InputTokens = summary.InputTokens
		event.OutputTokens = summary.OutputTokens
		event.CostUSD = summary.CostUSD
	}
	return event
}

func (s *Stage) failureEvent(req *stagepkg.Request) events.Event {
	return &events.BeadFailedEvent{
		BeadID:    req.Bead.ID,
		Error:     failureSummary(req),
		TimeMixin: events.TimeMixin{Time: time.Now()},
	}
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

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "epilogue"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "epilogue:default"
	}
	return profile + ":" + "epilogue"
}
