package visionmetrics

// MetricRate captures both raw counts and the derived floating-point rate for a KPI.
type MetricRate struct {
	Numerator   int     `json:"numerator"`
	Denominator int     `json:"denominator"`
	Rate        float64 `json:"rate"`
}

// Rollup aggregates the key vision KPIs over a set of validated cycle records.
type Rollup struct {
	HumanTacticalInterventionRate  MetricRate `json:"human_tactical_intervention_rate"`
	HumanDebuggingInterventionRate MetricRate `json:"human_debugging_intervention_rate"`
	FirstIntegrationPassRate       MetricRate `json:"first_integration_pass_rate"`
	EscapedRegressionRate          MetricRate `json:"escaped_regression_rate"`
	AcceptedWithoutReworkRate      MetricRate `json:"accepted_without_rework_rate"`
}

// ComputeRollup calculates KPI rates from the provided records, validating and filtering them internally.
func ComputeRollup(records []Record) Rollup {
	// Partition records into valid and invalid
	var valid []Record
	for _, rec := range records {
		if len(Validate(rec)) == 0 {
			valid = append(valid, rec)
		}
	}

var (
	tactical  int
	debugging int
	escaped   int
	accepted  int
	resolvedEscapedDenom int
	carveOuts int
)

	for _, rec := range valid {
		if rec.HumanTacticalIntervention == Yes {
			tactical++
		}
		if rec.HumanDebuggingIntervention == Yes {
			debugging++
		}
		if rec.EscapedRegressionWithin7D == Yes {
			escaped++
		}
		if rec.EscapedRegressionWithin7D != EscapedRegressionPending {
			resolvedEscapedDenom++
		}
		if rec.ReviewOutcome.IsAccepted() {
			accepted++
		}
		if rec.ReviewOutcome.IsCarveOut() {
			carveOuts++
		}
	}

	firstPass := accepted
	total := len(valid)
	acceptedDenom := total - carveOuts
	if acceptedDenom < 0 {
		acceptedDenom = 0
	}

	return Rollup{
		HumanTacticalInterventionRate:  newMetricRate(tactical, total),
		HumanDebuggingInterventionRate: newMetricRate(debugging, total),
		FirstIntegrationPassRate:       newMetricRate(firstPass, total),
	EscapedRegressionRate:          newMetricRate(escaped, resolvedEscapedDenom),
		AcceptedWithoutReworkRate:      newMetricRate(accepted, acceptedDenom),
	}
}

func newMetricRate(numerator, denominator int) MetricRate {
	rate := 0.0
	if denominator > 0 {
		rate = float64(numerator) / float64(denominator)
	}
	return MetricRate{Numerator: numerator, Denominator: denominator, Rate: rate}
}
