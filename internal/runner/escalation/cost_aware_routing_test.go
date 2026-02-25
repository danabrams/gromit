package escalation

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

// TestEstimateBreadScopeAndCostReturnsZeroCostForNilBead verifies that
// EstimateBreadScopeAndCost handles nil bead gracefully and returns zero cost.
// Expected failure: function does not exist yet
func TestEstimateBreadScopeAndCostReturnsZeroCostForNilBead(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"codex": {
				ModelCosts: map[string]*config.ModelCost{
					"gpt-5.3-codex": {
						CostPer1kInput:  0.10,
						CostPer1kOutput: 0.20,
					},
				},
			},
		},
	}

	cost := EstimateBreadScopeAndCost(cfg, nil, "gpt-5.3-codex", "codex")
	if cost != 0 {
		t.Errorf("EstimateBreadScopeAndCost(nil bead) = %f, want 0", cost)
	}
}

// TestEstimateBreadScopeAndCostCalculatesTotalCost verifies that
// EstimateBreadScopeAndCost estimates the total cost based on bead file count and model pricing.
// Expected failure: function does not exist yet
func TestEstimateBreadScopeAndCostCalculatesTotalCost(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"codex": {
				ModelCosts: map[string]*config.ModelCost{
					"gpt-5.3-codex": {
						CostPer1kInput:  0.10,
						CostPer1kOutput: 0.20,
					},
				},
			},
		},
	}

	b := &bead.Bead{
		Title:           "test bead",
		ExpectedOutputs: []string{"file1.go", "file2.go", "file3.go", "file4.go", "file5.go"},
	}

	// With 5 files and default tokens-per-file estimate, cost should be > 0
	cost := EstimateBreadScopeAndCost(cfg, b, "gpt-5.3-codex", "codex")
	if cost <= 0 {
		t.Errorf("EstimateBreadScopeAndCost(5-file bead) = %f, want > 0", cost)
	}
}

// TestCheckAndLogCostCeilingWarnsWhenExceeded verifies that
// CheckAndLogCostCeiling emits a warning when estimated cost exceeds the ceiling.
// Expected failure: function does not exist yet
func TestCheckAndLogCostCeilingWarnsWhenExceeded(t *testing.T) {
	warningLogged := false
	logFn := func(format string, args ...interface{}) {
		warningLogged = true
	}

	exceeded := CheckAndLogCostCeiling(0.50, 1.00, logFn)
	if !exceeded {
		t.Error("CheckAndLogCostCeiling(0.50, 1.00) should return true (exceeded)")
	}
	if !warningLogged {
		t.Error("CheckAndLogCostCeiling should call logFn when cost exceeds ceiling")
	}
}

// TestCheckAndLogCostCeilingNoWarningWhenUnderCeiling verifies that
// CheckAndLogCostCeiling does not emit a warning when estimated cost is under the ceiling.
// Expected failure: function does not exist yet
func TestCheckAndLogCostCeilingNoWarningWhenUnderCeiling(t *testing.T) {
	warningLogged := false
	logFn := func(format string, args ...interface{}) {
		warningLogged = true
	}

	exceeded := CheckAndLogCostCeiling(0.50, 0.25, logFn)
	if exceeded {
		t.Error("CheckAndLogCostCeiling(0.50, 0.25) should return false (not exceeded)")
	}
	if warningLogged {
		t.Error("CheckAndLogCostCeiling should not call logFn when cost is under ceiling")
	}
}

// TestSelectCostAwareModelPrefersCheaperWhenBroadScope verifies that
// SelectCostAwareModel returns cheaper model when bead scope is large (> 5 files).
// Expected failure: function does not exist yet
func TestSelectCostAwareModelPrefersCheaperWhenBroadScope(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"codex": {
				ModelCosts: map[string]*config.ModelCost{
					"gpt-5.3-codex": {
						CostPer1kInput:  0.30, // Expensive
						CostPer1kOutput: 0.60,
					},
					"gpt-5.2-codex": {
						CostPer1kInput:  0.05, // Cheap
						CostPer1kOutput: 0.10,
					},
				},
			},
		},
	}

	b := &bead.Bead{
		Title: "large bead",
		// 6 files = broad scope (> 5)
		ExpectedOutputs: []string{"f1.go", "f2.go", "f3.go", "f4.go", "f5.go", "f6.go"},
	}

	selectedModel := SelectCostAwareModel(cfg, b, "gpt-5.3-codex", "codex")
	// Should select cheaper gpt-5.2-codex for broad scope
	if selectedModel != "gpt-5.2-codex" {
		t.Errorf("SelectCostAwareModel() = %q, want %q for broad scope", selectedModel, "gpt-5.2-codex")
	}
}

// TestSelectCostAwareModelKeepsExpensiveForSmallScope verifies that
// SelectCostAwareModel returns original model when bead scope is small (<= 5 files).
// Expected failure: function does not exist yet
func TestSelectCostAwareModelKeepsExpensiveForSmallScope(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"codex": {
				ModelCosts: map[string]*config.ModelCost{
					"gpt-5.3-codex": {
						CostPer1kInput:  0.30,
						CostPer1kOutput: 0.60,
					},
					"gpt-5.2-codex": {
						CostPer1kInput:  0.05,
						CostPer1kOutput: 0.10,
					},
				},
			},
		},
	}

	b := &bead.Bead{
		Title: "small bead",
		// 3 files = small scope (<= 5)
		ExpectedOutputs: []string{"f1.go", "f2.go", "f3.go"},
	}

	selectedModel := SelectCostAwareModel(cfg, b, "gpt-5.3-codex", "codex")
	// Should keep original model for small scope
	if selectedModel != "gpt-5.3-codex" {
		t.Errorf("SelectCostAwareModel() = %q, want %q for small scope", selectedModel, "gpt-5.3-codex")
	}
}
