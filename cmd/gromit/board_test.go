package main

import (
    "context"
    "strings"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/pipeline"
)

func TestRunBoard_DelegatesToPipeline(t *testing.T) {
    t.Parallel()

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

    boardPipelineFactory = func(cfg *config.Config, gromitDir string) (boardExecutor, error) {
        return stubPipeline, nil
    }
    t.Cleanup(func() {
        boardPipelineFactory = createBoardPipeline
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
