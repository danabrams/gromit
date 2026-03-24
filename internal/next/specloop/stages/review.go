package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Verifier     review.FindingVerifier
	WorkDir      string
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

	// Verify blocking findings and filter based on verification results.
	// Findings in the diff are kept unchanged; out-of-diff findings are
	// verified and filtered based on disposition (confirmed/downgraded/fixed).
	if s.cfg.Verifier != nil && len(result.BlockingFindings) > 0 {
		diffFiles := review.FilesInDiff(diffSummary)
		kept, verifierResults := review.VerifyBlockingFindings(ctx, result.BlockingFindings, diffFiles, s.cfg.Verifier, s.cfg.WorkDir)

		// Emit review_finding_verified events and append to audit log
		if s.eventLog != nil || s.cfg.EvidenceDir != "" {
			// Open audit file once before the loop (Fix 3 & 5: guard and open outside loop)
			var auditFile *os.File
			if s.cfg.EvidenceDir != "" {
				auditPath := filepath.Join(s.cfg.EvidenceDir, "verifier-audit.jsonl")
				var openErr error
				auditFile, openErr = os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if openErr != nil {
					fmt.Fprintf(os.Stderr, "review stage: open verifier audit log: %v\n", openErr)
				} else {
					defer auditFile.Close()
				}
			}

			for _, vr := range verifierResults {
				// Emit event
				if s.eventLog != nil {
					s.eventLog.Append(runstore.ReviewFindingVerifiedEvent{
						BaseEvent:   runstore.BaseEvent{Type: "review_finding_verified", Timestamp: time.Now()},
						File:        vr.Finding.File,
						Line:        vr.Finding.Line,
						Severity:    vr.Finding.Severity.String(),
						Description: vr.Finding.Description,
						Disposition: string(vr.Disposition),
						Reason:      vr.Reason,
					})
				}

				// Best-effort append to audit file (Fix 4: capture json.Marshal error)
				if auditFile != nil {
					entry := review.VerifierAuditEntry{
						File:        vr.Finding.File,
						Line:        vr.Finding.Line,
						Severity:    vr.Finding.Severity.String(),
						Description: vr.Finding.Description,
						Disposition: string(vr.Disposition),
						Reason:      vr.Reason,
						FileExcerpt: vr.FileExcerpt,
					}
					data, marshalErr := json.Marshal(entry)
					if marshalErr != nil {
						fmt.Fprintf(os.Stderr, "review stage: marshal verifier audit entry: %v\n", marshalErr)
					}
					if data != nil {
						fmt.Fprintf(auditFile, "%s\n", string(data))
					}
				}
			}
		}

		// Update blocking findings (Fix 1: store all kept findings, not just SeverityError).
		// HasBlockingFindings reflects whether any kept finding is still Error severity;
		// downgraded findings (Warning) are retained in BlockingFindings for evidence
		// but do not block progress.
		result.BlockingFindings = kept
		hasBlocking := false
		for _, kf := range kept {
			if kf.Severity == review.SeverityError {
				hasBlocking = true
				break
			}
		}
		result.HasBlockingFindings = hasBlocking

		// Fix 2: sync AllFindings severities to reflect any downgrading done by VerifyBlockingFindings.
		keptByKey := make(map[string]review.Finding, len(kept))
		for _, kf := range kept {
			keptByKey[kf.File+"\x00"+fmt.Sprint(kf.Line)+"\x00"+kf.Description] = kf
		}
		for i, af := range result.AllFindings {
			key := af.File + "\x00" + fmt.Sprint(af.Line) + "\x00" + af.Description
			if kf, ok := keptByKey[key]; ok && kf.Severity != af.Severity {
				result.AllFindings[i].Severity = kf.Severity
			}
		}
	}

	// Emit review_result event AFTER verification to capture post-verification counts
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
