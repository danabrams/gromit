package stages

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// ReviewRunner abstracts the review runner for testability.
type ReviewRunner interface {
	Run(ctx context.Context, input review.RunInput) (*review.RunResult, error)
}

// ReviewStageConfig configures the ReviewStage.
type ReviewStageConfig struct {
	SpecContent  string
	EvidenceDir  string
	DiffProvider review.DiffProvider
	BaseBranch   string
	DefaultTier  string
	FacetTiers   map[string]string
}

// ReviewStage runs faceted code review and decides whether findings block progress.
type ReviewStage struct {
	runner        ReviewRunner
	cfg           ReviewStageConfig
	eventLog      *runstore.EventLog
	priorFindings []review.Finding
}

// NewReviewStage creates a new ReviewStage.
func NewReviewStage(runner ReviewRunner, cfg ReviewStageConfig, eventLog *runstore.EventLog) *ReviewStage {
	return &ReviewStage{
		runner:   runner,
		cfg:      cfg,
		eventLog: eventLog,
	}
}

// Name returns the stage name.
func (s *ReviewStage) Name() string { return "review" }

// Run executes the review and returns Continue or ReplanFrom.
func (s *ReviewStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	// Compute diff at runtime
	var diffSummary string
	if s.cfg.DiffProvider != nil {
		d, err := s.cfg.DiffProvider.Diff(s.cfg.BaseBranch)
		if err != nil {
			return specloop.NextAction{}, fmt.Errorf("review diff: %w", err)
		}
		diffSummary = d
	}

	result, err := s.runner.Run(ctx, review.RunInput{
		DiffSummary:   diffSummary,
		SpecContent:   s.cfg.SpecContent,
		Cycle:         rs.Cycle,
		PriorFindings: s.priorFindings,
	})
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("review run: %w", err)
	}

	// Accumulate prior findings for disposition matching across cycles
	s.priorFindings = append(s.priorFindings, result.AllFindings...)

	if result.HasBlockingFindings {
		rs.FinalReviewPassed = false
		// Convert blocking findings to strings for planner context
		var failures []string
		for _, f := range result.BlockingFindings {
			failures = append(failures, fmt.Sprintf("review:%s: %s in %s — %s",
				f.Severity, f.Facet, f.File, f.Description))
		}
		rs.ReviewFindings = failures

		return specloop.NextAction{
			Kind: specloop.ReplanFrom,
			Context: &specloop.FailureContext{
				Failures: failures,
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	rs.FinalReviewPassed = true
	return specloop.NextAction{Kind: specloop.Continue}, nil
}
