package planner

import "testing"

func TestValidatePlan_RejectsEmptyTasks(t *testing.T) {
	p := Plan{Tasks: nil}
	if err := ValidatePlan(p); err == nil {
		t.Fatal("expected error for empty tasks")
	}
}

func TestValidatePlan_RejectsDuplicateIDs(t *testing.T) {
	p := Plan{Tasks: []TaskDef{
		{TaskID: "t-001", Objective: "a", ExpectedTouchedArea: []string{"x"}, ProofChecks: []string{"y"}},
		{TaskID: "t-001", Objective: "b", ExpectedTouchedArea: []string{"x"}, ProofChecks: []string{"y"}},
	}}
	if err := ValidatePlan(p); err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
}

func TestValidatePlan_RejectsMissingFields(t *testing.T) {
	p := Plan{Tasks: []TaskDef{{TaskID: "t-001"}}} // missing objective, area, checks
	if err := ValidatePlan(p); err == nil {
		t.Fatal("expected error for missing required fields")
	}
}

func TestValidatePlan_AcceptsValid(t *testing.T) {
	p := Plan{Tasks: []TaskDef{{
		TaskID: "t-001", Objective: "do thing",
		ExpectedTouchedArea: []string{"pkg/a"}, ProofChecks: []string{"go test ./pkg/a/"},
	}}}
	if err := ValidatePlan(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePlan_CrossCycleSequentialIDs(t *testing.T) {
	priorMaxID := "t-004"
	p := Plan{
		Cycle: 2, Kind: "fix",
		Tasks: []TaskDef{
			{TaskID: "t-005", Objective: "fix lint", ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"}},
			{TaskID: "t-006", Objective: "fix test", ExpectedTouchedArea: []string{"b/"}, ProofChecks: []string{"true"}},
		},
	}
	if err := ValidatePlanWithPrior(p, priorMaxID); err != nil {
		t.Fatalf("sequential IDs should be accepted: %v", err)
	}
}

func TestValidatePlan_KindOriginal_Accepted(t *testing.T) {
	p := Plan{Kind: "original", Tasks: []TaskDef{{
		TaskID: "t-001", Objective: "do thing",
		ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"},
	}}}
	if err := ValidatePlan(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePlan_KindFix_Accepted(t *testing.T) {
	p := Plan{Kind: "fix", Tasks: []TaskDef{{
		TaskID: "t-001", Objective: "do thing",
		ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"},
	}}}
	if err := ValidatePlan(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePlan_KindEmpty_Accepted(t *testing.T) {
	p := Plan{Kind: "", Tasks: []TaskDef{{
		TaskID: "t-001", Objective: "do thing",
		ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"},
	}}}
	if err := ValidatePlan(p); err != nil {
		t.Fatalf("unexpected error for empty kind: %v", err)
	}
}

func TestValidatePlan_KindInvalid_Rejected(t *testing.T) {
	p := Plan{Kind: "banana", Tasks: []TaskDef{{
		TaskID: "t-001", Objective: "do thing",
		ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"},
	}}}
	if err := ValidatePlan(p); err == nil {
		t.Fatal("expected error for invalid kind \"banana\"")
	}
}

func TestValidatePlan_CrossCycleNonSequentialIDs_Rejected(t *testing.T) {
	priorMaxID := "t-004"
	p := Plan{
		Cycle: 2, Kind: "fix",
		Tasks: []TaskDef{
			{TaskID: "t-001", Objective: "fix lint", ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"}},
		},
	}
	if err := ValidatePlanWithPrior(p, priorMaxID); err == nil {
		t.Fatal("reusing prior cycle IDs should be rejected")
	}
}
