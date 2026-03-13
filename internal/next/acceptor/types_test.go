package acceptor

import (
	"encoding/json"
	"testing"
)

func TestCriterionResult_JSONRoundTrip(t *testing.T) {
	cr := CriterionResult{
		Criterion:    "Zero repo pollution",
		Status:       StatusPass,
		Rationale:    "No gromit files found in target repo.",
		EvidenceRefs: []string{"evidence/diff-summary.md", "evidence/worktree-info.json"},
	}

	data, err := json.Marshal(cr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got CriterionResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Status != StatusPass {
		t.Errorf("Status = %q, want %q", got.Status, StatusPass)
	}
	if len(got.EvidenceRefs) != 2 {
		t.Errorf("EvidenceRefs len = %d, want 2", len(got.EvidenceRefs))
	}
}

func TestCriterionResult_NormalizeNilFields(t *testing.T) {
	cr := CriterionResult{}
	cr.NormalizeNilFields()
	if cr.EvidenceRefs == nil {
		t.Error("NormalizeNilFields should set nil EvidenceRefs to empty slice")
	}
}

func TestAcceptanceResult_NormalizeNilFields(t *testing.T) {
	ar := AcceptanceResult{}
	ar.NormalizeNilFields()
	if ar.Results == nil {
		t.Error("NormalizeNilFields should set nil Results to empty slice")
	}
}

func TestStatus_Constants(t *testing.T) {
	if StatusPass != "pass" {
		t.Errorf("StatusPass = %q, want %q", StatusPass, "pass")
	}
	if StatusFail != "fail" {
		t.Errorf("StatusFail = %q, want %q", StatusFail, "fail")
	}
	if StatusUnclear != "unclear" {
		t.Errorf("StatusUnclear = %q, want %q", StatusUnclear, "unclear")
	}
}
