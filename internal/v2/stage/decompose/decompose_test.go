package decompose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func TestRunCreatesBeadsFromPlan(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	planPath := filepath.Join(planDir, "plan.md")
	planContent := "# Plan\nTask 1: Do the thing"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
	}

	llm := &fakeLLM{
		responses: []*llm.LLMResponse{
			{
				Success: true,
				Output: `[
                    {
                        "title": "first",
                        "description": "desc1",
                        "priority": "P1",
                        "estimated_files": 2,
                        "acceptance_criteria": ["crit1"],
                        "expected_outputs": ["out1"],
                        "depends_on_index": []
                    },
                    {
                        "title": "second",
                        "description": "desc2",
                        "priority": "P2",
                        "acceptance_criteria": ["crit2"],
                        "expected_outputs": ["out2"],
                        "depends_on_index": [0]
                    }
                ]`,
			},
		},
	}
	tracker := &fakeTracker{}
	stage, err := New(cfg, llm, tracker)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	res, err := stage.Run(context.Background(), &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: "spec"},
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", res.Decision)
	}

	artifacts, ok := res.Artifacts.(*DecomposeArtifacts)
	if !ok {
		t.Fatalf("artifacts type = %T", res.Artifacts)
	}
	if len(artifacts.Beads) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(artifacts.Beads))
	}
	if len(tracker.calls) != 2 {
		t.Fatalf("expected 2 create calls, got %d", len(tracker.calls))
	}
	if len(tracker.calls[1].Dependencies) == 0 || tracker.calls[1].Dependencies[0] != "bead-1" {
		t.Fatalf("second bead dependencies = %v", tracker.calls[1].Dependencies)
	}
	if !contains(tracker.calls[0].Labels, "spec:spec") {
		t.Fatalf("missing spec label")
	}
}

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

type fakeLLM struct {
	responses []*llm.LLMResponse
	calls     []llm.InvokeRequest
}

func (f *fakeLLM) Invoke(ctx context.Context, req llm.InvokeRequest) (*llm.LLMResponse, error) {
	f.calls = append(f.calls, req)
	if len(f.responses) == 0 {
		return nil, fmt.Errorf("no responses")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (fakeLLM) StreamInvoke(ctx context.Context, req llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Success: true, Output: "[]"}, nil
}

type fakeTracker struct {
	calls  []createCall
	nextID int
}

type createCall struct {
	Title        string
	Description  string
	Priority     int
	Labels       []string
	Dependencies []string
}

func (f *fakeTracker) NextBead(ctx context.Context) (*tasktracker.Bead, error) {
	return nil, nil
}

func (f *fakeTracker) CreateBead(ctx context.Context, title, description string, priority int, dependencies []string) (*tasktracker.Bead, error) {
	f.nextID++
	id := fmt.Sprintf("bead-%d", f.nextID)
	call := createCall{
		Title:        title,
		Description:  description,
		Priority:     priority,
		Labels:       append([]string(nil), dependencies...),
		Dependencies: append([]string(nil), dependencies...),
	}
	f.calls = append(f.calls, call)
	return &tasktracker.Bead{
		ID:          id,
		Title:       title,
		Description: description,
		Priority:    priority,
		Labels:      append([]string(nil), call.Labels...),
		DependsOn:   append([]string(nil), dependencies...),
	}, nil
}

func (f *fakeTracker) CloseBead(ctx context.Context, beadID string) error {
	return nil
}

func (f *fakeTracker) QueryBeads(ctx context.Context, labels []string, status, parent string) ([]tasktracker.Bead, error) {
	return nil, nil
}
