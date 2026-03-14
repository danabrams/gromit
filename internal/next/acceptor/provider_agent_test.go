package acceptor

import (
	"context"
	"errors"
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
			Output: `{"criterion":"tests pass","status":"pass","rationale":"all green","evidence_refs":["test.go:10"]}`,
		},
	}
	agent := NewProviderAcceptAgent(inv)

	got, err := agent.EvaluateCriterion(context.Background(), "evaluate tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Criterion != "tests pass" {
		t.Errorf("Criterion = %q, want %q", got.Criterion, "tests pass")
	}
	if got.Status != "pass" {
		t.Errorf("Status = %q, want %q", got.Status, "pass")
	}
	if got.Rationale != "all green" {
		t.Errorf("Rationale = %q, want %q", got.Rationale, "all green")
	}
	if len(got.EvidenceRefs) != 1 || got.EvidenceRefs[0] != "test.go:10" {
		t.Errorf("EvidenceRefs = %v, want [test.go:10]", got.EvidenceRefs)
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

func TestParseCriterionResult_Valid(t *testing.T) {
	output := `{"criterion":"x","status":"unclear","rationale":"maybe","evidence_refs":["a.go"]}`
	got, err := ParseCriterionResult(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "unclear" {
		t.Errorf("Status = %q, want %q", got.Status, "unclear")
	}
	if len(got.EvidenceRefs) != 1 {
		t.Errorf("EvidenceRefs length = %d, want 1", len(got.EvidenceRefs))
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
