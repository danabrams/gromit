package experiment

import (
	"testing"
	"time"
)

func TestGenerateReportReturnsReport(t *testing.T) {
	// Verify that GenerateReport returns an ExperimentReport
	exp := &Experiment{
		ID:    "exp-1",
		Phase: "build",
		Control: &Variant{
			ID: "control",
		},
		Variants: []*Variant{
			{ID: "variant-1"},
		},
		Created: time.Now(),
	}

	report, err := GenerateReport([]*Experiment{exp}, t.TempDir())
	if err != nil {
		t.Fatalf("GenerateReport error: %v", err)
	}

	if report == nil {
		t.Fatalf("expected non-nil report")
	}
}

func TestVariantReportHasSuccessRate(t *testing.T) {
	// Verify that VariantReport has a success rate field
	vr := &VariantReport{
		VariantID:   "variant-1",
		SuccessRate: 0.85,
	}

	if vr.VariantID != "variant-1" {
		t.Fatalf("expected variant ID variant-1, got %q", vr.VariantID)
	}
	if vr.SuccessRate != 0.85 {
		t.Fatalf("expected success rate 0.85, got %f", vr.SuccessRate)
	}
}

func TestVariantReportHasAvgCost(t *testing.T) {
	// Verify that VariantReport has an avg cost field
	vr := &VariantReport{
		VariantID: "variant-1",
		AvgCost:   0.125,
	}

	if vr.AvgCost != 0.125 {
		t.Fatalf("expected avg cost 0.125, got %f", vr.AvgCost)
	}
}

func TestVariantReportHasBanditWeight(t *testing.T) {
	// Verify that VariantReport has a BanditWeight field
	vr := &VariantReport{
		VariantID:    "variant-1",
		BanditWeight: 0.75,
	}

	if vr.BanditWeight != 0.75 {
		t.Fatalf("expected bandit weight 0.75, got %f", vr.BanditWeight)
	}
}

func TestExperimentReportHasVariantReports(t *testing.T) {
	// Verify that ExperimentReport has variant reports
	er := &ExperimentReport{
		ExperimentID: "exp-1",
		VariantReports: []*VariantReport{
			{VariantID: "control"},
			{VariantID: "variant-1"},
		},
	}

	if er.ExperimentID != "exp-1" {
		t.Fatalf("expected experiment ID exp-1, got %q", er.ExperimentID)
	}
	if len(er.VariantReports) != 2 {
		t.Fatalf("expected 2 variant reports, got %d", len(er.VariantReports))
	}
}

func TestFormatReportReturnsString(t *testing.T) {
	// Verify that FormatReport returns a formatted string
	er := &ExperimentReport{
		ExperimentID: "exp-1",
		VariantReports: []*VariantReport{
			{VariantID: "control", SuccessRate: 0.8},
		},
	}

	formatted := er.FormatReport()
	if formatted == "" {
		t.Fatalf("expected non-empty formatted report")
	}
	if formatted == "ExperimentReport{}" {
		t.Fatalf("expected formatted report with content, got %q", formatted)
	}
}

func TestFormatReportJSONReturnsJSON(t *testing.T) {
	// Verify that FormatReportJSON returns valid JSON
	er := &ExperimentReport{
		ExperimentID: "exp-1",
		VariantReports: []*VariantReport{
			{VariantID: "control", SuccessRate: 0.8},
		},
	}

	json := er.FormatReportJSON()
	if json == "" {
		t.Fatalf("expected non-empty JSON report")
	}
	if !contains(json, "exp-1") {
		t.Fatalf("expected JSON to contain experiment ID, got %s", json)
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGenerateReportLoadsStateAndBuildsReport(t *testing.T) {
	// Verify that GenerateReport loads bandit state and builds variant reports
	stateDir := t.TempDir()

	// Create a bandit state with some data
	state := &BanditState{
		Arms: []ArmState{
			{ID: "control", Successes: 10, Failures: 2},
			{ID: "variant-1", Successes: 15, Failures: 1},
		},
	}

	// Save the state
	if err := SaveState(stateDir, "exp-1", state); err != nil {
		t.Fatalf("SaveState error: %v", err)
	}

	// Create experiment
	exp := &Experiment{
		ID:    "exp-1",
		Phase: "build",
		Control: &Variant{
			ID: "control",
		},
		Variants: []*Variant{
			{ID: "variant-1"},
		},
		Created: time.Now(),
	}

	// Generate report
	report, err := GenerateReport([]*Experiment{exp}, stateDir)
	if err != nil {
		t.Fatalf("GenerateReport error: %v", err)
	}

	if report == nil {
		t.Fatalf("expected non-nil report")
	}

	if report.ExperimentID != "exp-1" {
		t.Fatalf("expected experiment ID exp-1, got %q", report.ExperimentID)
	}

	if len(report.VariantReports) == 0 {
		t.Fatalf("expected variant reports, got none")
	}
}
