package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/danabrams/gromit/internal/next/evidence"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// EvidenceStageConfig configures the EvidenceStage.
type EvidenceStageConfig struct {
	DiffSummary string
	StartTime   time.Time
}

// EvidenceStage assembles the evidence bundle for a run.
type EvidenceStage struct {
	store *runstore.Store
	cfg   EvidenceStageConfig
}

// NewEvidenceStage creates a new EvidenceStage.
func NewEvidenceStage(store *runstore.Store, cfg EvidenceStageConfig) *EvidenceStage {
	return &EvidenceStage{store: store, cfg: cfg}
}

// Name returns the stage name.
func (s *EvidenceStage) Name() string { return "evidence" }

// Run assembles the evidence bundle.
func (s *EvidenceStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	bundler := evidence.NewBundler(s.store.RunEvidenceDir(rs.RunID))
	if err := bundler.Init(); err != nil {
		return specloop.NextAction{}, fmt.Errorf("init evidence bundler: %w", err)
	}

	if err := bundler.WriteTaskResults(rs.Tasks); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write task results: %w", err)
	}

	// Build validation result from RunState (read at execution time, not statically configured)
	validationResult := validator.FinalResult{Pass: rs.FinalValidationPassed}
	if err := bundler.WriteValidation(validationResult); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write validation: %w", err)
	}

	// Compute metrics
	passCount := 0
	failCount := 0
	totalTokens := 0
	for _, t := range rs.Tasks {
		totalTokens += t.TokensUsed
		if t.Status == "done" {
			passCount++
		} else if t.Status == "failed" {
			failCount++
		}
	}

	durationMs := int64(0)
	if !s.cfg.StartTime.IsZero() {
		durationMs = time.Since(s.cfg.StartTime).Milliseconds()
	}

	metrics := evidence.Metrics{
		TotalTokens:  totalTokens,
		TotalCostUSD: rs.AccumulatedCost,
		TotalTasks:   len(rs.Tasks),
		PassedTasks:  passCount,
		FailedTasks:  failCount,
		DurationMs:   durationMs,
		Cycles:       rs.Cycle,
		Invocations:  []evidence.InvocationRecord{},
	}
	if err := bundler.WriteMetrics(metrics); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write metrics: %w", err)
	}

	if err := bundler.WriteDiffSummary(s.cfg.DiffSummary); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write diff summary: %w", err)
	}

	summary := evidence.SummaryInput{
		SpecID:    rs.SpecID,
		Status:    rs.Status,
		TaskCount: len(rs.Tasks),
		PassCount: passCount,
		Cycles:    rs.Cycle,
	}
	if err := bundler.WriteSummary(summary); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write summary: %w", err)
	}

	// Read review.json and acceptance.json from disk (written by ReviewStage/AcceptStage)
	reviewFindings, acceptanceCriteria := s.readReviewEvidence(rs.RunID)

	reviewInput := evidence.ReviewInput{
		TerminalState:      rs.Status,
		WhatChanged:        s.cfg.DiffSummary,
		CycleHistory:       []evidence.CycleRecord{{Cycle: rs.Cycle, TaskCount: len(rs.Tasks), PassCount: passCount}},
		ValidationResults:  fmt.Sprintf("pass=%v", rs.FinalValidationPassed),
		KnownRisks:         []string{},
		RecommendedAction:  "review",
		ReviewFindings:     reviewFindings,
		AcceptanceCriteria: acceptanceCriteria,
	}
	if err := bundler.WriteReview(reviewInput); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write review: %w", err)
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}

// readReviewEvidence reads review.json and acceptance.json from disk and converts
// them to summary types for the review decision sheet. Missing files produce
// "Not evaluated" sentinel entries for backward compatibility with 0002a runs.
func (s *EvidenceStage) readReviewEvidence(runID string) ([]evidence.ReviewFindingSummary, []evidence.AcceptanceCriterionSummary) {
	evidenceDir := s.store.RunEvidenceDir(runID)

	reviewFindings := s.readReviewFindings(filepath.Join(evidenceDir, "review.json"))
	acceptanceCriteria := s.readAcceptanceCriteria(filepath.Join(evidenceDir, "acceptance.json"))

	return reviewFindings, acceptanceCriteria
}

func (s *EvidenceStage) readReviewFindings(path string) []evidence.ReviewFindingSummary {
	data, err := os.ReadFile(path)
	if err != nil {
		return []evidence.ReviewFindingSummary{
			{Facet: "review", Count: 0, Severities: "Not evaluated"},
		}
	}

	var facetFindings map[string][]review.Finding
	if err := json.Unmarshal(data, &facetFindings); err != nil {
		return []evidence.ReviewFindingSummary{
			{Facet: "review", Count: 0, Severities: "Not evaluated"},
		}
	}

	// Collect facet keys and sort for deterministic output
	facetKeys := make([]string, 0, len(facetFindings))
	for facet := range facetFindings {
		facetKeys = append(facetKeys, facet)
	}
	sort.Strings(facetKeys)

	var summaries []evidence.ReviewFindingSummary
	for _, facet := range facetKeys {
		findings := facetFindings[facet]
		severityCounts := map[string]int{}
		for _, f := range findings {
			severityCounts[f.Severity.String()]++
		}
		sevStr := ""
		for sev, count := range severityCounts {
			if sevStr != "" {
				sevStr += ", "
			}
			sevStr += fmt.Sprintf("%d %s", count, sev)
		}
		if sevStr == "" {
			sevStr = "none"
		}

		summaries = append(summaries, evidence.ReviewFindingSummary{
			Facet:      facet,
			Count:      len(findings),
			Severities: sevStr,
		})
	}

	return summaries
}

func (s *EvidenceStage) readAcceptanceCriteria(path string) []evidence.AcceptanceCriterionSummary {
	data, err := os.ReadFile(path)
	if err != nil {
		return []evidence.AcceptanceCriterionSummary{
			{Criterion: "acceptance", Status: "Not evaluated", Rationale: "No acceptance.json found"},
		}
	}

	var result struct {
		Results []struct {
			Criterion string `json:"criterion"`
			Status    string `json:"status"`
			Rationale string `json:"rationale"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return []evidence.AcceptanceCriterionSummary{
			{Criterion: "acceptance", Status: "Not evaluated", Rationale: "Invalid acceptance.json"},
		}
	}

	var summaries []evidence.AcceptanceCriterionSummary
	for _, r := range result.Results {
		summaries = append(summaries, evidence.AcceptanceCriterionSummary{
			Criterion: r.Criterion,
			Status:    r.Status,
			Rationale: r.Rationale,
		})
	}

	return summaries
}
