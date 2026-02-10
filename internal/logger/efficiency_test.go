package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func TestReadEfficiencyReport_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	report, err := ReadEfficiencyReport(dir, "20260207-120000")
	if err != nil {
		t.Fatalf("ReadEfficiencyReport failed: %v", err)
	}

	if len(report.CurrentIterations) != 0 {
		t.Errorf("Expected 0 current iterations, got %d", len(report.CurrentIterations))
	}
	if len(report.CurrentModels) != 0 {
		t.Errorf("Expected 0 current models, got %d", len(report.CurrentModels))
	}
	if len(report.HistoricalModels) != 0 {
		t.Errorf("Expected 0 historical models, got %d", len(report.HistoricalModels))
	}
}

func TestReadEfficiencyReport_SingleRun(t *testing.T) {
	dir := t.TempDir()
	runID := "20260207-120000"

	// Create a log file with test data
	logs := []IterationLog{
		{
			BeadID:       "bead-1",
			Model:        "opus",
			DurationMs:   30000,
			CostUSD:      0.50,
			InputTokens:  10000,
			OutputTokens: 2000,
		},
		{
			BeadID:       "bead-2",
			Model:        "sonnet",
			DurationMs:   20000,
			CostUSD:      0.25,
			InputTokens:  8000,
			OutputTokens: 1500,
		},
	}

	writeTestLogFile(t, dir, runID, logs)

	report, err := ReadEfficiencyReport(dir, runID)
	if err != nil {
		t.Fatalf("ReadEfficiencyReport failed: %v", err)
	}

	// Verify current run data
	if len(report.CurrentIterations) != 2 {
		t.Errorf("Expected 2 current iterations, got %d", len(report.CurrentIterations))
	}
	if len(report.CurrentModels) != 2 {
		t.Errorf("Expected 2 current models, got %d", len(report.CurrentModels))
	}

	// Check opus stats
	opus := report.CurrentModels["opus"]
	if opus.IterationCount != 1 {
		t.Errorf("opus IterationCount = %d, want 1", opus.IterationCount)
	}
	if opus.TotalCostUSD != 0.50 {
		t.Errorf("opus TotalCostUSD = %f, want 0.50", opus.TotalCostUSD)
	}
	if opus.TotalInputTokens != 10000 {
		t.Errorf("opus TotalInputTokens = %d, want 10000", opus.TotalInputTokens)
	}

	// Check sonnet stats
	sonnet := report.CurrentModels["sonnet"]
	if sonnet.IterationCount != 1 {
		t.Errorf("sonnet IterationCount = %d, want 1", sonnet.IterationCount)
	}
	if sonnet.TotalCostUSD != 0.25 {
		t.Errorf("sonnet TotalCostUSD = %f, want 0.25", sonnet.TotalCostUSD)
	}

	// Overall averages
	expectedAvgCost := (0.50 + 0.25) / 2
	if report.CurrentAvgCostPerBead != expectedAvgCost {
		t.Errorf("CurrentAvgCostPerBead = %f, want %f", report.CurrentAvgCostPerBead, expectedAvgCost)
	}

	// No historical data
	if len(report.HistoricalModels) != 0 {
		t.Errorf("Expected 0 historical models, got %d", len(report.HistoricalModels))
	}
}

func TestReadEfficiencyReport_CurrentAndHistorical(t *testing.T) {
	dir := t.TempDir()
	currentRunID := "20260207-130000"
	historicalRunID := "20260207-120000"

	// Historical run
	historicalLogs := []IterationLog{
		{
			BeadID:       "bead-1",
			Model:        "opus",
			DurationMs:   25000,
			CostUSD:      0.40,
			InputTokens:  9000,
			OutputTokens: 1800,
		},
		{
			BeadID:       "bead-2",
			Model:        "sonnet",
			DurationMs:   15000,
			CostUSD:      0.20,
			InputTokens:  7000,
			OutputTokens: 1200,
		},
	}
	writeTestLogFile(t, dir, historicalRunID, historicalLogs)

	// Current run
	currentLogs := []IterationLog{
		{
			BeadID:       "bead-3",
			Model:        "opus",
			DurationMs:   35000,
			CostUSD:      0.60,
			InputTokens:  12000,
			OutputTokens: 2500,
		},
	}
	writeTestLogFile(t, dir, currentRunID, currentLogs)

	report, err := ReadEfficiencyReport(dir, currentRunID)
	if err != nil {
		t.Fatalf("ReadEfficiencyReport failed: %v", err)
	}

	// Verify current run
	if len(report.CurrentIterations) != 1 {
		t.Errorf("Expected 1 current iteration, got %d", len(report.CurrentIterations))
	}
	if len(report.CurrentModels) != 1 {
		t.Errorf("Expected 1 current model, got %d", len(report.CurrentModels))
	}

	// Verify historical data
	if len(report.HistoricalModels) != 2 {
		t.Errorf("Expected 2 historical models, got %d", len(report.HistoricalModels))
	}

	// Check historical opus
	histOpus := report.HistoricalModels["opus"]
	if histOpus.IterationCount != 1 {
		t.Errorf("historical opus IterationCount = %d, want 1", histOpus.IterationCount)
	}
	if histOpus.TotalCostUSD != 0.40 {
		t.Errorf("historical opus TotalCostUSD = %f, want 0.40", histOpus.TotalCostUSD)
	}

	// Check deltas
	historicalAvgCost := (0.40 + 0.20) / 2
	currentAvgCost := 0.60
	expectedDelta := currentAvgCost - historicalAvgCost
	// Use tolerance for floating point comparison
	tolerance := 0.0001
	if diff := report.CostDelta - expectedDelta; diff < -tolerance || diff > tolerance {
		t.Errorf("CostDelta = %f, want %f (diff=%f)", report.CostDelta, expectedDelta, diff)
	}

	if report.CostDelta <= 0 {
		t.Error("Expected positive CostDelta (current more expensive)")
	}
}

