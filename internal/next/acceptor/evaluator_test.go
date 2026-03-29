package acceptor

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"testing"
	"time"
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

type deadlineRecordingAgent struct {
	mu       sync.Mutex
	deadline time.Time
}

func (d *deadlineRecordingAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	if dl, ok := ctx.Deadline(); ok {
		d.mu.Lock()
		if d.deadline.IsZero() {
			d.deadline = dl
		}
		d.mu.Unlock()
	}
	return CriterionResult{Status: StatusPass, Rationale: "deadline recorded"}, nil
}

func (d *deadlineRecordingAgent) RecordedDeadline() (time.Time, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.deadline.IsZero() {
		return time.Time{}, false
	}
	return d.deadline, true
}

type blockingAcceptAgent struct {
	delay time.Duration
}

func (b *blockingAcceptAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	select {
	case <-ctx.Done():
		return CriterionResult{}, ctx.Err()
	case <-time.After(b.delay):
		return CriterionResult{Status: StatusPass, Rationale: "slow but done"}, nil
	}
}

func TestEvaluator_AllPass(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"returns 200":    {Criterion: "returns 200", Status: StatusPass, Rationale: "test proves it"},
			"handles errors": {Criterion: "handles errors", Status: StatusPass, Rationale: "error tests exist"},
		},
	}

	eval := NewEvaluator(agent, DefaultTimeoutConfig())
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

	eval := NewEvaluator(agent, DefaultTimeoutConfig())
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

	eval := NewEvaluator(agent, DefaultTimeoutConfig())
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

func TestEvaluator_SetsPerCriterionDeadline(t *testing.T) {
	cfg := TimeoutConfig{
		BaseSeconds:         1,
		RateConstant:        1_000_000,
		ComplexityBonusSecs: 0,
		HardMaximumSecs:     60,
	}
	input := EvaluateInput{
		Criteria:    []string{"deadline check"},
		DiffSummary: "d",
	}

	agent := &deadlineRecordingAgent{}
	eval := NewEvaluator(agent, cfg)
	start := time.Now()
	if _, err := eval.Evaluate(context.Background(), input); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	deadline, ok := agent.RecordedDeadline()
	if !ok {
		t.Fatal("expected recorded deadline from agent context")
	}

	expected := ComputeCriterionTimeout(cfg, len(input.DiffSummary), input.Criteria[0])
	elapsed := deadline.Sub(start)
	tolerance := 200 * time.Millisecond
	if elapsed < expected-tolerance || elapsed > expected+tolerance {
		t.Fatalf("expected deadline roughly %v after start, got %v", expected, elapsed)
	}
}

func TestEvaluator_ReturnsTimeoutErrorOnDeadlineExceeded(t *testing.T) {
	cfg := TimeoutConfig{
		BaseSeconds:         1,
		RateConstant:        1_000_000,
		ComplexityBonusSecs: 0,
		HardMaximumSecs:     1,
	}
	agent := &blockingAcceptAgent{delay: 2 * time.Second}
	eval := NewEvaluator(agent, cfg)
	_, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"timeout-heavy"},
		DiffSummary: "",
	})
	if err == nil {
		t.Fatal("expected error when evaluation exceeded deadline")
	}
	if !containsSubstring(err.Error(), "deadline exceeded") && !containsSubstring(err.Error(), "timeout") {
		t.Fatalf("error should mention deadline/timeout: %v", err)
	}
	if !containsSubstring(err.Error(), "timeout-heavy") {
		t.Fatalf("error should mention the criterion: %v", err)
	}
}

func TestEvaluator_LogsCriterionTimeoutComputed(t *testing.T) {
	cfg := TimeoutConfig{
		BaseSeconds:         5,
		RateConstant:        1_000_000,
		ComplexityBonusSecs: 0,
		HardMaximumSecs:     60,
	}
	criterion := "logs the computed timeout"
	diffSummary := "small diff"
	input := EvaluateInput{
		Criteria:    []string{criterion},
		DiffSummary: diffSummary,
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	agent := &deadlineRecordingAgent{}
	eval := NewEvaluator(agent, cfg)
	if _, err := eval.Evaluate(context.Background(), input); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	logOutput := buf.String()
	if !containsSubstring(logOutput, "criterion_timeout_computed") {
		t.Errorf("log output should contain criterion_timeout_computed; got: %s", logOutput)
	}
	if !containsSubstring(logOutput, criterion) {
		t.Errorf("log output should contain criterion text %q; got: %s", criterion, logOutput)
	}
	expectedTimeout := ComputeCriterionTimeout(cfg, len(diffSummary), criterion)
	if !containsSubstring(logOutput, expectedTimeout.String()) {
		t.Errorf("log output should contain timeout value %s; got: %s", expectedTimeout, logOutput)
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

func TestEvaluator_InvalidStatus_ReturnsError(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"some criterion": {Status: "bogus", Rationale: "has rationale"},
		},
	}

	eval := NewEvaluator(agent, DefaultTimeoutConfig())
	_, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"some criterion"},
		DiffSummary: "diff",
	})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	if !containsSubstring(err.Error(), "invalid status") {
		t.Errorf("error should mention invalid status: %v", err)
	}
}

