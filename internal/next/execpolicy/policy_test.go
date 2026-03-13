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

func TestDefaultPolicy_HasFacetRetryCount(t *testing.T) {
	p := DefaultPolicy()
	if p.Review.FacetRetries != 2 {
		t.Errorf("default FacetRetries = %d, want 2", p.Review.FacetRetries)
	}
}

func TestDefaultPolicy_HasReviewConfig(t *testing.T) {
	p := DefaultPolicy()
	if p.Review.ReplanThreshold != "warning" {
		t.Errorf("default ReplanThreshold = %q, want %q", p.Review.ReplanThreshold, "warning")
	}
	if len(p.Review.Facets) != 2 {
		t.Errorf("default Facets count = %d, want 2", len(p.Review.Facets))
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
