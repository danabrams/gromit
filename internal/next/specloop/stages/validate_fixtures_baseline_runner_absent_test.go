package stages

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestValidate_BaselineRunnerAbsent_FixtureSetup ensures the baseline-runner-absent
// fixture includes the project-level policy file that validation expects.
func TestValidate_BaselineRunnerAbsent_FixtureSetup(t *testing.T) {
	t.Parallel()

	fixtureDir := filepath.Join("testdata", "baseline-runner-absent")
	info, err := os.Stat(fixtureDir)
	if err != nil {
		t.Fatalf("stat fixture dir %q: %v", fixtureDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("fixture path %q is not a directory", fixtureDir)
	}

	keepFile := filepath.Join(fixtureDir, ".keep")
	if _, err := os.Stat(keepFile); err != nil {
		t.Fatalf("baseline fixture .keep missing: %v", err)
	}

	policyPath := filepath.Join(fixtureDir, "policy.json")
	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("read policy file %q: %v", policyPath, err)
	}

	var fixturePolicy struct {
		Budgets struct {
			MaxSpecCycles int `json:"max_spec_cycles"`
		} `json:"budgets"`
		AlwaysRun []struct {
			Name    string `json:"name"`
			Command string `json:"command"`
		} `json:"always_run"`
	}
	if err := json.Unmarshal(policyBytes, &fixturePolicy); err != nil {
		t.Fatalf("unmarshal policy %q: %v", policyPath, err)
	}

	if fixturePolicy.Budgets.MaxSpecCycles <= 0 {
		t.Fatalf("policy missing budgets.max_spec_cycles, got %d", fixturePolicy.Budgets.MaxSpecCycles)
	}
	if len(fixturePolicy.AlwaysRun) == 0 {
		t.Fatal("policy should declare at least one always_run check")
	}
	for _, check := range fixturePolicy.AlwaysRun {
		if check.Name == "" {
			t.Fatalf("always_run check missing name: %+v", check)
		}
		if check.Command == "" {
			t.Fatalf("always_run check %q missing command", check.Name)
		}
	}
}
