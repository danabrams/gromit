package analytics

import "time"

// ConvergenceSummary captures recent TDD instability state.
type ConvergenceSummary struct {
	LatestInstability string    `json:"latest_instability,omitempty"`
	LatestIteration   int       `json:"latest_iteration,omitempty"`
	LatestTimestamp   time.Time `json:"latest_timestamp,omitempty"`
	DeadlockCount     int       `json:"deadlock_count"`
	OscillationCount  int       `json:"oscillation_count"`
}

// PackageMaintenanceCost records validation-time UCL streaks per package.
type PackageMaintenanceCost struct {
	Package             string    `json:"package"`
	Metric              string    `json:"metric"`
	Severity            string    `json:"severity"`
	ConsecutiveBreaches int       `json:"consecutive_breaches"`
	LatestValue         float64   `json:"latest_value"`
	UCL                 float64   `json:"ucl"`
	DetectedAt          time.Time `json:"detected_at,omitempty"`
}

func applyPackageMaintenanceCosts(trend *ProcessTrend, metrics []IterationMetric, limit *TrendControlLimit) {
	if trend == nil || limit == nil {
		return
	}
	trend.PackageMaintenanceCosts = detectPackageMaintenanceCosts(metrics, *limit)
	for _, cost := range trend.PackageMaintenanceCosts {
		trend.FlaggedPackages = append(trend.FlaggedPackages, FlaggedPackage{
			Package:            cost.Package,
			Metric:             cost.Metric,
			Severity:           cost.Severity,
			PersistenceWindows: cost.ConsecutiveBreaches,
			DetectedAt:         cost.DetectedAt,
		})
	}
}
