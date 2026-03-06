package decompose

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestRunErrorsWhenPlanMissing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Paths: config.PathsConfig{GromitDir: ".gromit"},
	}

	stg, err := New(cfg, &fakeLLM{}, &fakeTracker{})
	if err != nil {
		t.Fatalf("unexpected stage creation error: %v", err)
	}

	_, err = stg.Run(context.Background(), &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: "spec"},
		Config: cfg,
	})
	if err == nil {
		t.Fatal("expected error when plan is missing")
	}
	if !strings.Contains(err.Error(), "plan not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type fakeLLM struct{}

func (fakeLLM) Invoke(ctx context.Context, req llm.InvokeRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Success: true, Output: "[]"}, nil
}

func (fakeLLM) StreamInvoke(ctx context.Context, req llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Success: true, Output: "[]"}, nil
}

type fakeTracker struct{}

func (fakeTracker) NextBead(ctx context.Context) (*tasktracker.Bead, error) {
	return nil, nil
}

func (fakeTracker) CreateBead(ctx context.Context, title, description string, priority int, dependencies []string) (*tasktracker.Bead, error) {
	return &tasktracker.Bead{ID: "fake"}, nil
}

func (fakeTracker) CloseBead(ctx context.Context, beadID string) error {
	return nil
}

func (fakeTracker) QueryBeads(ctx context.Context, labels []string, status, parent string) ([]tasktracker.Bead, error) {
	return nil, nil
}
