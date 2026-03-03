package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestUnstick_WithBeadIDCallsUnstick verifies unstick with a bead ID argument calls Unstick and prints confirmation
func TestUnstick_WithBeadIDCallsUnstick(t *testing.T) {

	originalConfigPath := configPath
	configPath = filepath.Join("..", "..", "gromit.yaml")
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	pipelineCalled := false
	stubPipeline := &mockUnstickExecutor{
		UnstickFn: func(ctx context.Context, beadID, gromitDir string) error {
			pipelineCalled = true
			return nil
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
	if !strings.Contains(output, "Unsticked") {
		t.Fatalf("expected confirmation in output: %s", output)
	}
}

// TestUnstick_WithoutArgsEmptyListPrintsMessage verifies unstick without args prints "No stuck beads" when list is empty
func TestUnstick_WithoutArgsEmptyListPrintsMessage(t *testing.T) {

	originalConfigPath := configPath
	configPath = filepath.Join("..", "..", "gromit.yaml")
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	stubPipeline := &mockUnstickExecutor{
		ListStuckFn: func(ctx context.Context, input pipeline.QueueInput) ([]*bead.Bead, error) {
			return []*bead.Bead{}, nil
		},
	}

	createUnstickPipelineFn = func(cfg *config.Config, gromitDir string) (unstickExecutor, error) {
		return stubPipeline, nil
	}
	t.Cleanup(func() {
		createUnstickPipelineFn = createUnstickPipeline
	})

	output := captureStdout(t, func() {
		if err := runUnstick(unstickCmd, []string{}); err != nil {
			t.Fatalf("runUnstick returned error: %v", err)
		}
	})

	if !strings.Contains(output, "No stuck beads") {
		t.Fatalf("expected 'No stuck beads' in output: %s", output)
	}
}

// TestUnstick_WithoutArgsDisplaysNumberedList verifies unstick without args displays numbered list of stuck beads
func TestUnstick_WithoutArgsDisplaysNumberedList(t *testing.T) {
	originalConfigPath := configPath
	configPath = filepath.Join("..", "..", "gromit.yaml")
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	stubPipeline := &mockUnstickExecutor{
		ListStuckFn: func(ctx context.Context, input pipeline.QueueInput) ([]*bead.Bead, error) {
			return []*bead.Bead{
				{ID: "stuck-1", Title: "First Stuck Bead"},
				{ID: "stuck-2", Title: "Second Stuck Bead"},
			}, nil
		},
		UnstickFn: func(ctx context.Context, beadID, gromitDir string) error {
			return nil
		},
	}

	createUnstickPipelineFn = func(cfg *config.Config, gromitDir string) (unstickExecutor, error) {
		return stubPipeline, nil
	}
	t.Cleanup(func() {
		createUnstickPipelineFn = createUnstickPipeline
	})

	stdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	if _, err := w.Write([]byte("1\n")); err != nil {
		t.Fatalf("failed to write to stdin: %v", err)
	}
	w.Close()
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = stdin
	})

	output := captureStdout(t, func() {
		if err := runUnstick(unstickCmd, []string{}); err != nil {
			t.Fatalf("runUnstick returned error: %v", err)
		}
	})

	if !strings.Contains(output, "1.") || !strings.Contains(output, "First Stuck Bead") {
		t.Fatalf("expected numbered list with first bead in output: %s", output)
	}
	if !strings.Contains(output, "2.") || !strings.Contains(output, "Second Stuck Bead") {
		t.Fatalf("expected numbered list with second bead in output: %s", output)
	}
	if !strings.Contains(output, "Unsticked") {
		t.Fatalf("expected confirmation message in output: %s", output)
	}
}

// mockUnstickExecutor is a test double for the unstick executor.
type mockUnstickExecutor struct {
	UnstickFn   func(context.Context, string, string) error
	ListStuckFn func(context.Context, pipeline.QueueInput) ([]*bead.Bead, error)
}

func (m *mockUnstickExecutor) Unstick(ctx context.Context, beadID, gromitDir string) error {
	if m == nil || m.UnstickFn == nil {
		return nil
	}
	return m.UnstickFn(ctx, beadID, gromitDir)
}

func (m *mockUnstickExecutor) ListStuck(ctx context.Context, input pipeline.QueueInput) ([]*bead.Bead, error) {
	if m == nil || m.ListStuckFn == nil {
		return nil, nil
	}
	return m.ListStuckFn(ctx, input)
}
