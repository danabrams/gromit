package runner

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner/specbranch"
)

// mockBranchRouter records calls to Resolve
type mockBranchRouter struct {
	calls [][]string
}

func (m *mockBranchRouter) BranchForLabels(labels []string) (string, error) {
	m.calls = append(m.calls, labels)
	return specbranch.NewRouter("main").BranchForLabels(labels)
}

// mockGitCheckout records calls to CreateOrCheckoutSpecBranch
type mockGitCheckout struct {
	calls []string
}

func (m *mockGitCheckout) CreateOrCheckoutSpecBranch(ctx context.Context, specBranchName string) error {
	m.calls = append(m.calls, specBranchName)
	return nil
}

// mockStage is a minimal pipeline Stage for testing
type mockStage struct {
	decision pipeline.Decision
}

func (m *mockStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	return pipeline.Output{Decision: m.decision}, nil
}

// TestOrchestratorCheckoutCalledAfterGateProceed verifies checkout is executed
// after Gate Proceed and before Build.
func TestOrchestratorCheckoutCalledAfterGateProceed(t *testing.T) {
	t.Parallel()

	mockRouter := &mockBranchRouter{}
	mockCheckout := &mockGitCheckout{}

	beadCount := 0
	cfg := OrchestratorConfig{
		Gate:         &mockStage{decision: pipeline.Proceed},
		Build:        &mockStage{decision: pipeline.Proceed},
		Validate:     &mockStage{decision: pipeline.Proceed},
		Epilogue:     &mockStage{},
		BranchRouter: mockRouter,
		GitCheckout:  mockCheckout,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{
					ID:     "test-bead",
					Title:  "Test Bead",
					Labels: []string{"spec:auth"},
				}, nil
			}
			return nil, nil // Only one bead
		},
	}

	o := NewOrchestrator(cfg)
	ctx := context.Background()

	// Run one iteration
	_ = o.Run(ctx, 1, time.Time{}, make(chan struct{}))

	// Verify that checkout was called
	if len(mockCheckout.calls) == 0 {
		t.Error("GitCheckout.CreateOrCheckoutSpecBranch was not called")
	}
}

func TestOrchestratorCheckoutSkippedForNonSpecBead(t *testing.T) {
	t.Parallel()

	mockRouter := &mockBranchRouter{}
	mockCheckout := &mockGitCheckout{}

	beadCount := 0
	cfg := OrchestratorConfig{
		Gate:         &mockStage{decision: pipeline.Proceed},
		Build:        &mockStage{decision: pipeline.Proceed},
		Validate:     &mockStage{decision: pipeline.Proceed},
		Epilogue:     &mockStage{},
		BranchRouter: mockRouter,
		GitCheckout:  mockCheckout,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{
					ID:     "test-bead",
					Title:  "Test Bead",
					Labels: []string{"from-review"},
				}, nil
			}
			return nil, nil
		},
	}

	o := NewOrchestrator(cfg)
	ctx := context.Background()

	_ = o.Run(ctx, 1, time.Time{}, make(chan struct{}))

	if len(mockRouter.calls) != 0 {
		t.Fatalf("BranchRouter should not be called for non-spec beads, got %d call(s)", len(mockRouter.calls))
	}
	if len(mockCheckout.calls) != 0 {
		t.Fatalf("GitCheckout should not be called for non-spec beads, got %d call(s)", len(mockCheckout.calls))
	}
}
