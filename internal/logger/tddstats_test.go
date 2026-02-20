package logger

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadTDDPhaseRecords_FiltersByType(t *testing.T) {
	dir := t.TempDir()

	run1 := []any{
		IterationLog{
			Timestamp: time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC),
			Iteration: 1,
			BeadID:    "b-1",
			Model:     "sonnet",
			Success:   true,
		},
		TDDPhaseRecord{
			Type:        "tdd_phase",
			Timestamp:   time.Date(2026, 2, 19, 10, 1, 0, 0, time.UTC),
			BeadID:      "b-1",
			Phase:       "red",
			CycleNumber: 1,
			Model:       "haiku",
			Success:     false,
		},
		TDDSummaryRecord{
			Type:        "tdd_summary",
			Timestamp:   time.Date(2026, 2, 19, 10, 2, 0, 0, time.UTC),
			BeadID:      "b-1",
			TotalCycles: 1,
			TotalPhases: 3,
			Success:     true,
		},
	}
	writeMixedLogFile(t, dir, "20260219-100000", run1)

	run2 := []any{
		TDDPhaseRecord{
			Type:          "tdd_phase",
			Timestamp:     time.Date(2026, 2, 19, 11, 1, 0, 0, time.UTC),
			BeadID:        "b-2",
			Phase:         "green",
			CycleNumber:   1,
			Model:         "sonnet",
			Success:       true,
			Escalated:     true,
			EscalatedFrom: "haiku",
		},
		TDDPhaseRecord{
			Type:        "tdd_phase",
			Timestamp:   time.Date(2026, 2, 19, 11, 2, 0, 0, time.UTC),
			BeadID:      "b-2",
			Phase:       "refactor",
			CycleNumber: 1,
			Model:       "sonnet",
			Success:     true,
		},
	}
	writeMixedLogFile(t, dir, "20260219-110000", run2)

	records, err := ReadTDDPhaseRecords(dir)
	if err != nil {
		t.Fatalf("ReadTDDPhaseRecords failed: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("ReadTDDPhaseRecords length = %d, want 3", len(records))
	}
	if records[0].Type != "tdd_phase" || records[1].Type != "tdd_phase" || records[2].Type != "tdd_phase" {
		t.Fatalf("all returned records must be tdd_phase")
	}
}

func TestReadTDDSummaries_FiltersByType(t *testing.T) {
	dir := t.TempDir()

	records := []any{
		IterationLog{
			Timestamp: time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC),
			Iteration: 1,
			BeadID:    "b-1",
			Model:     "sonnet",
			Success:   true,
		},
		TDDPhaseRecord{
			Type:        "tdd_phase",
			Timestamp:   time.Date(2026, 2, 19, 12, 1, 0, 0, time.UTC),
			BeadID:      "b-1",
			Phase:       "red",
			CycleNumber: 1,
			Model:       "sonnet",
			Success:     false,
		},
		TDDSummaryRecord{
			Type:            "tdd_summary",
			Timestamp:       time.Date(2026, 2, 19, 12, 2, 0, 0, time.UTC),
			BeadID:          "b-1",
			TotalCycles:     2,
			TotalPhases:     6,
			Success:         true,
			TotalDurationMs: 1000,
		},
		TDDSummaryRecord{
			Type:            "tdd_summary",
			Timestamp:       time.Date(2026, 2, 19, 12, 3, 0, 0, time.UTC),
			BeadID:          "b-2",
			TotalCycles:     1,
			TotalPhases:     3,
			Success:         false,
			TotalDurationMs: 500,
		},
	}
	writeMixedLogFile(t, dir, "20260219-120000", records)

	got, err := ReadTDDSummaries(dir)
	if err != nil {
		t.Fatalf("ReadTDDSummaries failed: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("ReadTDDSummaries length = %d, want 2", len(got))
	}
	if got[0].Type != "tdd_summary" || got[1].Type != "tdd_summary" {
		t.Fatalf("all returned records must be tdd_summary")
	}
}