func TestEvaluator_MissingRationaleOnFail_ReturnsError(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"needs rationale": {Status: StatusFail, Rationale: ""},
		},
	}

	eval := NewEvaluator(agent, DefaultTimeoutConfig())
	_, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"needs rationale"},
		DiffSummary: "diff",
	})
	if err == nil {
		t.Fatal("expected error for missing rationale on fail")
	}
	if !containsSubstring(err.Error(), "missing rationale") {
		t.Errorf("error should mention missing rationale: %v", err)
	}
}

func TestEvaluator_MissingRationaleOnUnclear_ReturnsError(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"unclear thing": {Status: StatusUnclear, Rationale: ""},
		},
	}

	eval := NewEvaluator(agent, DefaultTimeoutConfig())
	_, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"unclear thing"},
		DiffSummary: "diff",
	})
	if err == nil {
		t.Fatal("expected error for missing rationale on unclear")
	}
	if !containsSubstring(err.Error(), "missing rationale") {
		t.Errorf("error should mention missing rationale: %v", err)
	}
}

func TestEvaluator_EmptyRationaleOnPass_OK(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"passing": {Status: StatusPass, Rationale: ""},
		},
	}

	eval := NewEvaluator(agent, DefaultTimeoutConfig())
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"passing"},
		DiffSummary: "diff",
	})
	if err != nil {
		t.Fatalf("pass with empty rationale should not error: %v", err)
	}
	if !result.AllPass {
		t.Error("expected AllPass=true")
	}
}

// sequencingAcceptAgent returns different results on successive calls for the
// same criterion, enabling retry-at-caller-level testing.
type sequencingAcceptAgent struct {
	calls   int
	results []CriterionResult
	errs    []error
}

func (s *sequencingAcceptAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	idx := s.calls
	s.calls++
	if idx < len(s.errs) && s.errs[idx] != nil {
		return CriterionResult{}, s.errs[idx]
	}
	if idx < len(s.results) {
		return s.results[idx], nil
	}
	return CriterionResult{Status: StatusPass, Rationale: "default"}, nil
}

func TestEvaluator_RetryOnInvalidOutput(t *testing.T) {
	// First call: agent returns an invalid status (simulating unparseable/garbage output).
	// The evaluator should return an error because the status is not pass/fail/unclear.
	// Second call (retry at caller level): agent returns valid JSON with StatusPass.
	agent := &sequencingAcceptAgent{
		results: []CriterionResult{
			{Status: "GARBAGE_NOT_VALID", Rationale: "bad"},
			{Status: StatusPass, Rationale: "looks good"},
		},
	}

	eval := NewEvaluator(agent, DefaultTimeoutConfig())
	input := EvaluateInput{
		Criteria:    []string{"endpoint works"},
		DiffSummary: "added endpoint",
	}

	// First attempt: should fail due to invalid status
	_, err := eval.Evaluate(context.Background(), input)
	if err == nil {
		t.Fatal("expected error on first call with invalid status")
	}
	if !containsSubstring(err.Error(), "invalid status") {
		t.Errorf("error should mention invalid status: %v", err)
	}

	// Second attempt (caller retries): should succeed
	result, err := eval.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("expected success on retry, got: %v", err)
	}
	if !result.AllPass {
		t.Error("expected AllPass=true on retry")
	}
	if agent.calls != 2 {
		t.Errorf("expected 2 agent calls total, got %d", agent.calls)
	}
}

func TestEvaluator_AggregatesResults(t *testing.T) {
	// Three criteria: one pass, one fail, one unclear.
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"returns 200":  {Status: StatusPass, Rationale: "test proves it"},
			"handles auth": {Status: StatusFail, Rationale: "no auth middleware found"},
			"logs events":  {Status: StatusUnclear, Rationale: "logging exists but unclear if events covered"},
		},
	}

	eval := NewEvaluator(agent, DefaultTimeoutConfig())
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"returns 200", "handles auth", "logs events"},
		DiffSummary: "added handler with partial auth",
		TaskResults: "some tests pass",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if result.AllPass {
		t.Error("expected AllPass=false with mixed results")
	}
	if !result.HasFailOrUnclear {
		t.Error("expected HasFailOrUnclear=true with fail and unclear results")
	}
	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Results))
	}

	// Verify each criterion is present with correct status
	statusMap := make(map[string]string)
	for _, r := range result.Results {
		statusMap[r.Criterion] = r.Status
	}
	if statusMap["returns 200"] != StatusPass {
		t.Errorf("returns 200: got status %q, want %q", statusMap["returns 200"], StatusPass)
	}
	if statusMap["handles auth"] != StatusFail {
		t.Errorf("handles auth: got status %q, want %q", statusMap["handles auth"], StatusFail)
	}
	if statusMap["logs events"] != StatusUnclear {
		t.Errorf("logs events: got status %q, want %q", statusMap["logs events"], StatusUnclear)
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
