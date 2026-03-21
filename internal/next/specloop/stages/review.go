package stages

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/evidence"
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
	bundler       *evidence.Bundler
	priorFindings []review.Finding
}

// NewReviewStage creates a new ReviewStage.
func NewReviewStage(runner ReviewRunner, cfg ReviewStageConfig, eventLog *runstore.EventLog) *ReviewStage {
	var bundler *evidence.Bundler
	if cfg.EvidenceDir != "" {
		bundler = evidence.NewBundler(cfg.EvidenceDir)
	}
	return &ReviewStage{
		runner:   runner,
		cfg:      cfg,
		eventLog: eventLog,
		bundler:  bundler,
	}
}

// Name returns the stage name.
func (s *ReviewStage) Name() string { return "review" }

// Run executes the review and returns Continue or ReplanFrom.
func (s *ReviewStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	// Compute diff at runtime with graceful degradation on error
	var diffSummary string
	var diffUnavailable bool
	if s.cfg.DiffProvider != nil {
		d, err := s.cfg.DiffProvider.Diff(s.cfg.BaseBranch)
		if err != nil {
			// Graceful degradation: set placeholder, emit event, continue without diff
			diffSummary = fmt.Sprintf("[diff unavailable: %v]", err)
			diffUnavailable = true
			if s.eventLog != nil {
				s.eventLog.Append(runstore.DiffUnavailableEvent{
					BaseEvent: runstore.BaseEvent{Type: "diff_unavailable", Timestamp: time.Now()},
					Reason:    err.Error(),
					Message:   "Diff provider error during review",
				})
			}
		} else {
			diffSummary = d
		}
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

	// Emit review_result event (fire-and-forget, consistent with SpecLoop.emitEvent)
	if s.eventLog != nil {
		facetSet := make(map[string]bool)
		for f := range result.FindingsByFacet {
			facetSet[f] = true
		}
		for f := range result.ErroredFacets {
			facetSet[f] = true
		}
		var facets []string
		for f := range facetSet {
			facets = append(facets, f)
		}
		sort.Strings(facets)
		var erroredFacetNames []string
		for f := range result.ErroredFacets {
			erroredFacetNames = append(erroredFacetNames, f)
		}
		sort.Strings(erroredFacetNames)
		findingsBySeverity := make(map[string]int)
		for _, f := range result.AllFindings {
			findingsBySeverity[f.Severity.String()]++
		}
		s.eventLog.Append(runstore.ReviewResultEvent{
			BaseEvent:          runstore.BaseEvent{Type: "review_result", Timestamp: time.Now()},
			TotalFindings:      len(result.AllFindings),
			BlockingFindings:   len(result.BlockingFindings),
			FindingsBySeverity: findingsBySeverity,
			FacetsReviewed:     facets,
			ErroredFacets:      erroredFacetNames,
		})
	}

	// Handle all-facets-errored case
	if result.AllFacetsErrored {
		var errMsgs []string
		for _, msg := range result.ErroredFacets {
			errMsgs = append(errMsgs, msg)
		}
		sort.Strings(errMsgs)
		return specloop.NextAction{
			Kind: specloop.Blocked,
			Context: &specloop.FailureContext{
				Failures: []string{fmt.Sprintf("all review facets failed: [%s]", strings.Join(errMsgs, ", "))},
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	// Accumulate prior findings for disposition matching across cycles,
	// deduplicating by file+description to prevent prompt bloat.
	for _, f := range result.AllFindings {
		if !findingExists(s.priorFindings, f) {
			s.priorFindings = append(s.priorFindings, f)
		}
	}

	// Store findings in RunState: all findings for evidence (Continue path),
	// but only blocking findings for the planner (ReplanFrom path, set below).
	rs.ReviewFindings = review.ReviewFailuresToStrings(result.AllFindings)

	// Write structured review.json via Bundler
	if s.bundler != nil {
		output := evidence.ReviewFindingsOutput{
			Findings:        result.FindingsByFacet,
			DiffUnavailable: diffUnavailable,
		}
		if err := s.bundler.WriteReviewFindings(output); err != nil {
			return specloop.NextAction{}, fmt.Errorf("write review findings: %w", err)
		}
	}

	if result.HasBlockingFindings {
		rs.FinalReviewPassed = false

		// Filter out review findings that contradict contract assertions.
		// A contradiction: review says "remove X from file Y" but contract
		// asserts "X must exist in file Y". Suppressing these prevents
		// infinite replan loops where reviewer and contracts disagree.
		blockingFiltered, suppressed := filterContractContradictions(
			result.BlockingFindings, s.cfg.EvidenceDir,
		)

		// If all blocking findings were contradicted by contracts, pass the review.
		if len(blockingFiltered) == 0 && suppressed > 0 {
			rs.FinalReviewPassed = true
			return specloop.NextAction{Kind: specloop.Continue}, nil
		}

		failures := review.ReviewFailuresToStrings(blockingFiltered)

		// On the ReplanFrom path, restrict ReviewFindings to blocking findings only.
		// These feed the planner's FailureContext; info/pre-existing findings are noise.
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

// findingExists returns true if a finding with the same file and description
// already exists in the slice. Used to deduplicate priorFindings across cycles.
func findingExists(findings []review.Finding, f review.Finding) bool {
	for _, existing := range findings {
		if existing.File == f.File && existing.Description == f.Description {
			return true
		}
	}
	return false
}
