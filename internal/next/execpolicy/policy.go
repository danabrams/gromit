package execpolicy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Policy defines execution policy configuration: always-run checks, budgets,
// model tier config, and review settings.
type Policy struct {
	AlwaysRun []Check      `json:"always_run"`
	Budgets   Budgets      `json:"budgets"`
	Models    Models       `json:"models"`
	Review    ReviewConfig `json:"review"`
}

// ReviewConfig defines review stage configuration.
type ReviewConfig struct {
	Facets          []string          `json:"facets"`
	Tiers           map[string]string `json:"tiers"`
	ReplanThreshold string            `json:"replan_threshold"`
	FacetRetries    int               `json:"facet_retries"`
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
	Planner   string `json:"planner"`
	Executor  string `json:"executor"`
	Evaluator string `json:"evaluator"`
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
		Models: Models{Planner: "high", Executor: "medium", Evaluator: "high"},
		Review: ReviewConfig{
			Facets:          []string{"spec_alignment", "code_quality"},
			Tiers:           map[string]string{"spec_alignment": "high", "code_quality": "medium"},
			ReplanThreshold: "warning",
			FacetRetries:    2,
		},
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

// Validate checks that required policy fields have valid values.
// MaxTaskRetries and MaxRedecompositionPasses may be zero.
func (p *Policy) Validate() error {
	var errs []error
	if len(p.AlwaysRun) == 0 {
		errs = append(errs, fmt.Errorf("at least one always_run check is required"))
	}
	for i, c := range p.AlwaysRun {
		if c.Name == "" {
			errs = append(errs, fmt.Errorf("AlwaysRun[%d].Name must be non-empty", i))
		}
		if c.Command == "" {
			errs = append(errs, fmt.Errorf("AlwaysRun[%d].Command must be non-empty", i))
		}
	}
	if p.Budgets.MaxSpecCycles <= 0 {
		errs = append(errs, fmt.Errorf("MaxSpecCycles must be > 0, got %d", p.Budgets.MaxSpecCycles))
	}
	if p.Budgets.MaxTaskDurationSeconds <= 0 {
		errs = append(errs, fmt.Errorf("MaxTaskDurationSeconds must be > 0, got %d", p.Budgets.MaxTaskDurationSeconds))
	}
	if p.Budgets.MaxRunDurationSeconds <= 0 {
		errs = append(errs, fmt.Errorf("MaxRunDurationSeconds must be > 0, got %d", p.Budgets.MaxRunDurationSeconds))
	}
	if p.Budgets.MaxRunCostUSD <= 0 {
		errs = append(errs, fmt.Errorf("MaxRunCostUSD must be > 0, got %v", p.Budgets.MaxRunCostUSD))
	}
	if p.Models.Planner == "" {
		errs = append(errs, fmt.Errorf("Models.Planner must be non-empty"))
	}
	if p.Models.Executor == "" {
		errs = append(errs, fmt.Errorf("Models.Executor must be non-empty"))
	}
	if p.Models.Evaluator == "" {
		errs = append(errs, fmt.Errorf("Models.Evaluator must be non-empty"))
	}
	return errors.Join(errs...)
}

// NormalizeNilFields maps nil slices/maps to empty values.
// Exported since Policy is a cross-package type.
func (p *Policy) NormalizeNilFields() {
	if p.AlwaysRun == nil {
		p.AlwaysRun = []Check{}
	}
	if p.Review.Facets == nil {
		p.Review.Facets = []string{}
	}
	if p.Review.Tiers == nil {
		p.Review.Tiers = map[string]string{}
	}
}
