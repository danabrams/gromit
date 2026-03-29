package acceptor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// mockInvoker satisfies llmadapter.Invoker for testing.
type mockInvoker struct {
	result *provider.Result
	err    error
}

func (m *mockInvoker) Invoke(_ context.Context, _ string) (*provider.Result, error) {
	return m.result, m.err
}

func (m *mockInvoker) InvokeInDir(ctx context.Context, prompt string, _ string) (*provider.Result, error) {
	return m.Invoke(ctx, prompt)
}

func TestProviderAcceptAgent_ValidJSON(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Output: `{"criterion":"AC1","status":"pass","rationale":"All tests pass","evidence_refs": ["foo.go"]}`,
		},
	}
	agent := NewProviderAcceptAgent(inv)

	got, err := agent.EvaluateCriterion(context.Background(), "evaluate tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Criterion != "AC1" {
		t.Errorf("Criterion = %q, want %q", got.Criterion, "AC1")
	}
	if !(got.Status == "pass") {
		t.Errorf("Status = %q, want %q", got.Status, "pass")
	}
	if got.Rationale != "All tests pass" {
		t.Errorf("Rationale = %q, want %q", got.Rationale, "All tests pass")
	}
	if len(got.EvidenceRefs) != 1 || got.EvidenceRefs[0] != "foo.go" {
		t.Errorf("EvidenceRefs = %v, want [foo.go]", got.EvidenceRefs)
	}
	if got.Criterion == proseFallbackCriterionPlaceholder {
		t.Errorf("Criterion = %q, want actual JSON criterion (no fallback)", got.Criterion)
	}
}

func TestProviderAcceptAgent_MarkdownFencedJSON(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Output: "Here is my evaluation:\n```json\n{\"criterion\":\"lint clean\",\"status\":\"fail\",\"rationale\":\"3 warnings\",\"evidence_refs\":[\"main.go:5\"]}\n```\n",
		},
	}
	agent := NewProviderAcceptAgent(inv)

	got, err := agent.EvaluateCriterion(context.Background(), "check lint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "fail" {
		t.Errorf("Status = %q, want %q", got.Status, "fail")
	}
	if got.Criterion != "lint clean" {
		t.Errorf("Criterion = %q, want %q", got.Criterion, "lint clean")
	}
}

func TestProviderAcceptAgent_NilEvidenceRefsNormalized(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Output: `{"criterion":"c","status":"pass","rationale":"ok"}`,
		},
	}
	agent := NewProviderAcceptAgent(inv)

	got, err := agent.EvaluateCriterion(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EvidenceRefs == nil {
		t.Fatal("EvidenceRefs should be non-nil empty slice after NormalizeNilFields")
	}
	if len(got.EvidenceRefs) != 0 {
		t.Errorf("EvidenceRefs length = %d, want 0", len(got.EvidenceRefs))
	}
}

func TestProviderAcceptAgent_MalformedJSON(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Output: `{not valid json`,
		},
	}
	agent := NewProviderAcceptAgent(inv)

	_, err := agent.EvaluateCriterion(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestProviderAcceptAgent_ProseFallbackPreventsEvaluationFailure(t *testing.T) {
	cases := []struct {
		name       string
		output     string
		wantStatus string
	}{
		{
			name:       "pass",
			output:     "## Assessment: PASS\nAll requirements satisfied.",
			wantStatus: StatusPass,
		},
		{
			name:       "fail",
			output:     "## Assessment: FAIL\nNecessary behavior is missing.",
			wantStatus: StatusFail,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := &mockInvoker{
				result: &provider.Result{
					Output: tc.output,
				},
			}
			agent := NewProviderAcceptAgent(inv)

			got, err := agent.EvaluateCriterion(context.Background(), "prompt")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertProseResult(t, got, tc.wantStatus)
			if got.Rationale != tc.output {
				t.Errorf("Rationale = %q, want %q", got.Rationale, tc.output)
			}
		})
	}
}

