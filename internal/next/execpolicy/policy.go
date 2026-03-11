package execpolicy

// Policy defines execution policy configuration: always-run checks, budgets,
// and model tier config.
type Policy struct {
	AlwaysRun []Check `json:"always_run"`
	Budgets   Budgets `json:"budgets"`
	Models    Models  `json:"models"`
}

// Check is a validation check that always runs after task execution.
type Check struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Type    string `json:"type"` // "test" or "lint"
}

// Budgets defines resource limits for the execution loop.
type Budgets struct {
	MaxSpecCycles            int     `json:"max_spec_cycles"`
	MaxTaskRetries           int     `json:"max_task_retries"`
	MaxRedecompositionPasses int     `json:"max_redecomposition_passes"`
	MaxTaskDurationSeconds   int     `json:"max_task_duration_seconds"`
	MaxRunDurationSeconds    int     `json:"max_run_duration_seconds"`
	MaxRunCostUSD            float64 `json:"max_run_cost_usd"`
}

// Models defines which model tier to use for each role.
type Models struct {
	Planner  string `json:"planner"`
	Executor string `json:"executor"`
}

// DefaultPolicy returns a Policy with sensible production defaults.
func DefaultPolicy() Policy {
	return Policy{
		AlwaysRun: []Check{
			{Name: "unit-tests", Command: "go test ./...", Type: "test"},
			{Name: "format", Command: "gofmt -l .", Type: "lint"},
			{Name: "vet", Command: "go vet ./...", Type: "lint"},
		},
		Budgets: Budgets{
			MaxSpecCycles:            3,
			MaxTaskRetries:           1,
			MaxRedecompositionPasses: 1,
			MaxTaskDurationSeconds:   300,
			MaxRunDurationSeconds:    3600,
			MaxRunCostUSD:            50.0,
		},
		Models: Models{Planner: "high", Executor: "medium"},
	}
}

// NormalizeNilFields maps nil slices/maps to empty values.
// Exported since Policy is a cross-package type.
func (p *Policy) NormalizeNilFields() {
	if p.AlwaysRun == nil {
		p.AlwaysRun = []Check{}
	}
}
