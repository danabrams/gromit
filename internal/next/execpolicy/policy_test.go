package execpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPolicy_HasExpectedBudgets(t *testing.T) {
	p := DefaultPolicy()
	if p.Budgets.MaxSpecCycles != 3 {
		t.Fatalf("want MaxSpecCycles=3, got %d", p.Budgets.MaxSpecCycles)
	}
	if p.Budgets.MaxTaskRetries != 1 {
		t.Fatalf("want MaxTaskRetries=1, got %d", p.Budgets.MaxTaskRetries)
	}
	if p.Budgets.MaxRunDurationSeconds != 3600 {
		t.Fatalf("want MaxRunDurationSeconds=3600, got %d", p.Budgets.MaxRunDurationSeconds)
	}
	if p.Models.Planner != "high" {
		t.Fatalf("want Planner=high, got %s", p.Models.Planner)
	}
}

func TestDefaultPolicy_AlwaysRunChecksNonEmpty(t *testing.T) {
	p := DefaultPolicy()
	if len(p.AlwaysRun) == 0 {
		t.Fatal("default policy must include at least one always-run check")
	}
}

func TestLoadPolicy_FromJSON(t *testing.T) {
	dir := t.TempDir()
	data := `{"budgets":{"max_spec_cycles":5},"models":{"planner":"xhigh","executor":"high"},"always_run":[{"name":"vet","command":"go vet ./...","type":"lint"}]}`
	os.WriteFile(filepath.Join(dir, "execution-policy.json"), []byte(data), 0644)

	p, err := LoadPolicy(filepath.Join(dir, "execution-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Budgets.MaxSpecCycles != 5 {
		t.Fatalf("want 5, got %d", p.Budgets.MaxSpecCycles)
	}
	if p.Models.Planner != "xhigh" {
		t.Fatalf("want xhigh, got %s", p.Models.Planner)
	}
}

func TestLoadPolicy_FileNotFound_ReturnsDefault(t *testing.T) {
	p, err := LoadPolicy("/nonexistent/policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if p.Budgets.MaxSpecCycles != 3 {
		t.Fatal("expected default when file missing")
	}
}

func TestValidate_RejectsZeroRequiredBudgets(t *testing.T) {
	p := Policy{} // all zeroes
	err := p.Validate()
	if err == nil {
		t.Fatal("expected validation error for zero budgets")
	}
}

func TestValidate_AcceptsDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if err := p.Validate(); err != nil {
		t.Fatalf("default policy should be valid: %v", err)
	}
}

func TestValidate_AcceptsZeroTaskRetriesAndRedecomposition(t *testing.T) {
	p := DefaultPolicy()
	p.Budgets.MaxTaskRetries = 0
	p.Budgets.MaxRedecompositionPasses = 0
	if err := p.Validate(); err != nil {
		t.Fatalf("MaxTaskRetries=0 and MaxRedecompositionPasses=0 should be valid: %v", err)
	}
}

