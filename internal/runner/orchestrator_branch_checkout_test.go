package runner

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner/specbranch"
)

// mockBranchRouter records calls to Resolve
type mockBranchRouter struct {
	calls              [][]string
	sessionModeEnabled bool
}

func (m *mockBranchRouter) BranchForLabels(labels []string) (string, error) {
	m.calls = append(m.calls, labels)
	if m.sessionModeEnabled && !specbranch.HasSpecLabel(labels) {
		return "", nil
	}
	return specbranch.NewRouter("main").BranchForLabels(labels)
}

func (m *mockBranchRouter) EnableSessionWorktreeMode() {
	m.sessionModeEnabled = true
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
	called   bool
}

func (m *mockStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	m.called = true
	return pipeline.Output{Decision: m.decision}, nil
}

type dirtyWorktreeCheckout struct{}

func (d *dirtyWorktreeCheckout) CreateOrCheckoutSpecBranch(ctx context.Context, specBranchName string) error {
	return &specbranch.DirtyWorktreeError{
		RepoDir: specBranchName,
		Status:  "M dirty.go",
	}
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

func TestOrchestratorSessionWorktreeSkipsNonSpecCheckout(t *testing.T) {
	t.Parallel()

	mockRouter := &mockBranchRouter{}
	mockCheckout := &mockGitCheckout{}

	beadCount := 0
	cfg := OrchestratorConfig{
		Gate:            &mockStage{decision: pipeline.Proceed},
		Build:           &mockStage{decision: pipeline.Proceed},
		Validate:        &mockStage{decision: pipeline.Proceed},
		Epilogue:        &mockStage{},
		BranchRouter:    mockRouter,
		GitCheckout:     mockCheckout,
		SessionWorktree: true,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{
					ID:     "session-bead",
					Title:  "Session Bead",
					Labels: []string{"from-session"},
				}, nil
			}
			return nil, nil
		},
	}

	o := NewOrchestrator(cfg)
	ctx := context.Background()

	_ = o.Run(ctx, 1, time.Time{}, make(chan struct{}))

	if len(mockRouter.calls) == 0 {
		t.Fatal("BranchRouter should be called in session worktree mode")
	}
	if len(mockCheckout.calls) != 0 {
		t.Fatalf("GitCheckout should not be called for non-spec beads in session worktree mode, got %d call(s)", len(mockCheckout.calls))
	}
	if !mockRouter.sessionModeEnabled {
		t.Fatal("BranchRouter should be enabled for session worktree mode")
	}
}

func TestOrchestratorHaltsWhenCheckoutDirtyWorktree(t *testing.T) {
	t.Parallel()

	gateStage := &mockStage{decision: pipeline.Proceed}
	buildStage := &mockStage{decision: pipeline.Proceed}
	checkout := &dirtyWorktreeCheckout{}
	beadCount := 0
	cfg := OrchestratorConfig{
		Gate:         gateStage,
		Build:        buildStage,
		Validate:     &mockStage{},
		Epilogue:     &mockStage{},
		BranchRouter: &mockBranchRouter{},
		GitCheckout:  checkout,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{
					ID:     "dirty",
					Title:  "Dirty Bead",
					Labels: []string{"spec:dirty"},
				}, nil
			}
			return nil, nil
		},
	}

	o := NewOrchestrator(cfg)
	err := o.Run(context.Background(), 1, time.Time{}, make(chan struct{}))
	if err == nil {
		t.Fatal("expected orchestrator run to fail when checkout blocked by dirty worktree")
	}
	var dirtyErr *specbranch.DirtyWorktreeError
	if !errors.As(err, &dirtyErr) {
		t.Fatalf("expected DirtyWorktreeError, got %T: %v", err, err)
	}
	if buildStage.called {
		t.Fatal("build stage should not run when checkout precondition fails")
	}
}

func TestOrchestratorLogsActionableMessageOnDirtyWorktree(t *testing.T) {
	t.Parallel()

	gateStage := &mockStage{decision: pipeline.Proceed}
	buildStage := &mockStage{decision: pipeline.Proceed}
	checkout := &dirtyWorktreeCheckout{}
	buf := &bytes.Buffer{}
	beadCount := 0
	cfg := OrchestratorConfig{
		Gate:         gateStage,
		Build:        buildStage,
		Validate:     &mockStage{},
		Epilogue:     &mockStage{},
		BranchRouter: &mockBranchRouter{},
		GitCheckout:  checkout,
		Output:       buf,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{
					ID:     "dirty",
					Title:  "Dirty Bead",
					Labels: []string{"spec:dirty"},
				}, nil
			}
			return nil, nil
		},
	}

	o := NewOrchestrator(cfg)
	o.startSubscribersFn = func(ctx context.Context) (*sync.WaitGroup, error) { return nil, nil }
	err := o.Run(context.Background(), 1, time.Time{}, make(chan struct{}))
	if err == nil {
		t.Fatal("expected orchestrator run to fail when checkout blocked by dirty worktree")
	}
	output := buf.String()
	if !strings.Contains(output, "dirty worktree precondition") {
		t.Fatalf("log output missing dirty worktree guidance: %q", output)
	}
	if !strings.Contains(output, "session worktree mode") {
		t.Fatalf("log output missing session worktree hint: %q", output)
	}
}
