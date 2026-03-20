package execpolicy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// EscalationConfig defines escalation thresholds for error handling and model selection.
type EscalationConfig struct {
	ErrorContextThreshold    int `json:"error_context_threshold"`
	ModelEscalationThreshold int `json:"model_escalation_threshold"`
}

// Policy defines execution policy configuration: always-run checks, budgets,
// model tier config, and review settings.
type Policy struct {
	AlwaysRun  []Check          `json:"always_run"`
	Budgets    Budgets          `json:"budgets"`
	Models     Models           `json:"models"`
	Review     ReviewConfig     `json:"review"`
	Routing    RoutingConfig    `json:"routing"`
	Escalation EscalationConfig `json:"escalation"`
}

// RoutingConfig defines multi-provider routing preferences.
type RoutingConfig struct {
	Preferences     map[string]string `json:"preferences"`      // phase -> provider name or "any"
	Ratio           map[string]int    `json:"ratio"`            // provider name -> percentage (must sum to 100)
	CooldownSeconds int               `json:"cooldown_seconds"` // seconds to mark provider unavailable after usage-limit
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices/maps to empty values.
func (rc *RoutingConfig) NormalizeNilFields() {
	if rc.Preferences == nil {
		rc.Preferences = map[string]string{}
	}
	if rc.Ratio == nil {
		rc.Ratio = map[string]int{}
	}
}

// ReviewConfig defines review stage configuration.
type ReviewConfig struct {
	Facets           []string          `json:"facets"`
	Tiers            map[string]string `json:"tiers"`
	ReplanThreshold  string            `json:"replan_threshold"`
	FacetMaxAttempts int               `json:"facet_max_attempts"`
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

// Models defines which model tier to use for each role, plus the mapping
// from tier names to concrete model names and optional reasoning effort.
type Models struct {
	Planner         string            `json:"planner"`
	Executor        string            `json:"executor"`
	Evaluator       string            `json:"evaluator"`
	TierModels      map[string]string `json:"tier_models,omitempty"`      // tier name -> model name (e.g., "high": "opus")
	ReasoningEffort map[string]string `json:"reasoning_effort,omitempty"` // tier name -> effort level (e.g., "high": "high", "low": "low")
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
		Models: Models{
			Planner:   "high",
			Executor:  "medium",
			Evaluator: "high",
			TierModels: map[string]string{
				"low":    "haiku",
				"medium": "sonnet",
				"high":   "opus",
			},
		},
		Review: ReviewConfig{
			Facets:           []string{"spec_alignment", "code_quality", "logic_gaps"},
			Tiers:            map[string]string{"spec_alignment": "high", "code_quality": "medium", "logic_gaps": "high"},
			ReplanThreshold:  "warning",
			FacetMaxAttempts: 2,
		},
		Routing: RoutingConfig{
			// "validate" is intentionally absent: the validate stage uses ShellValidator (no LLM provider).
			Preferences:     map[string]string{"plan": "any", "execute": "any", "review": "any", "accept": "any"},
			Ratio:           map[string]int{"claude": 100},
			CooldownSeconds: 300,
		},
		Escalation: EscalationConfig{
			ErrorContextThreshold:    2,
			ModelEscalationThreshold: 3,
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
	if len(p.Review.Facets) == 0 {
		errs = append(errs, fmt.Errorf("at least one review facet is required"))
	}
	// Validate review config (threshold enum). Facet validation requires a
	// known-facets list from the registry, so it must be called separately
	// via ValidateReviewFacets().
	if err := p.ValidateReviewConfig(); err != nil {
		errs = append(errs, err)
	}
	if len(p.Routing.Ratio) > 0 {
		sum := 0
		for name, v := range p.Routing.Ratio {
			if v < 0 {
				errs = append(errs, fmt.Errorf("routing.ratio[%q] must be non-negative, got %d", name, v))
			}
			sum += v
		}
		if sum != 100 {
			errs = append(errs, fmt.Errorf("routing.ratio values must sum to 100, got %d", sum))
		}
	}
	return errors.Join(errs...)
}

// ValidateReviewFacets checks that each facet in p.Review.Facets is in the
// known facets list.
func (p *Policy) ValidateReviewFacets(knownFacets []string) error {
	known := make(map[string]bool, len(knownFacets))
	for _, f := range knownFacets {
		known[f] = true
	}
	var errs []error
	for _, f := range p.Review.Facets {
		if !known[f] {
			errs = append(errs, fmt.Errorf("unknown review facet %q", f))
		}
	}
	return errors.Join(errs...)
}

// ValidateReviewConfig checks that ReplanThreshold is a valid severity level.
func (p *Policy) ValidateReviewConfig() error {
	switch p.Review.ReplanThreshold {
	case "error", "critical", "warning", "suggestion": // "critical" is the spec-defined alias for "error"
		return nil
	default:
		return fmt.Errorf("invalid ReplanThreshold %q, must be one of: error, critical, warning, suggestion", p.Review.ReplanThreshold)
	}
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices/maps to empty values.
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
	if p.Models.TierModels == nil {
		p.Models.TierModels = map[string]string{}
	}
	if p.Models.ReasoningEffort == nil {
		p.Models.ReasoningEffort = map[string]string{}
	}
	p.Routing.NormalizeNilFields()
}