func TestValidate_RejectsEmptyAlwaysRun(t *testing.T) {
	p := DefaultPolicy()
	p.AlwaysRun = []Check{}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty AlwaysRun")
	}
	if !strings.Contains(err.Error(), "at least one always_run check is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidate_RejectsZeroMaxTaskDurationSeconds(t *testing.T) {
	p := DefaultPolicy()
	p.Budgets.MaxTaskDurationSeconds = 0
	err := p.Validate()
	if err == nil {
		t.Fatal("expected validation error for zero MaxTaskDurationSeconds")
	}
	if !strings.Contains(err.Error(), "MaxTaskDurationSeconds must be > 0") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidate_RejectsNegativeMaxTaskDurationSeconds(t *testing.T) {
	p := DefaultPolicy()
	p.Budgets.MaxTaskDurationSeconds = -10
	err := p.Validate()
	if err == nil {
		t.Fatal("expected validation error for negative MaxTaskDurationSeconds")
	}
}

func TestValidate_RejectsEmptyCheckName(t *testing.T) {
	p := DefaultPolicy()
	p.AlwaysRun = []Check{{Name: "", Command: "go test ./...", Type: "test"}}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty Check Name")
	}
	if !strings.Contains(err.Error(), "AlwaysRun[0].Name must be non-empty") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidate_RejectsEmptyCheckCommand(t *testing.T) {
	p := DefaultPolicy()
	p.AlwaysRun = []Check{{Name: "vet", Command: "", Type: "lint"}}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty Check Command")
	}
	if !strings.Contains(err.Error(), "AlwaysRun[0].Command must be non-empty") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLoadPolicy_ReviewConfigFromJSON(t *testing.T) {
	tmpDir := t.TempDir()
	policyJSON := `{
		"review": {
			"facets": ["spec_alignment", "code_quality", "logic_gaps"],
			"tiers": {
				"spec_alignment": "high",
				"code_quality": "medium",
				"logic_gaps": "high"
			},
			"replan_threshold": "error"
		}
	}`
	path := filepath.Join(tmpDir, "policy.json")
	if err := os.WriteFile(path, []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if len(p.Review.Facets) != 3 {
		t.Errorf("expected 3 facets, got %d", len(p.Review.Facets))
	}
	if p.Review.ReplanThreshold != "error" {
		t.Errorf("threshold = %q, want error", p.Review.ReplanThreshold)
	}
	if p.Review.Tiers["logic_gaps"] != "high" {
		t.Errorf("logic_gaps tier = %q, want high", p.Review.Tiers["logic_gaps"])
	}
}

func TestLoadPolicy_ReviewDefaultsPreservedOnPartialJSON(t *testing.T) {
	tmpDir := t.TempDir()
	policyJSON := `{
		"review": {
			"replan_threshold": "warning"
		}
	}`
	path := filepath.Join(tmpDir, "policy.json")
	if err := os.WriteFile(path, []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.Review.ReplanThreshold != "warning" {
		t.Errorf("threshold should be overridden to warning, got %q", p.Review.ReplanThreshold)
	}
	if len(p.Review.Facets) != 2 {
		t.Errorf("facets should be default (2), got %d", len(p.Review.Facets))
	}
}

func TestDefaultPolicy_HasFacetRetryCount(t *testing.T) {
	p := DefaultPolicy()
	if p.Review.FacetRetries != 2 {
		t.Errorf("default FacetRetries = %d, want 2", p.Review.FacetRetries)
	}
}

func TestDefaultPolicy_HasReviewConfig(t *testing.T) {
	p := DefaultPolicy()
	if len(p.Review.Facets) != 2 {
		t.Fatalf("default review should have 2 facets, got %d", len(p.Review.Facets))
	}
	if p.Review.Facets[0] != "spec_alignment" {
		t.Errorf("first facet = %q, want spec_alignment", p.Review.Facets[0])
	}
	if p.Review.Facets[1] != "code_quality" {
		t.Errorf("second facet = %q, want code_quality", p.Review.Facets[1])
	}
	if p.Review.ReplanThreshold != "warning" {
		t.Errorf("ReplanThreshold = %q, want warning", p.Review.ReplanThreshold)
	}
}

func TestDefaultPolicy_ReviewTiers(t *testing.T) {
	p := DefaultPolicy()
	if p.Review.Tiers["spec_alignment"] != "high" {
		t.Errorf("spec_alignment tier = %q, want high", p.Review.Tiers["spec_alignment"])
	}
	if p.Review.Tiers["code_quality"] != "medium" {
		t.Errorf("code_quality tier = %q, want medium", p.Review.Tiers["code_quality"])
	}
}

func TestDefaultPolicy_HasEvaluatorTier(t *testing.T) {
	p := DefaultPolicy()
	if p.Models.Evaluator != "high" {
		t.Errorf("Evaluator tier = %q, want high", p.Models.Evaluator)
	}
}

func TestPolicy_Validate_EvaluatorRequired(t *testing.T) {
	p := DefaultPolicy()
	p.Models.Evaluator = ""
	err := p.Validate()
	if err == nil {
		t.Error("Validate should fail when Evaluator is empty")
	}
}

func TestPolicy_ValidateReviewFacets_ValidFacets(t *testing.T) {
	p := DefaultPolicy()
	err := p.ValidateReviewFacets([]string{"spec_alignment", "code_quality", "logic_gaps", "test_coverage", "architecture_drift"})
	if err != nil {
		t.Errorf("valid facets should not error: %v", err)
	}
}

func TestPolicy_ValidateReviewFacets_UnknownFacet(t *testing.T) {
	p := DefaultPolicy()
	p.Review.Facets = []string{"spec_alignment", "nonexistent_facet"}
	err := p.ValidateReviewFacets([]string{"spec_alignment", "code_quality"})
	if err == nil {
		t.Error("unknown facet should produce validation error")
	}
}

func TestPolicy_ValidateReviewThreshold_Valid(t *testing.T) {
	p := DefaultPolicy()
	for _, threshold := range []string{"error", "warning", "suggestion"} {
		p.Review.ReplanThreshold = threshold
		if err := p.ValidateReviewConfig(); err != nil {
			t.Errorf("threshold %q should be valid: %v", threshold, err)
		}
	}
}

func TestPolicy_ValidateReviewThreshold_Invalid(t *testing.T) {
	p := DefaultPolicy()
	p.Review.ReplanThreshold = "critical"
	if err := p.ValidateReviewConfig(); err == nil {
		t.Error("invalid threshold should produce error")
	}
}

func TestPolicy_Validate_CatchesInvalidThreshold(t *testing.T) {
	p := DefaultPolicy()
	p.Review.ReplanThreshold = "critical"
	err := p.Validate()
	if err == nil {
		t.Error("Validate should catch invalid ReplanThreshold")
	}
}

func TestPolicy_Validate_CatchesEmptyThreshold(t *testing.T) {
	p := DefaultPolicy()
	p.Review.ReplanThreshold = ""
	err := p.Validate()
	if err == nil {
		t.Error("Validate should catch empty ReplanThreshold")
	}
}

func TestValidate_RejectsEmptyCheckNameAndCommand(t *testing.T) {
	p := DefaultPolicy()
	p.AlwaysRun = []Check{
		{Name: "valid", Command: "go test ./...", Type: "test"},
		{Name: "", Command: "", Type: "lint"},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty Name and Command")
	}
	if !strings.Contains(err.Error(), "AlwaysRun[1].Name must be non-empty") {
		t.Fatalf("expected Name error for index 1: %v", err)
	}
	if !strings.Contains(err.Error(), "AlwaysRun[1].Command must be non-empty") {
		t.Fatalf("expected Command error for index 1: %v", err)
	}
}