func TestAggregateTDDStats_ComputesExpectedMetrics(t *testing.T) {
	dir := t.TempDir()

	records := []any{
		IterationLog{
			Timestamp: time.Date(2026, 2, 19, 13, 0, 0, 0, time.UTC),
			Iteration: 1,
			BeadID:    "bead-a",
			Model:     "haiku",
			Success:   false,
			CostUSD:   0.60,
		},
		IterationLog{
			Timestamp: time.Date(2026, 2, 19, 13, 1, 0, 0, time.UTC),
			Iteration: 2,
			BeadID:    "bead-a",
			Model:     "sonnet",
			Success:   true,
			CostUSD:   0.40,
		},
		TDDPhaseRecord{
			Type:         "tdd_phase",
			Timestamp:    time.Date(2026, 2, 19, 13, 2, 0, 0, time.UTC),
			BeadID:       "bead-a",
			Phase:        "red",
			CycleNumber:  1,
			Model:        "haiku",
			InputTokens:  100,
			OutputTokens: 30,
			Success:      false,
		},
		TDDPhaseRecord{
			Type:          "tdd_phase",
			Timestamp:     time.Date(2026, 2, 19, 13, 3, 0, 0, time.UTC),
			BeadID:        "bead-a",
			Phase:         "green",
			CycleNumber:   1,
			Model:         "sonnet",
			InputTokens:   120,
			OutputTokens:  40,
			Success:       true,
			Escalated:     true,
			EscalatedFrom: "haiku",
		},
		TDDPhaseRecord{
			Type:         "tdd_phase",
			Timestamp:    time.Date(2026, 2, 19, 13, 4, 0, 0, time.UTC),
			BeadID:       "bead-a",
			Phase:        "refactor",
			CycleNumber:  1,
			Model:        "sonnet",
			InputTokens:  80,
			OutputTokens: 20,
			Success:      true,
		},
		TDDPhaseRecord{
			Type:         "tdd_phase",
			Timestamp:    time.Date(2026, 2, 19, 13, 5, 0, 0, time.UTC),
			BeadID:       "bead-a",
			Phase:        "red",
			CycleNumber:  2,
			Model:        "sonnet",
			InputTokens:  90,
			OutputTokens: 30,
			Success:      true,
		},
		TDDSummaryRecord{
			Type:            "tdd_summary",
			Timestamp:       time.Date(2026, 2, 19, 13, 6, 0, 0, time.UTC),
			BeadID:          "bead-a",
			TotalCycles:     2,
			TotalPhases:     4,
			Success:         true,
			TotalDurationMs: 1800,
		},
		IterationLog{
			Timestamp: time.Date(2026, 2, 19, 14, 0, 0, 0, time.UTC),
			Iteration: 1,
			BeadID:    "bead-b",
			Model:     "sonnet",
			Success:   true,
			CostUSD:   0.30,
		},
		TDDPhaseRecord{
			Type:         "tdd_phase",
			Timestamp:    time.Date(2026, 2, 19, 14, 1, 0, 0, time.UTC),
			BeadID:       "bead-b",
			Phase:        "red",
			CycleNumber:  1,
			Model:        "sonnet",
			InputTokens:  70,
			OutputTokens: 20,
			Success:      true,
		},
		TDDPhaseRecord{
			Type:         "tdd_phase",
			Timestamp:    time.Date(2026, 2, 19, 14, 2, 0, 0, time.UTC),
			BeadID:       "bead-b",
			Phase:        "green",
			CycleNumber:  1,
			Model:        "sonnet",
			InputTokens:  60,
			OutputTokens: 15,
			Success:      true,
		},
		TDDPhaseRecord{
			Type:         "tdd_phase",
			Timestamp:    time.Date(2026, 2, 19, 14, 3, 0, 0, time.UTC),
			BeadID:       "bead-b",
			Phase:        "refactor",
			CycleNumber:  1,
			Model:        "sonnet",
			InputTokens:  50,
			OutputTokens: 10,
			Success:      false,
		},
		TDDSummaryRecord{
			Type:            "tdd_summary",
			Timestamp:       time.Date(2026, 2, 19, 14, 4, 0, 0, time.UTC),
			BeadID:          "bead-b",
			TotalCycles:     1,
			TotalPhases:     3,
			Success:         false,
			TotalDurationMs: 900,
		},
	}
	writeMixedLogFile(t, dir, "20260219-130000", records)

	stats, err := AggregateTDDStats(dir)
	if err != nil {
		t.Fatalf("AggregateTDDStats failed: %v", err)
	}

	if stats.BeadRuns != 2 {
		t.Errorf("BeadRuns = %d, want 2", stats.BeadRuns)
	}
	if stats.TotalCycles != 3 {
		t.Errorf("TotalCycles = %d, want 3", stats.TotalCycles)
	}
	if stats.TotalPhases != 7 {
		t.Errorf("TotalPhases = %d, want 7", stats.TotalPhases)
	}
	assertFloatNearValue(t, stats.AvgCyclesPerBead, 1.5, "AvgCyclesPerBead")
	assertFloatNearValue(t, stats.AvgCostUSDPerCycle, 1.3/3.0, "AvgCostUSDPerCycle")
	assertFloatNearValue(t, stats.AvgInputTokensCycle, 190.0, "AvgInputTokensCycle")
	assertFloatNearValue(t, stats.AvgOutputTokensCycle, 55.0, "AvgOutputTokensCycle")
	assertFloatNearValue(t, stats.PhaseSuccessRates["red"], 2.0/3.0, "PhaseSuccessRates[red]")
	assertFloatNearValue(t, stats.PhaseSuccessRates["green"], 1.0, "PhaseSuccessRates[green]")
	assertFloatNearValue(t, stats.PhaseSuccessRates["refactor"], 0.5, "PhaseSuccessRates[refactor]")

	if stats.EscalationPatterns["haiku->sonnet"] != 1 {
		t.Errorf("EscalationPatterns[haiku->sonnet] = %d, want 1", stats.EscalationPatterns["haiku->sonnet"])
	}
}

func TestAggregateTDDStats_EmptyDir(t *testing.T) {
	stats, err := AggregateTDDStats(t.TempDir())
	if err != nil {
		t.Fatalf("AggregateTDDStats failed: %v", err)
	}

	if stats.BeadRuns != 0 || stats.TotalCycles != 0 || stats.TotalPhases != 0 {
		t.Fatalf("expected zero stats in empty dir, got %+v", stats)
	}
	if stats.PhaseSuccessRates == nil {
		t.Fatal("PhaseSuccessRates should be initialized")
	}
	if stats.EscalationPatterns == nil {
		t.Fatal("EscalationPatterns should be initialized")
	}
}

func writeMixedLogFile(t *testing.T, dir string, runID string, records []any) {
	t.Helper()

	filename := filepath.Join(dir, "run-"+runID+".jsonl")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("failed creating log file: %v", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			t.Fatalf("failed encoding log record: %v", err)
		}
	}
}

func assertFloatNearValue(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}
