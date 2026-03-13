package acceptor

import (
	"context"
	"encoding/json"
	"testing"
)

type mockAcceptAgent struct {
	results map[string]CriterionResult
}

func (m *mockAcceptAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	// Extract criterion from prompt (simplified: return based on first match)
	for criterion, result := range m.results {
		if containsSubstring(prompt, criterion) {
			return result, nil
		}
	}
	return CriterionResult{Status: StatusPass, Rationale: "default pass"}, nil
}

func TestEvaluator_AllPass(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"returns 200":    {Criterion: "returns 200", Status: StatusPass, Rationale: "test proves it"},
			"handles errors": {Criterion: "handles errors", Status: StatusPass, Rationale: "error tests exist"},
		},
	}

	eval := NewEvaluator(agent)
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"returns 200", "handles errors"},
		DiffSummary: "Added handler",
		TaskResults: "all passed",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.AllPass {
		t.Error("expected AllPass=true")
	}
	if result.HasFailOrUnclear {
		t.Error("expected HasFailOrUnclear=false")
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
}

func TestEvaluator_FailTriggersReplan(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"multi-currency": {Criterion: "multi-currency", Status: StatusFail, Rationale: "only USD implemented"},
		},
	}

	eval := NewEvaluator(agent)
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"multi-currency"},
		DiffSummary: "Added handler",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.AllPass {
		t.Error("expected AllPass=false")
	}
	if !result.HasFailOrUnclear {
		t.Error("expected HasFailOrUnclear=true")
	}
}

func TestEvaluator_UnclearTriggersReplan(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"audit log": {Criterion: "audit log", Status: StatusUnclear, Rationale: "no test verifies audit call"},
		},
	}

	eval := NewEvaluator(agent)
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"audit log"},
		DiffSummary: "Added handler",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.HasFailOrUnclear {
		t.Error("unclear should set HasFailOrUnclear=true")
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- CriterionResult JSON parsing boundary tests ---

func TestCriterionResult_UnmarshalPassFailUnclear(t *testing.T) {
	cases := []struct {
		status string
	}{
		{StatusPass},
		{StatusFail},
		{StatusUnclear},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			raw := `{"criterion":"c","status":"` + tc.status + `","rationale":"r","evidence_refs":["e1"]}`
			var cr CriterionResult
			if err := json.Unmarshal([]byte(raw), &cr); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if cr.Status != tc.status {
				t.Errorf("Status = %q, want %q", cr.Status, tc.status)
			}
		})
	}
}

func TestCriterionResult_UnmarshalInvalidStatus(t *testing.T) {
	// CriterionResult uses plain string for Status (no custom UnmarshalJSON),
	// so any string is accepted by json.Unmarshal. The invalid status is only
	// caught by business logic (e.g., in Evaluator). This test documents that
	// the JSON layer does NOT reject invalid statuses.
	raw := `{"criterion":"c","status":"invalid_status","rationale":"r","evidence_refs":[]}`
	var cr CriterionResult
	if err := json.Unmarshal([]byte(raw), &cr); err != nil {
		t.Fatalf("unmarshal should not fail for unknown status string: %v", err)
	}
	if cr.Status != "invalid_status" {
		t.Errorf("Status = %q, want %q", cr.Status, "invalid_status")
	}
}

func TestCriterionResult_UnmarshalMissingRationale(t *testing.T) {
	raw := `{"criterion":"c","status":"pass","evidence_refs":["e1"]}`
	var cr CriterionResult
	if err := json.Unmarshal([]byte(raw), &cr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cr.Rationale != "" {
		t.Errorf("expected empty Rationale, got %q", cr.Rationale)
	}
}

func TestCriterionResult_UnmarshalMissingEvidenceRefs_IsNil(t *testing.T) {
	// When evidence_refs is omitted from JSON, the field is nil (not []).
	// NormalizeNilFields must be called to get [].
	raw := `{"criterion":"c","status":"pass","rationale":"r"}`
	var cr CriterionResult
	if err := json.Unmarshal([]byte(raw), &cr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cr.EvidenceRefs != nil {
		t.Error("expected nil EvidenceRefs before normalization")
	}
	cr.NormalizeNilFields()
	if cr.EvidenceRefs == nil {
		t.Error("expected non-nil EvidenceRefs after NormalizeNilFields")
	}
	if len(cr.EvidenceRefs) != 0 {
		t.Errorf("expected 0 evidence refs, got %d", len(cr.EvidenceRefs))
	}
}

func TestCriterionResult_UnmarshalNullEvidenceRefs(t *testing.T) {
	// Explicit null in JSON should result in nil slice.
	raw := `{"criterion":"c","status":"fail","rationale":"r","evidence_refs":null}`
	var cr CriterionResult
	if err := json.Unmarshal([]byte(raw), &cr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cr.EvidenceRefs != nil {
		t.Error("expected nil EvidenceRefs for explicit null")
	}
	cr.NormalizeNilFields()
	if cr.EvidenceRefs == nil {
		t.Error("expected non-nil EvidenceRefs after NormalizeNilFields")
	}
}

func TestCriterionResult_UnmarshalMalformedJSON(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"truncated", `{"criterion":"c","status":"pa`},
		{"not_json", `not json at all`},
		{"empty", ``},
		{"just_brace", `{`},
		{"wrong_type_evidence", `{"criterion":"c","status":"pass","rationale":"r","evidence_refs":"not_array"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cr CriterionResult
			err := json.Unmarshal([]byte(tc.raw), &cr)
			if err == nil {
				t.Fatal("expected error for malformed JSON")
			}
		})
	}
}

func TestAcceptanceResult_UnmarshalEmptyResults_NotNull(t *testing.T) {
	raw := `{"results":[],"all_pass":true,"has_fail_or_unclear":false}`
	var ar AcceptanceResult
	if err := json.Unmarshal([]byte(raw), &ar); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ar.Results == nil {
		t.Error("empty results array should unmarshal as empty slice, not nil")
	}
	if len(ar.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(ar.Results))
	}
}

func TestAcceptanceResult_UnmarshalNullResults_NormalizeFixesIt(t *testing.T) {
	raw := `{"results":null,"all_pass":false,"has_fail_or_unclear":false}`
	var ar AcceptanceResult
	if err := json.Unmarshal([]byte(raw), &ar); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ar.Results != nil {
		t.Error("expected nil Results for explicit null before normalization")
	}
	ar.NormalizeNilFields()
	if ar.Results == nil {
		t.Error("expected non-nil Results after NormalizeNilFields")
	}
}
