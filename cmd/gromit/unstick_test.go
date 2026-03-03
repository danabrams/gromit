package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestUnstick_WithBeadIDCallsUnstick verifies unstick with a bead ID argument calls Unstick and prints confirmation
func TestUnstick_WithBeadIDCallsUnstick(t *testing.T) {
	t.Parallel()

	originalConfigPath := configPath
	configPath = filepath.Join("..", "..", "gromit.yaml")
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	pipelineCalled := false
	stubPipeline := &mockUnstickExecutor{
		UnstickFn: func(ctx context.Context, beadID string) (*pipeline.UnstickResult, error) {
			pipelineCalled = true
			return &pipeline.UnstickResult{
				BeadID: beadID,
				Status: "unstuck",
			}, nil
		},
	}

	createUnstickPipelineFn = func(cfg *config.Config, gromitDir string) (unstickExecutor, error) {
		return stubPipeline, nil
	}
	t.Cleanup(func() {
		createUnstickPipelineFn = createUnstickPipeline
	})

	output := captureStdout(t, func() {
		if err := runUnstick(unstickCmd, []string{"test-bead-id"}); err != nil {
			t.Fatalf("runUnstick returned error: %v", err)
		}
	})

	if !pipelineCalled {
		t.Fatal("expected Unstick to be called")
	}
	if !strings.Contains(output, "unstuck") {
		t.Fatalf("expected confirmation in output: %s", output)
	}
}

// mockUnstickExecutor is a test double for the unstick executor.
type mockUnstickExecutor struct {
	UnstickFn func(context.Context, string) (*pipeline.UnstickResult, error)
}

func (m *mockUnstickExecutor) Unstick(ctx context.Context, beadID string) (*pipeline.UnstickResult, error) {
	if m == nil || m.UnstickFn == nil {
		return nil, nil
	}
	return m.UnstickFn(ctx, beadID)
}
