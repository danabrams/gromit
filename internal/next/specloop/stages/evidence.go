package stages

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/next/evidence"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// EvidenceStageConfig configures the EvidenceStage.
type EvidenceStageConfig struct {
	ValidationResult validator.FinalResult
	DiffSummary      string
	StartTime        time.Time
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

	if err := bundler.WriteValidation(s.cfg.ValidationResult); err != nil {
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

	review := evidence.ReviewInput{
		TerminalState:     rs.Status,
		WhatChanged:       s.cfg.DiffSummary,
		CycleHistory:      []evidence.CycleRecord{{Cycle: rs.Cycle, TaskCount: len(rs.Tasks), PassCount: passCount}},
		ValidationResults: fmt.Sprintf("pass=%v", s.cfg.ValidationResult.Pass),
		KnownRisks:        []string{},
		RecommendedAction: "review",
	}
	if err := bundler.WriteReview(review); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write review: %w", err)
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}
