package logger

import (
	"testing"
	"time"
)

func TestIterationEfficiency_Struct(t *testing.T) {
	// Verify struct can be instantiated and fields are accessible
	ie := IterationEfficiency{
		BeadID:            "test-123",
		Model:             "opus",
		Duration:          30 * time.Second,
		CostUSD:           0.42,
		InputTokens:       10000,
		OutputTokens:      2000,
		ContextWindowUsed: 0.85,
		ExceededThreshold: true,
	}

	if ie.BeadID != "test-123" {
		t.Errorf("BeadID = %s, want test-123", ie.BeadID)
	}
	if ie.Model != "opus" {
		t.Errorf("Model = %s, want opus", ie.Model)
	}
	if ie.Duration != 30*time.Second {
		t.Errorf("Duration = %v, want 30s", ie.Duration)
	}
	if ie.CostUSD != 0.42 {
		t.Errorf("CostUSD = %f, want 0.42", ie.CostUSD)
	}
	if ie.InputTokens != 10000 {
		t.Errorf("InputTokens = %d, want 10000", ie.InputTokens)
	}
	if ie.OutputTokens != 2000 {
		t.Errorf("OutputTokens = %d, want 2000", ie.OutputTokens)
	}
	if ie.ContextWindowUsed != 0.85 {
		t.Errorf("ContextWindowUsed = %f, want 0.85", ie.ContextWindowUsed)
	}
	if !ie.ExceededThreshold {
		t.Error("ExceededThreshold = false, want true")
	}
}

func TestModelEfficiency_Struct(t *testing.T) {
	// Verify struct can be instantiated and fields are accessible
	me := ModelEfficiency{
		Model:             "sonnet",
		IterationCount:    5,
		AvgCostUSD:        0.25,
		AvgDuration:       20 * time.Second,
		AvgInputTokens:    8000,
		AvgOutputTokens:   1500,
		TotalCostUSD:      1.25,
		TotalDuration:     100 * time.Second,
		TotalInputTokens:  40000,
		TotalOutputTokens: 7500,
	}

	if me.Model != "sonnet" {
		t.Errorf("Model = %s, want sonnet", me.Model)
	}
	if me.IterationCount != 5 {
		t.Errorf("IterationCount = %d, want 5", me.IterationCount)
	}
	if me.AvgCostUSD != 0.25 {
		t.Errorf("AvgCostUSD = %f, want 0.25", me.AvgCostUSD)
	}
	if me.AvgDuration != 20*time.Second {
		t.Errorf("AvgDuration = %v, want 20s", me.AvgDuration)
	}
	if me.AvgInputTokens != 8000 {
		t.Errorf("AvgInputTokens = %f, want 8000", me.AvgInputTokens)
	}
	if me.AvgOutputTokens != 1500 {
		t.Errorf("AvgOutputTokens = %f, want 1500", me.AvgOutputTokens)
	}
	if me.TotalCostUSD != 1.25 {
		t.Errorf("TotalCostUSD = %f, want 1.25", me.TotalCostUSD)
	}
	if me.TotalDuration != 100*time.Second {
		t.Errorf("TotalDuration = %v, want 100s", me.TotalDuration)
	}
	if me.TotalInputTokens != 40000 {
		t.Errorf("TotalInputTokens = %d, want 40000", me.TotalInputTokens)
	}
	if me.TotalOutputTokens != 7500 {
		t.Errorf("TotalOutputTokens = %d, want 7500", me.TotalOutputTokens)
	}
}

func TestEfficiencyReport_Struct(t *testing.T) {
	// Verify struct can be instantiated and fields are accessible
	report := EfficiencyReport{
		CurrentIterations: []IterationEfficiency{
			{BeadID: "bead-1", Model: "opus"},
			{BeadID: "bead-2", Model: "sonnet"},
		},
		CurrentModels: map[string]ModelEfficiency{
			"opus":   {Model: "opus", IterationCount: 1},
			"sonnet": {Model: "sonnet", IterationCount: 1},
		},
		HistoricalModels: map[string]ModelEfficiency{
			"opus":   {Model: "opus", IterationCount: 10},
			"sonnet": {Model: "sonnet", IterationCount: 20},
		},
		CurrentAvgCostPerBead:        0.42,
		CurrentAvgDurationPerBead:    30 * time.Second,
		HistoricalAvgCostPerBead:     0.31,
		HistoricalAvgDurationPerBead: 25 * time.Second,
		CostDelta:                    0.11,
		DurationDelta:                5 * time.Second,
		HighContextIterations: []IterationEfficiency{
			{BeadID: "bead-1", ExceededThreshold: true},
		},
	}

	if len(report.CurrentIterations) != 2 {
		t.Errorf("len(CurrentIterations) = %d, want 2", len(report.CurrentIterations))
	}
	if len(report.CurrentModels) != 2 {
		t.Errorf("len(CurrentModels) = %d, want 2", len(report.CurrentModels))
	}
	if len(report.HistoricalModels) != 2 {
		t.Errorf("len(HistoricalModels) = %d, want 2", len(report.HistoricalModels))
	}
	if report.CurrentAvgCostPerBead != 0.42 {
		t.Errorf("CurrentAvgCostPerBead = %f, want 0.42", report.CurrentAvgCostPerBead)
	}
	if report.CurrentAvgDurationPerBead != 30*time.Second {
		t.Errorf("CurrentAvgDurationPerBead = %v, want 30s", report.CurrentAvgDurationPerBead)
	}
	if report.HistoricalAvgCostPerBead != 0.31 {
		t.Errorf("HistoricalAvgCostPerBead = %f, want 0.31", report.HistoricalAvgCostPerBead)
	}
	if report.HistoricalAvgDurationPerBead != 25*time.Second {
		t.Errorf("HistoricalAvgDurationPerBead = %v, want 25s", report.HistoricalAvgDurationPerBead)
	}
	if report.CostDelta != 0.11 {
		t.Errorf("CostDelta = %f, want 0.11", report.CostDelta)
	}
	if report.DurationDelta != 5*time.Second {
		t.Errorf("DurationDelta = %v, want 5s", report.DurationDelta)
	}
	if len(report.HighContextIterations) != 1 {
		t.Errorf("len(HighContextIterations) = %d, want 1", len(report.HighContextIterations))
	}
}

func TestEfficiencyReport_ZeroValues(t *testing.T) {
	// Verify zero-value initialization works correctly
	var report EfficiencyReport

	if report.CurrentIterations != nil {
		t.Error("CurrentIterations should be nil when zero-initialized")
	}
	if report.CurrentModels != nil {
		t.Error("CurrentModels should be nil when zero-initialized")
	}
	if report.HistoricalModels != nil {
		t.Error("HistoricalModels should be nil when zero-initialized")
	}
	if report.CurrentAvgCostPerBead != 0 {
		t.Errorf("CurrentAvgCostPerBead = %f, want 0", report.CurrentAvgCostPerBead)
	}
	if report.CostDelta != 0 {
		t.Errorf("CostDelta = %f, want 0", report.CostDelta)
	}
}
