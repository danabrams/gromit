package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

func TestRunBoard_DelegatesToPipeline(t *testing.T) {
	t.Parallel()

	originalConfigPath := configPath
	configPath = filepath.Join("..", "..", "gromit.yaml")
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	pipelineCalled := false
	stubPipeline := &mockBoardExecutor{
		BoardFn: func(ctx context.Context) (*pipeline.BoardData, error) {
			pipelineCalled = true
			return &pipeline.BoardData{
				Open: []pipeline.BeadInfo{
					{ID: "open-1", Title: "Open Task", Priority: 0},
				},
				Closed: []pipeline.BeadInfo{
					{ID: "closed-1", Title: "Closed Task", Priority: 1},
				},
			}, nil
		},
	}

	createBoardPipelineFn = func(cfg *config.Config, gromitDir string) (boardExecutor, error) {
		return stubPipeline, nil
	}
	t.Cleanup(func() {
		createBoardPipelineFn = createBoardPipeline
	})

	output := captureStdout(t, func() {
		if err := runBoard(boardCmd, nil); err != nil {
			t.Fatalf("runBoard returned error: %v", err)
		}
	})

	if !pipelineCalled {
		t.Fatal("expected pipeline.Board to be called")
	}
	if !strings.Contains(output, "open-1") || !strings.Contains(output, "Closed Task") {
		t.Fatalf("unexpected board output: %s", output)
	}
}

// mockBoardExecutor is a test double for the pipeline board executor.
type mockBoardExecutor struct {
	BoardFn func(context.Context) (*pipeline.BoardData, error)
}

func (m *mockBoardExecutor) Board(ctx context.Context) (*pipeline.BoardData, error) {
	if m == nil || m.BoardFn == nil {
		return nil, nil
	}
	return m.BoardFn(ctx)
}
