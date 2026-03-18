package execpolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScenario_DefaultThresholds_WhenPolicyOmitsEscalation(t *testing.T) {
	// Seed: write an execution policy JSON with no "escalation" section
	tmp := t.TempDir()
	policyJSON := `{
		"always_run": [
			{"name": "unit-tests", "command": "go test ./...", "type": "test"}
		],
		"budgets": {
			"max_spec_cycles": 2,
			"max_task_retries": 1,
			"max_redecomposition_passes": 1,
			"max_task_duration_seconds": 120,
			"max_run_duration_seconds": 1800,
			"max_run_cost_usd": 25.0
		},
		"models": {
			"planner": "high",
			"executor": "medium",
			"evaluator": "high"
		},
		"review": {
			"facets": ["spec_alignment"],
			"tiers": {"spec_alignment": "high"},
			"replan_threshold": "warning"
		},
		"routing": {
			"preferences": {"plan": "any", "execute": "any"},
			"ratio": {"claude": 100}
		}
	}`
	path := filepath.Join(tmp, "execution-policy.json")
	if err := os.WriteFile(path, []byte(policyJSON), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	// Invoke: load the policy
	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}

	// Assert: escalation defaults are applied
	if p.Escalation.ErrorContextThreshold != 2 {
		t.Errorf("ErrorContextThreshold = %d, want 2 (default)", p.Escalation.ErrorContextThreshold)
	}
	if p.Escalation.ModelEscalationThreshold != 3 {
		t.Errorf("ModelEscalationThreshold = %d, want 3 (default)", p.Escalation.ModelEscalationThreshold)
	}

	// Assert: explicitly set fields were not overwritten by defaults
	if p.Budgets.MaxSpecCycles != 2 {
		t.Errorf("MaxSpecCycles = %d, want 2 (from JSON)", p.Budgets.MaxSpecCycles)
	}
}