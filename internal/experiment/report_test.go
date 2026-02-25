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
