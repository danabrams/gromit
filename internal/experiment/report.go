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
	if len(experiments) == 0 {
		return &ExperimentReport{}, nil
	}

	exp := experiments[0]
	state, err := LoadState(stateDir, exp.ID)
	if err != nil {
		return nil, err
	}

	report := &ExperimentReport{
		ExperimentID: exp.ID,
	}

	// Build variant reports from bandit state
	for _, arm := range state.Arms {
		total := arm.Successes + arm.Failures
		successRate := 0.0
		if total > 0 {
			successRate = float64(arm.Successes) / float64(total)
		}

		banditWeight := computeBanditWeight(state, arm.ID)

		vr := &VariantReport{
			VariantID:    arm.ID,
			SuccessRate:  successRate,
			AvgCost:      0.0,
			BanditWeight: banditWeight,
		}
		report.VariantReports = append(report.VariantReports, vr)
	}

	return report, nil
}

// computeBanditWeight computes the probability that an arm is the best using Monte Carlo sampling.
func computeBanditWeight(bs *BanditState, targetArmID string) float64 {
	if len(bs.Arms) == 0 {
		return 0.0
	}

	const numDraws = 10000

	// Count how many times the target arm is the best across draws
	winCount := 0

	for draw := 0; draw < numDraws; draw++ {
		maxValue := -1.0
		bestArmID := ""

		for _, arm := range bs.Arms {
			// Sample from Beta(successes+1, failures+1)
			x := sampleGamma(float64(arm.Successes + 1))
			y := sampleGamma(float64(arm.Failures + 1))
			sample := x / (x + y)

			if sample > maxValue {
				maxValue = sample
				bestArmID = arm.ID
			}
		}

		if bestArmID == targetArmID {
			winCount++
		}
	}

	return float64(winCount) / numDraws
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
