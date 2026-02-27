package main

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// mockBeadClient implements a simple mock of bead operations
type mockBeadClient struct {
	capturedContexts []context.Context
	beads            []*bead.Bead
}

func (m *mockBeadClient) ListWithLabel(ctx context.Context, label string) ([]*bead.Bead, error) {
	m.capturedContexts = append(m.capturedContexts, ctx)
	return m.beads, nil
}


func TestBuildBeadFilterWithClient_ThreadsProvidedContext(t *testing.T) {
	t.Parallel()

	// Create a custom context with a value to verify it's threaded through
	testValue := "test-context-value"
	customCtx := context.WithValue(context.Background(), "test-key", testValue)

	// Create mock client
	mockClient := &mockBeadClient{
		beads: []*bead.Bead{
			{ID: "bead-1", Title: "Test Bead 1"},
			{ID: "bead-2", Title: "Test Bead 2"},
		},
	}

	// Call testable version with custom context
	result, err := buildBeadFilterWithClient(customCtx, []string{"test-label"}, mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify filter was built correctly
	if len(result) != 2 || !result["bead-1"] || !result["bead-2"] {
		t.Fatalf("unexpected filter result: %v", result)
	}

	// Verify that the context was passed to ListWithLabel (not context.Background())
	if len(mockClient.capturedContexts) != 1 {
		t.Fatalf("expected 1 ListWithLabel call, got %d", len(mockClient.capturedContexts))
	}

	capturedCtx := mockClient.capturedContexts[0]
	// Verify the context value is available (proves we passed the right context)
	if capturedCtx.Value("test-key") != testValue {
		t.Fatal("context value not found in captured context - buildBeadFilterWithClient not threading context properly")
	}
}

func TestHasOpenBeadsForLabelWithClient_ThreadsProvidedContext(t *testing.T) {
	t.Parallel()

	// Create a custom context with a value to verify it's threaded through
	testValue := "test-context-value"
	customCtx := context.WithValue(context.Background(), "test-key", testValue)

	// Create mock client
	mockClient := &mockBeadClient{
		beads: []*bead.Bead{
			{ID: "bead-1", Title: "Test Bead"},
		},
	}

	// Call testable version with custom context
	hasBeads, err := hasOpenBeadsForLabelWithClient(customCtx, "test-label", mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify result
	if !hasBeads {
		t.Fatalf("expected hasBeads to be true, got false")
	}

	// Verify that the context was passed to ListWithLabel
	if len(mockClient.capturedContexts) != 1 {
		t.Fatalf("expected 1 ListWithLabel call, got %d", len(mockClient.capturedContexts))
	}

	capturedCtx := mockClient.capturedContexts[0]
	// Verify the context value is available (proves we passed the right context)
	if capturedCtx.Value("test-key") != testValue {
		t.Fatal("context value not found in captured context - hasOpenBeadsForLabelWithClient not threading context properly")
	}
}