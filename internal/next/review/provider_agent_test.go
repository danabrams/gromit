package review

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
	// capture what was passed
	calledWithPrompt string
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	m.calledWithPrompt = prompt
	return m.result, m.err
}

func TestProviderReviewAgent_ValidJSONFindings(t *testing.T) {
	mock := &mockInvoker{
		result: &provider.Result{
			Output: `[{"severity":"error","file":"handler.go","line":10,"description":"missing validation"},{"severity":"warning","file":"main.go","line":5,"description":"unused import"}]`,
		},
	}

	agent := NewProviderReviewAgent(mock)
	findings, err := agent.ReviewFacet(context.Background(), "spec_alignment", "review this code")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].File != "handler.go" {
		t.Errorf("findings[0].File = %q, want %q", findings[0].File, "handler.go")
	}
	if findings[0].Severity != SeverityError {
		t.Errorf("findings[0].Severity = %v, want %v", findings[0].Severity, SeverityError)
	}
	if findings[0].Line != 10 {
		t.Errorf("findings[0].Line = %d, want 10", findings[0].Line)
	}
	if findings[0].Description != "missing validation" {
		t.Errorf("findings[0].Description = %q, want %q", findings[0].Description, "missing validation")
	}
	if findings[1].File != "main.go" {
		t.Errorf("findings[1].File = %q, want %q", findings[1].File, "main.go")
	}
	if findings[1].Severity != SeverityWarning {
		t.Errorf("findings[1].Severity = %v, want %v", findings[1].Severity, SeverityWarning)
	}
	if mock.calledWithPrompt != "review this code" {
		t.Errorf("invoker received prompt %q, want %q", mock.calledWithPrompt, "review this code")
	}
}

func TestProviderReviewAgent_MarkdownFencedJSON(t *testing.T) {
	mock := &mockInvoker{
		result: &provider.Result{
			Output: "Here are the findings:\n```json\n[{\"severity\":\"info\",\"file\":\"readme.go\",\"description\":\"consider documenting\"}]\n```\n",
		},
	}

	agent := NewProviderReviewAgent(mock)
	findings, err := agent.ReviewFacet(context.Background(), "docs", "check docs")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].File != "readme.go" {
		t.Errorf("findings[0].File = %q, want %q", findings[0].File, "readme.go")
	}
}

func TestProviderReviewAgent_EmptyArray(t *testing.T) {
	mock := &mockInvoker{
		result: &provider.Result{
			Output: "[]",
		},
	}

	agent := NewProviderReviewAgent(mock)
	findings, err := agent.ReviewFacet(context.Background(), "quality", "check quality")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if findings == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestProviderReviewAgent_MalformedJSON(t *testing.T) {
	mock := &mockInvoker{
		result: &provider.Result{
			Output: "this is not json at all {{{",
		},
	}

	agent := NewProviderReviewAgent(mock)
	_, err := agent.ReviewFacet(context.Background(), "quality", "check")

	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestProviderReviewAgent_MissingRequiredField(t *testing.T) {
	// Missing "file" field
	mock := &mockInvoker{
		result: &provider.Result{
			Output: `[{"severity":"error","description":"something wrong"}]`,
		},
	}

	agent := NewProviderReviewAgent(mock)
	_, err := agent.ReviewFacet(context.Background(), "quality", "check")

	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestProviderReviewAgent_MissingDescription(t *testing.T) {
	mock := &mockInvoker{
		result: &provider.Result{
			Output: `[{"severity":"error","file":"handler.go"}]`,
		},
	}

	agent := NewProviderReviewAgent(mock)
	_, err := agent.ReviewFacet(context.Background(), "quality", "check")

	if err == nil {
		t.Fatal("expected error for missing description, got nil")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestProviderReviewAgent_InvokerErrorPropagated(t *testing.T) {
	expectedErr := errors.New("provider unavailable")
	mock := &mockInvoker{err: expectedErr}

	agent := NewProviderReviewAgent(mock)
	_, err := agent.ReviewFacet(context.Background(), "quality", "check")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
	// Must NOT be a ParseError
	var pe *ParseError
	if errors.As(err, &pe) {
		t.Errorf("invoker errors must not be wrapped as ParseError, got %v", pe)
	}
}

func TestProviderReviewAgent_NilResultReturnsError(t *testing.T) {
	mock := &mockInvoker{
		result: nil,
		err:    nil,
	}

	agent := NewProviderReviewAgent(mock)
	_, err := agent.ReviewFacet(context.Background(), "quality", "check")

	if err == nil {
		t.Fatal("expected error for nil result, got nil")
	}
	if err.Error() != "review: provider returned nil result" {
		t.Errorf("error = %q, want %q", err.Error(), "review: provider returned nil result")
	}
}

func TestProviderReviewAgent_ReviewFacet_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mock := &mockInvoker{
		result: nil,
		err:    context.Canceled,
	}

	agent := NewProviderReviewAgent(mock)
	_, err := agent.ReviewFacet(ctx, "quality", "check quality")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestProviderReviewAgent_CompileTimeInterfaceCheck(t *testing.T) {
	// The actual compile-time check is: var _ ReviewAgent = (*ProviderReviewAgent)(nil)
	// in the implementation file. This test confirms the type assertion works.
	var agent ReviewAgent = &ProviderReviewAgent{}
	_ = agent
}