func TestProviderAcceptAgent_InvokerErrorPropagated(t *testing.T) {
	expectedErr := errors.New("provider timeout")
	inv := &mockInvoker{
		err: expectedErr,
	}
	agent := NewProviderAcceptAgent(inv)

	_, err := agent.EvaluateCriterion(context.Background(), "prompt")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestProviderAcceptAgent_NilResultReturnsError(t *testing.T) {
	inv := &mockInvoker{
		result: nil,
		err:    nil,
	}
	agent := NewProviderAcceptAgent(inv)

	_, err := agent.EvaluateCriterion(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error for nil result, got nil")
	}
	if err.Error() != "acceptor: provider returned nil result" {
		t.Errorf("error = %q, want %q", err.Error(), "acceptor: provider returned nil result")
	}
}

func TestProviderAcceptAgent_EvaluateCriterion_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inv := &mockInvoker{
		result: nil,
		err:    context.Canceled,
	}
	agent := NewProviderAcceptAgent(inv)

	_, err := agent.EvaluateCriterion(ctx, "evaluate tests")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestParseCriterionResult_ValidJSONHappyPathUnchanged(t *testing.T) {
	output := `{"criterion":"AC1","status":"pass","rationale":"All tests pass","evidence_refs": ["foo.go"]}`
	got, err := ParseCriterionResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != StatusPass {
		t.Errorf("Status = %q, want %q", got.Status, StatusPass)
	}
	if len(got.EvidenceRefs) != 1 || got.EvidenceRefs[0] != "foo.go" {
		t.Errorf("EvidenceRefs = %v, want [foo.go]", got.EvidenceRefs)
	}
	if got.Criterion != "AC1" {
		t.Errorf("Criterion = %q, want %q", got.Criterion, "AC1")
	}
	if got.Rationale != "All tests pass" {
		t.Errorf("Rationale = %q, want %q", got.Rationale, "All tests pass")
	}
}

func TestParseCriterionResult_NormalizesNilFields(t *testing.T) {
	output := `{"criterion":"x","status":"pass","rationale":"ok"}`
	got, err := ParseCriterionResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EvidenceRefs == nil {
		t.Fatal("EvidenceRefs should be non-nil empty slice")
	}
}

func TestParseCriterionResult_MalformedJSON(t *testing.T) {
	_, err := ParseCriterionResult("not json at all")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestParseCriterionResult_InvalidStatus(t *testing.T) {
	_, err := ParseCriterionResult(`{"criterion":"x","status":"bogus","rationale":"r"}`)
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestParseCriterionResult_EmptyStatus(t *testing.T) {
	_, err := ParseCriterionResult(`{"criterion":"x","status":"","rationale":"r"}`)
	if err == nil {
		t.Fatal("expected error for empty status")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestParseCriterionResult_EmptyCriterion(t *testing.T) {
	_, err := ParseCriterionResult(`{"criterion":"","status":"pass","rationale":"r"}`)
	if err == nil {
		t.Fatal("expected error for empty criterion")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestParseCriterionResult_EmptyRationale(t *testing.T) {
	_, err := ParseCriterionResult(`{"criterion":"x","status":"pass","rationale":""}`)
	if err == nil {
		t.Fatal("expected error for empty rationale")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

func assertProseResult(t *testing.T, got CriterionResult, wantStatus string) {
	t.Helper()
	if got.Status != wantStatus {
		t.Errorf("Status = %q, want %q", got.Status, wantStatus)
	}
	if got.Criterion != proseFallbackCriterionPlaceholder {
		t.Errorf("Criterion = %q, want %q", got.Criterion, proseFallbackCriterionPlaceholder)
	}
	if got.Rationale == "" {
		t.Error("expected non-empty Rationale from prose fallback")
	}
	if got.EvidenceRefs == nil {
		t.Fatal("EvidenceRefs should be non-nil empty slice after NormalizeNilFields")
	}
	if len(got.EvidenceRefs) != 0 {
		t.Errorf("EvidenceRefs length = %d, want 0", len(got.EvidenceRefs))
	}
}

func TestParseCriterionResult_ProseSuccessStatuses(t *testing.T) {
	cases := []struct {
		name       string
		output     string
		wantStatus string
	}{
		{
			name:       "pass signal",
			output:     "## Assessment: PASS\nThe implementation meets the acceptance criterion.",
			wantStatus: StatusPass,
		},
		{
			name:       "fail signal",
			output:     "Criterion Assessment: FAIL - necessary behavior is missing.",
			wantStatus: StatusFail,
		},
		{
			name:       "not satisfied",
			output:     "The criterion is not satisfied by the implementation.",
			wantStatus: StatusFail,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCriterionResult(tc.output)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertProseResult(t, got, tc.wantStatus)
		})
	}
}

func TestParseCriterionResult_ProseConflictingSignals(t *testing.T) {
	output := "The criterion PASSED for file A but FAILED for file B."
	_, err := ParseCriterionResult(output)
	if err == nil {
		t.Fatal("expected error for conflicting prose signals")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "raw output: "+output) {
		t.Errorf("error = %q, want raw output snippet", err.Error())
	}
}

func TestParseCriterionResult_ProseNoSignalIncludesRawOutput(t *testing.T) {
	output := "I'm not sure about this criterion"
	_, err := ParseCriterionResult(output)
	if err == nil {
		t.Fatal("expected error for prose without status signal")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "raw output: "+output) {
		t.Fatalf("error = %q, want raw output snippet", err.Error())
	}
}

func TestParseCriterionResult_ParseErrorIncludesRawOutput(t *testing.T) {
	output := "{not valid json"
	_, err := ParseCriterionResult(output)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "raw output: "+output) {
		t.Fatalf("error = %q, want raw output snippet", err.Error())
	}
}

func TestParseCriterionResult_RawOutputTruncatedInParseError(t *testing.T) {
	raw := strings.Repeat("x", parseErrorRawOutputLimit+10)
	_, err := ParseCriterionResult(raw)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), raw[:parseErrorRawOutputLimit]) {
		t.Fatalf("error = %q, want truncated raw output snippet", err.Error())
	}
	if strings.Contains(err.Error(), raw) {
		t.Fatalf("error contains full raw output, want truncation to %d chars", parseErrorRawOutputLimit)
	}
}

func TestParseCriterionResult_ProseUnclear(t *testing.T) {
	output := "Insufficient evidence was provided to evaluate this criterion."
	got, err := ParseCriterionResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != StatusUnclear {
		t.Errorf("Status = %q, want %q", got.Status, StatusUnclear)
	}
	if got.Criterion != proseFallbackCriterionPlaceholder {
		t.Errorf("Criterion = %q, want %q", got.Criterion, proseFallbackCriterionPlaceholder)
	}
	if got.Rationale == "" {
		t.Error("expected non-empty Rationale from prose fallback")
	}
	if got.EvidenceRefs == nil {
		t.Fatal("EvidenceRefs should be non-nil empty slice after NormalizeNilFields")
	}
}

func TestParseCriterionResult_ProseFallbackRationaleIsTruncated(t *testing.T) {
	// Build prose >500 chars with a single clear pass signal at the start.
	prefix := "assessment: pass "
	padding := strings.Repeat("a", proseRationaleMaxLen+50)
	output := prefix + padding

	got, err := ParseCriterionResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != StatusPass {
		t.Errorf("Status = %q, want %q", got.Status, StatusPass)
	}
	if len(got.Rationale) != proseRationaleMaxLen {
		t.Errorf("Rationale length = %d, want %d", len(got.Rationale), proseRationaleMaxLen)
	}
	if got.Rationale != output[:proseRationaleMaxLen] {
		t.Errorf("Rationale = %q, want prefix of output", got.Rationale)
	}
}

func TestParseCriterionResult_ProseConflictingSignalsVariants(t *testing.T) {
	cases := []struct {
		name   string
		output string
	}{
		{
			name:   "pass and unclear",
			output: "The criterion passed but there is insufficient evidence to be certain.",
		},
		{
			name:   "fail and unclear",
			output: "The criterion failed and we cannot determine the full extent of the problem.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCriterionResult(tc.output)
			if err == nil {
				t.Fatal("expected error for conflicting prose signals")
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("expected *ParseError, got %T: %v", err, err)
			}
		})
	}
}

func TestParseCriterionResult_ValidJSONWithProseKeywordsUsesJSON(t *testing.T) {
	// Valid JSON embedded in prose that contains PASS/FAIL keywords.
	// The JSON parser should win; prose fallback must not be used.
	output := `Here is my assessment. The feature clearly PASS all checks. FAIL cases were not found.
{"criterion":"AC1","status":"pass","rationale":"All requirements met","evidence_refs":["main.go"]}`

	got, err := ParseCriterionResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Values must come from JSON, not prose fallback.
	if got.Criterion != "AC1" {
		t.Errorf("Criterion = %q, want %q (from JSON)", got.Criterion, "AC1")
	}
	if got.Rationale != "All requirements met" {
		t.Errorf("Rationale = %q, want %q (from JSON)", got.Rationale, "All requirements met")
	}
	if got.Criterion == proseFallbackCriterionPlaceholder {
		t.Error("Criterion is prose fallback placeholder — JSON path was not used")
	}
}
