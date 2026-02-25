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
