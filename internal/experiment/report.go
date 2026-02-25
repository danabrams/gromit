package experiment

// VariantReport represents the report for a single variant.
type VariantReport struct {
	VariantID    string
	SuccessRate  float64
	AvgCost      float64
	BanditWeight float64
}

// ExperimentReport represents the report for a single experiment.
type ExperimentReport struct {
	ExperimentID string
}

// GenerateReport loads bandit state for each experiment and returns the report.
func GenerateReport(experiments []*Experiment, stateDir string) (*ExperimentReport, error) {
	return &ExperimentReport{}, nil
}
