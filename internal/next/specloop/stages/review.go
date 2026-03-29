package stages

import (
	"bytes"
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
	runner              ReviewRunner
	cfg                 ReviewStageConfig
	eventLog            *runstore.EventLog
	bundler             *evidence.Bundler
	priorFindings       []review.Finding
	lastRunID           string // Track the last RunID to clear stale findings when run changes
	lastHydratedPayload []byte // Track last-seen payload to detect resume changes (for re-hydration)
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

	// Detect run transitions: if RunID changed, clear stale findings from prior run
	if rs.RunID != "" && s.lastRunID != "" && rs.RunID != s.lastRunID {
		s.priorFindings = nil
		s.lastHydratedPayload = nil // Also reset hydration tracker for new run
	}
	s.lastRunID = rs.RunID

	// Hydrate s.priorFindings from new/changed resume payload (one-time per payload).
	// If the payload differs from what we previously hydrated, rehydrate (handles resume changes).
	if len(rs.PriorReviewFindings) > 0 && !bytes.Equal(rs.PriorReviewFindings, s.lastHydratedPayload) {
		if prior, err := parsePriorReviewFindings(rs.PriorReviewFindings); err == nil {
			s.priorFindings = prior
			s.lastHydratedPayload = rs.PriorReviewFindings
		} else {
			fmt.Fprintf(os.Stderr, "warning: failed to parse prior review findings: %v\n", err)
		}
	}

	// Compute runner input from RunState payload semantics (AC5):
	// - If payload is non-empty (resumed run): hydrated s.priorFindings
	// - If payload is empty (fresh run): zero-value (nil) to prevent stale carryover
	// Stage-local s.priorFindings accumulates for deduplication, but only payload drives runner input.
	var runnerPriorFindings []review.Finding
	if len(rs.PriorReviewFindings) > 0 {
		runnerPriorFindings = s.priorFindings
	} else {
		runnerPriorFindings = nil
	}

	result, err := s.runner.Run(ctx, review.RunInput{
		DiffSummary:   diffSummary,
		SpecContent:   s.cfg.SpecContent,
		Cycle:         rs.Cycle,
		PriorFindings: runnerPriorFindings,
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
			rs.ReviewThrashCounts = map[string]int{}
			return specloop.NextAction{Kind: specloop.Continue}, nil
		}

		failures := review.ReviewFailuresToStrings(blockingFiltered)

		// On the ReplanFrom path, restrict ReviewFindings to blocking findings only.
		// These feed the planner's FailureContext; info/pre-existing findings are noise.
		rs.ReviewFindings = failures

		prevCounts := rs.ReviewThrashCounts
		newCounts := make(map[string]int, len(blockingFiltered))
		var escalated []reviewThrashRecord
		var blockedFindings []reviewThrashRecord
		for i, f := range blockingFiltered {
			if f.Severity != review.SeverityError {
				continue
			}
			fp := thrashFingerprint(f)
			count := prevCounts[fp] + 1
			newCounts[fp] = count
			if count >= 3 {
				blockedFindings = append(blockedFindings, reviewThrashRecord{
					failure:     failures[i],
					file:        f.File,
					description: f.Description,
					count:       count,
				})
			} else if count == 2 {
				escalated = append(escalated, reviewThrashRecord{
					failure:     failures[i],
					file:        f.File,
					description: f.Description,
					count:       count,
				})
			}
		}
		rs.ReviewThrashCounts = newCounts

		if len(blockedFindings) > 0 {
			blockedFailures := make([]string, 0, len(blockedFindings))
			for _, rec := range blockedFindings {
				blockedFailures = append(blockedFailures, rec.failure)
			}
			return specloop.NextAction{
				Kind: specloop.Blocked,
				Context: &specloop.FailureContext{
					Failures: blockedFailures,
					Cycle:    rs.Cycle,
				},
			}, nil
		}

		if len(escalated) > 0 {
			if s.eventLog != nil {
				for _, rec := range escalated {
					s.eventLog.Append(runstore.ReviewThrashEscalatedEvent{
						BaseEvent:          runstore.BaseEvent{Type: "review_thrash_escalated", Timestamp: time.Now()},
						FindingFile:        rec.file,
						FindingDescription: rec.description,
						ConsecutiveCount:   rec.count,
					})
				}
			}
			escalatedFailures := make([]string, 0, len(escalated))
			for _, rec := range escalated {
				escalatedFailures = append(escalatedFailures, rec.failure)
			}
			return specloop.NextAction{
				Kind: specloop.ReplanFrom,
				Context: &specloop.FailureContext{
					Failures:          failures,
					Cycle:             rs.Cycle,
					EscalatedFailures: escalatedFailures,
				},
			}, nil
		}

		return specloop.NextAction{
			Kind: specloop.ReplanFrom,
			Context: &specloop.FailureContext{
				Failures: failures,
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	rs.FinalReviewPassed = true
	rs.ReviewThrashCounts = map[string]int{}
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

type reviewThrashRecord struct {
	failure     string
	file        string
	description string
	count       int
}

func thrashFingerprint(f review.Finding) string {
	return f.File + "\x00" + f.Description
}

func parsePriorReviewFindings(data json.RawMessage) ([]review.Finding, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var findings []review.Finding
	for key, payload := range raw {
		if key == "diff_unavailable" {
			continue
		}
		var facetFindings []review.Finding
		if err := json.Unmarshal(payload, &facetFindings); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping malformed prior findings for facet %q: %v\n", key, err)
			continue
		}
		findings = append(findings, facetFindings...)
	}
	return findings, nil
}
