package stages

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/evidence"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// AcceptEvaluator abstracts acceptance evaluation for testability.
type AcceptEvaluator interface {
	Evaluate(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error)
}

// AcceptStageConfig configures the AcceptStage.
type AcceptStageConfig struct {
	Criteria     []string
	SpecContent  string
	EvidenceDir  string
	DiffProvider review.DiffProvider
	BaseBranch   string
	Tier         string
}

// AcceptStage evaluates acceptance criteria and decides whether to continue or replan.
type AcceptStage struct {
	evaluator AcceptEvaluator
	cfg       AcceptStageConfig
	eventLog  *runstore.EventLog
	bundler   *evidence.Bundler
}

// NewAcceptStage creates a new AcceptStage.
func NewAcceptStage(evaluator AcceptEvaluator, cfg AcceptStageConfig, eventLog *runstore.EventLog) *AcceptStage {
	var bundler *evidence.Bundler
	if cfg.EvidenceDir != "" {
		bundler = evidence.NewBundler(cfg.EvidenceDir)
	}
	return &AcceptStage{
		evaluator: evaluator,
		cfg:       cfg,
		eventLog:  eventLog,
		bundler:   bundler,
	}
}

// Name returns the stage name.
func (s *AcceptStage) Name() string { return "accept" }

// Run evaluates acceptance criteria and returns Continue or ReplanFrom.
func (s *AcceptStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	criteria := s.cfg.Criteria
	if len(criteria) == 0 {
		// Try parsing from spec content
		parsed, err := acceptor.ParseAcceptanceCriteria(s.cfg.SpecContent)
		if err != nil {
			return specloop.NextAction{
				Kind: specloop.NeedsHuman,
				Context: &specloop.FailureContext{
					Failures: []string{"spec lacks acceptance criteria section — cannot evaluate acceptance. Revise the spec to include acceptance criteria."},
					Cycle:    rs.Cycle,
				},
			}, nil
		}
		if len(parsed) == 0 {
			return specloop.NextAction{
				Kind: specloop.NeedsHuman,
				Context: &specloop.FailureContext{
					Failures: []string{"spec lacks acceptance criteria section — cannot evaluate acceptance. Revise the spec to include acceptance criteria."},
					Cycle:    rs.Cycle,
				},
			}, nil
		}
		criteria = parsed
	}

	// Compute diff at runtime (fresh on each cycle, not stale from construction time)
	var diffSummary string
	if s.cfg.DiffProvider != nil {
		d, err := s.cfg.DiffProvider.Diff(s.cfg.BaseBranch)
		if err != nil {
			return specloop.NextAction{}, fmt.Errorf("acceptance diff: %w", err)
		}
		diffSummary = d
	}

	// Populate task results summary from RunState
	var taskResults string
	for _, t := range rs.Tasks {
		if taskResults != "" {
			taskResults += "; "
		}
		taskResults += fmt.Sprintf("task %s: %s", t.TaskID, t.Status)
	}

	var validationResults string
	if rs.LastValidationResult != nil {
		validationResults = *rs.LastValidationResult
	}

	var reviewFindings string
	for _, f := range rs.ReviewFindings {
		if reviewFindings != "" {
			reviewFindings += "; "
		}
		reviewFindings += f
	}

	input := acceptor.EvaluateInput{
		Criteria:          criteria,
		DiffSummary:       diffSummary,
		TaskResults:       taskResults,
		ValidationResults: validationResults,
		ReviewFindings:    reviewFindings,
	}

	// Call evaluator with one retry on API failure
	result, err := s.evaluator.Evaluate(ctx, input)
	if err != nil {
		// Don't retry if context is canceled/expired
		if ctx.Err() != nil {
			return specloop.NextAction{}, fmt.Errorf("acceptance evaluation: %w", err)
		}
		// Retry once
		result, err = s.evaluator.Evaluate(ctx, input)
		if err != nil {
			return specloop.NextAction{}, fmt.Errorf("acceptance evaluation: %w", err)
		}
	}

	// Write structured acceptance.json via Bundler
	if s.bundler != nil {
		if err := s.bundler.WriteAcceptanceResults(result); err != nil {
			return specloop.NextAction{}, fmt.Errorf("write acceptance results: %w", err)
		}
	}

	// Emit acceptance_result event (fire-and-forget, consistent with SpecLoop.emitEvent)
	if s.eventLog != nil {
		var passCount, failCount, unclearCount int
		for _, r := range result.Results {
			switch r.Status {
			case acceptor.StatusPass:
				passCount++
			case acceptor.StatusFail:
				failCount++
			case acceptor.StatusUnclear:
				unclearCount++
			}
		}
		s.eventLog.Append(runstore.AcceptanceResultEvent{
			BaseEvent:     runstore.BaseEvent{Type: "acceptance_result", Timestamp: time.Now()},
			TotalCriteria: len(result.Results),
			PassCount:     passCount,
			FailCount:     failCount,
			UnclearCount:  unclearCount,
		})
	}

	if result.HasFailOrUnclear {
		rs.FinalAcceptancePassed = false
		failureStrs := acceptor.AcceptanceFailuresToStrings(result.Results)
		rs.AcceptanceResults = failureStrs

		return specloop.NextAction{
			Kind: specloop.ReplanFrom,
			Context: &specloop.FailureContext{
				Failures: failureStrs,
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	rs.FinalAcceptancePassed = true
	return specloop.NextAction{Kind: specloop.Continue}, nil
}
