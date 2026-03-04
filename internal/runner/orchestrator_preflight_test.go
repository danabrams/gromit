package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner/specbranch"
)

// dirtyPreflightChecker simulates a dirty worktree at preflight time.
type dirtyPreflightChecker struct{}

func (d *dirtyPreflightChecker) EnsureWorktreeClean(ctx context.Context) error {
	return &specbranch.DirtyWorktreeError{
		RepoDir: "/fake/repo",
		Status:  "M dirty.go",
	}
}

// cleanPreflightChecker simulates a clean worktree at preflight time.
type cleanPreflightChecker struct{}

func (c *cleanPreflightChecker) EnsureWorktreeClean(ctx context.Context) error {
	return nil
}

// TestOrchestrator_PreflightDirtyWorktree_ReturnsEnvironmentBlocked verifies that
// Run() returns an error containing "environment_blocked" when the preflight
// worktree check detects a dirty worktree.
func TestOrchestrator_PreflightDirtyWorktree_ReturnsEnvironmentBlocked(t *testing.T) {
	t.Parallel()

	buildCalled := false
	beadCount := 0
	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			buildCalled = true
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		PreflightCheck: &dirtyPreflightChecker{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{ID: "b-preflight-dirty", Title: "Preflight Dirty"}, nil
			}
			return nil, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	err := o.Run(context.Background(), 1, time.Time{}, nil)

	if err == nil {
		t.Fatal("expected Run() to return an error for dirty worktree at preflight")
	}
	if !strings.Contains(err.Error(), "environment_blocked") {
		t.Errorf("error %q does not contain 'environment_blocked'", err.Error())
	}
	if !strings.Contains(err.Error(), "dirty worktree at run start") {
		t.Errorf("error %q does not contain 'dirty worktree at run start'", err.Error())
	}
	if buildCalled {
		t.Error("Build stage should not run when preflight worktree check fails")
	}
}

// TestOrchestrator_PreflightCleanWorktree_ProceedsNormally verifies that
// Run() proceeds to process beads when the preflight worktree check passes.
func TestOrchestrator_PreflightCleanWorktree_ProceedsNormally(t *testing.T) {
	t.Parallel()

	buildCalled := false
	beadCount := 0
	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			buildCalled = true
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		PreflightCheck: &cleanPreflightChecker{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{ID: "b-preflight-clean", Title: "Preflight Clean"}, nil
			}
			return nil, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	err := o.Run(context.Background(), 1, time.Time{}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !buildCalled {
		t.Error("Build stage should have been called when preflight worktree check passes")
	}
}
