package experiment

import (
	"encoding/json"
	"fmt"
)

// VariantReport represents the report for a single variant.
type VariantReport struct {
	VariantID    string
	SuccessRate  float64
	AvgCost      float64
	BanditWeight float64
}

// ExperimentReport represents the report for a single experiment.
type ExperimentReport struct {
	ExperimentID   string
	VariantReports []*VariantReport
}

// GenerateReport loads bandit state for each experiment and returns the report.
func GenerateReport(experiments []*Experiment, stateDir string) (*ExperimentReport, error) {
	return &ExperimentReport{}, nil
}

// FormatReport formats the experiment report as a human-readable string.
func (er *ExperimentReport) FormatReport() string {
	result := fmt.Sprintf("Experiment %s\n", er.ExperimentID)
	for _, vr := range er.VariantReports {
		result += fmt.Sprintf("  %s: success_rate=%.2f, avg_cost=%.4f, bandit_weight=%.4f\n",
			vr.VariantID, vr.SuccessRate, vr.AvgCost, vr.BanditWeight)
	}
	return result
}

// FormatReportJSON formats the experiment report as JSON.
func (er *ExperimentReport) FormatReportJSON() string {
	data, _ := json.MarshalIndent(er, "", "  ")
	return string(data)
}
