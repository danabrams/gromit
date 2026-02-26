package display

// RunStatus holds the data needed to render run information.
type RunStatus struct {
	Running                bool
	Iteration              int
	IterationTotal         int
	MaxIterations          int
	TimeBudgetMinutes      int
	BeadID                 string
	BeadTitle              string
	Model                  string
	ElapsedS               int
	AutonomyRate           float64
	FirstPassSuccessRate   float64
	MTTRProxyMs            int64
	EscalationRatesByClass map[string]float64
	RecurrenceCounters     map[string]int
}