func TestReadEfficiencyReport_ContextWindowThreshold(t *testing.T) {
	dir := t.TempDir()
	runID := "20260207-120000"

	// Opus context window is 200000 tokens
	// 80% threshold = 160000 tokens
	logs := []IterationLog{
		{
			BeadID:       "bead-below",
			Model:        "opus",
			DurationMs:   30000,
			CostUSD:      0.50,
			InputTokens:  150000, // 75% - below threshold
			OutputTokens: 2000,
		},
		{
			BeadID:       "bead-at",
			Model:        "opus",
			DurationMs:   30000,
			CostUSD:      0.50,
			InputTokens:  160000, // exactly 80% - at threshold
			OutputTokens: 2000,
		},
		{
			BeadID:       "bead-above",
			Model:        "opus",
			DurationMs:   30000,
			CostUSD:      0.50,
			InputTokens:  170000, // 85% - above threshold
			OutputTokens: 2000,
		},
	}

	writeTestLogFile(t, dir, runID, logs)

	report, err := ReadEfficiencyReport(dir, runID)
	if err != nil {
		t.Fatalf("ReadEfficiencyReport failed: %v", err)
	}

	// Verify context window flags
	if len(report.HighContextIterations) != 2 {
		t.Errorf("Expected 2 high context iterations, got %d", len(report.HighContextIterations))
	}

	// Check that the right beads are flagged
	flagged := make(map[string]bool)
	for _, ie := range report.HighContextIterations {
		flagged[ie.BeadID] = true
		if !ie.ExceededThreshold {
			t.Errorf("Iteration %s should have ExceededThreshold=true", ie.BeadID)
		}
	}

	if !flagged["bead-at"] {
		t.Error("bead-at (80%) should be flagged")
	}
	if !flagged["bead-above"] {
		t.Error("bead-above (85%) should be flagged")
	}
	if flagged["bead-below"] {
		t.Error("bead-below (75%) should not be flagged")
	}

	// Verify context window percentages
	for _, ie := range report.CurrentIterations {
		switch ie.BeadID {
		case "bead-below":
			expected := 150000.0 / 200000.0
			if ie.ContextWindowUsed != expected {
				t.Errorf("bead-below ContextWindowUsed = %f, want %f", ie.ContextWindowUsed, expected)
			}
		case "bead-at":
			expected := 160000.0 / 200000.0
			if ie.ContextWindowUsed != expected {
				t.Errorf("bead-at ContextWindowUsed = %f, want %f", ie.ContextWindowUsed, expected)
			}
		case "bead-above":
			expected := 170000.0 / 200000.0
			if ie.ContextWindowUsed != expected {
				t.Errorf("bead-above ContextWindowUsed = %f, want %f", ie.ContextWindowUsed, expected)
			}
		}
	}
}

