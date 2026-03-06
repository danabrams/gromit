package review_test

import (
    "context"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/v2/adapter/llm"
    "github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    "github.com/danabrams/gromit/internal/v2/stage/review"
)

func TestStageSkipsWhenReviewDisabled(t *testing.T) {
    cfg := &config.Config{Review: config.ReviewConfig{Enabled: false, Tier: "sonnet"}}
    stage, err := review.New(cfg, func(context.Context, string) (string, error) { return "", nil }, &fakeLLM{}, &fakeTracker{}, "", "", "")
    if err != nil {
        t.Fatalf("unexpected error creating stage: %v", err)
    }

    req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec-1"}, Config: cfg}
    res, err := stage.Run(context.Background(), req)
    if err != nil {
        t.Fatalf("Run() error = %v", err)
    }
    if res == nil || res.Decision != stagepkg.DecisionProceed {
        t.Fatalf("unexpected decision: %#v", res)
    }
}

type fakeLLM struct{}

func (f *fakeLLM) Invoke(context.Context, llm.InvokeRequest) (*llm.LLMResponse, error) {
    return &llm.LLMResponse{Success: true, Output: "{}"}, nil
}

func (f *fakeLLM) StreamInvoke(context.Context, llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
    return &llm.LLMResponse{Success: true, Output: ""}, nil
}

type fakeTracker struct{}

func (f *fakeTracker) NextBead(context.Context) (*tasktracker.Bead, error) { return nil, nil }
func (f *fakeTracker) CreateBead(context.Context, string, string, int, []string, []string) (*tasktracker.Bead, error) { return nil, nil }
func (f *fakeTracker) CloseBead(context.Context, string) error { return nil }
func (f *fakeTracker) QueryBeads(context.Context, []string, string, string) ([]tasktracker.Bead, error) {
    return nil, nil
}
