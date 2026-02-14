//go:build acceptance

package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/prompt"
)

// TestPromptRenderer_IncludesRenderReviewAcceptanceTests verifies that
// the PromptRenderer interface includes the RenderReviewAcceptanceTests method.
func TestPromptRenderer_IncludesRenderReviewAcceptanceTests(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method is not in PromptRenderer interface yet

	// This test verifies the interface includes the new method by attempting
	// to call it through the interface type
	var renderer PromptRenderer = &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "mock review prompt", nil
		},
	}

	ctx := &prompt.ReviewAcceptanceTestsContext{
		BeadTitle:          "Test bead",
		BeadDescription:    "Test description",
		AcceptanceCriteria: "Test criteria",
		TestDiff:           "test diff",
	}

	result, err := renderer.RenderReviewAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "mock review prompt" {
		t.Errorf("expected 'mock review prompt', got %q", result)
	}
}

// TestPromptRenderer_RenderReviewAcceptanceTestsSignature verifies that
// the method has the correct signature.
func TestPromptRenderer_RenderReviewAcceptanceTestsSignature(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method is not in PromptRenderer interface yet

	var renderer PromptRenderer = &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			// Verify parameter type
			if ctx == nil {
				return "", nil
			}
			// Verify we can access expected fields
			_ = ctx.BeadTitle
			_ = ctx.BeadDescription
			_ = ctx.AcceptanceCriteria
			_ = ctx.TestDiff
			return "test", nil
		},
	}

	ctx := &prompt.ReviewAcceptanceTestsContext{
		BeadTitle:          "Title",
		BeadDescription:    "Description",
		AcceptanceCriteria: "Criteria",
		TestDiff:           "Diff",
	}

	// Verify return types
	result, err := renderer.RenderReviewAcceptanceTests(ctx)
	var _ string = result
	var _ error = err
}

// TestMockPromptRenderer_SupportsRenderReviewAcceptanceTests verifies that
// mockPromptRenderer implements the new method.
func TestMockPromptRenderer_SupportsRenderReviewAcceptanceTests(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTestsFn field does not exist on mockPromptRenderer yet
	// Expected failure: RenderReviewAcceptanceTests method does not exist on mockPromptRenderer yet

	called := false
	mock := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			called = true
			if ctx.BeadTitle != "Expected Title" {
				t.Errorf("expected BeadTitle='Expected Title', got %q", ctx.BeadTitle)
			}
			return "mock output", nil
		},
	}

	ctx := &prompt.ReviewAcceptanceTestsContext{
		BeadTitle:          "Expected Title",
		BeadDescription:    "Description",
		AcceptanceCriteria: "Criteria",
		TestDiff:           "Diff",
	}

	result, err := mock.RenderReviewAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Error("RenderReviewAcceptanceTestsFn was not called")
	}

	if result != "mock output" {
		t.Errorf("expected 'mock output', got %q", result)
	}
}

// TestMockPromptRenderer_RenderReviewAcceptanceTestsDefaultBehavior verifies
// that the mock returns a sensible default when the Fn field is not set.
func TestMockPromptRenderer_RenderReviewAcceptanceTestsDefaultBehavior(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method does not exist on mockPromptRenderer yet

	mock := &mockPromptRenderer{}

	ctx := &prompt.ReviewAcceptanceTestsContext{
		BeadTitle: "Test",
	}

	result, err := mock.RenderReviewAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("unexpected error from default behavior: %v", err)
	}

	// Default should return some non-empty string
	if result == "" {
		t.Error("default behavior returned empty string")
	}
}

// TestMockPromptRenderer_RenderReviewAcceptanceTestsCanReturnError verifies
// that the mock can return errors when configured to do so.
func TestMockPromptRenderer_RenderReviewAcceptanceTestsCanReturnError(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTestsFn field does not exist on mockPromptRenderer yet
	// Expected failure: RenderReviewAcceptanceTests method does not exist on mockPromptRenderer yet

	expectedErr := "test error"
	mock := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "", &mockError{msg: expectedErr}
		},
	}

	ctx := &prompt.ReviewAcceptanceTestsContext{}

	_, err := mock.RenderReviewAcceptanceTests(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

// mockError is a simple error type for testing
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

// TestPromptRenderer_AllMethodsPresent verifies that adding the new method
// doesn't break the existing interface contract.
func TestPromptRenderer_AllMethodsPresent(t *testing.T) {
	// Expected failure: RenderReviewAcceptanceTests method is not in PromptRenderer interface yet

	// Create a mock with all methods implemented
	mock := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "test", nil
		},
	}

	// Verify it still satisfies the interface
	var _ PromptRenderer = mock

	// Verify we can call both old and new methods
	ctx := &prompt.ReviewAcceptanceTestsContext{
		BeadTitle: "Test",
	}
	result, err := mock.RenderReviewAcceptanceTests(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}

	// Also verify an existing method still works
	buildCtx := &prompt.Context{}
	buildResult, err := mock.RenderBuild(buildCtx)
	if err != nil {
		t.Fatalf("unexpected error from RenderBuild: %v", err)
	}
	if buildResult == "" {
		t.Error("expected non-empty result from RenderBuild")
	}
}