func TestReadEfficiencyReport_MultipleHistoricalRuns(t *testing.T) {
	dir := t.TempDir()
	currentRunID := "20260207-150000"

	// Create multiple historical runs
	for i, runID := range []string{"20260207-120000", "20260207-130000", "20260207-140000"} {
		logs := []IterationLog{
			{
				BeadID:       "bead-" + runID,
				Model:        "sonnet",
				DurationMs:   20000,
				CostUSD:      0.20 + float64(i)*0.05,
				InputTokens:  8000,
				OutputTokens: 1500,
			},
		}
		writeTestLogFile(t, dir, runID, logs)
	}

	// Current run
	currentLogs := []IterationLog{
		{
			BeadID:       "bead-current",
			Model:        "sonnet",
			DurationMs:   25000,
			CostUSD:      0.50,
			InputTokens:  9000,
			OutputTokens: 1800,
		},
	}
	writeTestLogFile(t, dir, currentRunID, currentLogs)

	report, err := ReadEfficiencyReport(dir, currentRunID)
	if err != nil {
		t.Fatalf("ReadEfficiencyReport failed: %v", err)
	}

	// Verify historical aggregation
	histSonnet := report.HistoricalModels["sonnet"]
	if histSonnet.IterationCount != 3 {
		t.Errorf("historical sonnet IterationCount = %d, want 3", histSonnet.IterationCount)
	}

	expectedHistCost := (0.20 + 0.25 + 0.30) / 3
	if histSonnet.AvgCostUSD != expectedHistCost {
		t.Errorf("historical sonnet AvgCostUSD = %f, want %f", histSonnet.AvgCostUSD, expectedHistCost)
	}

	// Verify current run
	currSonnet := report.CurrentModels["sonnet"]
	if currSonnet.IterationCount != 1 {
		t.Errorf("current sonnet IterationCount = %d, want 1", currSonnet.IterationCount)
	}
	if currSonnet.AvgCostUSD != 0.50 {
		t.Errorf("current sonnet AvgCostUSD = %f, want 0.50", currSonnet.AvgCostUSD)
	}
}

func TestExtractRunID(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "standard format",
			path:     "/path/to/run-20260207-120000.jsonl",
			expected: "20260207-120000",
		},
		{
			name:     "relative path",
			path:     "run-20260207-130000.jsonl",
			expected: "20260207-130000",
		},
		{
			name:     "short path",
			path:     "run-.jsonl",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRunID(tt.path)
			if result != tt.expected {
				t.Errorf("extractRunID(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

// Helper function to write test log files
func writeTestLogFile(t *testing.T, dir string, runID string, logs []IterationLog) {
	t.Helper()

	filename := filepath.Join(dir, "run-"+runID+".jsonl")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			t.Fatalf("Failed to write test log entry: %v", err)
		}
	}
}

func TestReadEfficiencyReportFiltered_NilFilterIncludesAll(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-07T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000,"cost_usd":0.1,"input_tokens":1000,"output_tokens":200}
{"timestamp":"2026-02-07T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"haiku","success":true,"validated":true,"escalated":false,"duration_ms":500,"cost_usd":0.05,"input_tokens":500,"output_tokens":100}
`
	runID := "20260207-120000"
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("run-%s.jsonl", runID)), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := ReadEfficiencyReportFiltered(dir, runID, nil)
	if err != nil {
		t.Fatalf("ReadEfficiencyReportFiltered failed: %v", err)
	}

	if len(report.CurrentIterations) != 2 {
		t.Errorf("expected 2 current iterations with nil filter, got %d", len(report.CurrentIterations))
	}
}

func TestReadEfficiencyReportFiltered_FilterIncludesOnlyMatchingBeads(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-07T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000,"cost_usd":0.1,"input_tokens":1000,"output_tokens":200}
{"timestamp":"2026-02-07T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"haiku","success":true,"validated":true,"escalated":false,"duration_ms":500,"cost_usd":0.05,"input_tokens":500,"output_tokens":100}
{"timestamp":"2026-02-07T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"opus","success":true,"validated":true,"escalated":false,"duration_ms":2000,"cost_usd":0.5,"input_tokens":2000,"output_tokens":400}
`
	runID := "20260207-120000"
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("run-%s.jsonl", runID)), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Filter to only include b1 and b3
	filter := map[string]bool{
		"b1": true,
		"b3": true,
	}

	report, err := ReadEfficiencyReportFiltered(dir, runID, filter)
	if err != nil {
		t.Fatalf("ReadEfficiencyReportFiltered failed: %v", err)
	}

	if len(report.CurrentIterations) != 2 {
		t.Errorf("expected 2 current iterations (b1 and b3), got %d", len(report.CurrentIterations))
	}

	// Verify b2 is excluded
	for _, iter := range report.CurrentIterations {
		if iter.BeadID == "b2" {
			t.Error("b2 should be excluded from filtered report")
		}
	}

	// Verify b1 and b3 are included
	foundB1 := false
	foundB3 := false
	for _, iter := range report.CurrentIterations {
		if iter.BeadID == "b1" {
			foundB1 = true
		}
		if iter.BeadID == "b3" {
			foundB3 = true
		}
	}

	if !foundB1 {
		t.Error("b1 should be included in filtered report")
	}
	if !foundB3 {
		t.Error("b3 should be included in filtered report")
	}
}
