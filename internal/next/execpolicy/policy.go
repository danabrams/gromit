package execpolicy

import (
	"encoding/json"
	"fmt"
	"os"
)

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

// LoadPolicy reads a JSON policy file. If the file does not exist, it returns
// DefaultPolicy. Partial JSON is unmarshalled on top of defaults.
func LoadPolicy(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPolicy(), nil
		}
		return Policy{}, fmt.Errorf("read policy: %w", err)
	}
	p := DefaultPolicy() // start from defaults
	if err := json.Unmarshal(data, &p); err != nil {
		return Policy{}, fmt.Errorf("parse policy: %w", err)
	}
	// NOTE: Do NOT add zero-value fallback lines here. The unmarshal-into-defaults
	// approach already handles partial configs correctly. Explicit zero-value
	// checks would make it impossible to intentionally set a field to 0.
	return p, nil
}

// NormalizeNilFields maps nil slices/maps to empty values.
// Exported since Policy is a cross-package type.
func (p *Policy) NormalizeNilFields() {
	if p.AlwaysRun == nil {
		p.AlwaysRun = []Check{}
	}
}
