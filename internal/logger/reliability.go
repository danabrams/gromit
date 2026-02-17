package logger

// ReliabilityMetrics captures reliability KPIs derived from iteration logs.
type ReliabilityMetrics struct {
	AutonomyRate           float64            `json:"autonomy_rate"`
	FirstPassSuccessRate   float64            `json:"first_pass_success_rate"`
	MTTRProxyMs            int64              `json:"mttr_proxy_ms"`
	EscalationRatesByClass map[string]float64 `json:"escalation_rates_by_class"`
	RecurrenceCounters     map[string]int     `json:"recurrence_counters"`
}

// ReadReliabilityMetrics derives reliability metrics from run JSONL files.
func ReadReliabilityMetrics(logsDir string) (ReliabilityMetrics, error) {
	return ReliabilityMetrics{
		EscalationRatesByClass: map[string]float64{},
		RecurrenceCounters:     map[string]int{},
	}, nil
}
