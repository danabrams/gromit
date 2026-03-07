package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/presentation"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func requireStageSequence(t *testing.T, recorder *recordingStageRecorder) {
	t.Helper()
	if got := recorder.stageNames(); !reflect.DeepEqual(got, StageSequence) {
		t.Fatalf("stage order = %v, want %v", got, StageSequence)
	}
}

func collectEvents(t *testing.T, ch chan events.Event, target int) []events.Event {
	t.Helper()
	collected := make([]events.Event, 0, target)
	deadline := time.After(time.Second)
	for len(collected) < target {
		select {
		case evt := <-ch:
			collected = append(collected, evt)
		case <-deadline:
			t.Fatalf("timed out waiting for events, got %d", len(collected))
		}
	}
	return collected
}

type recordingStageRecorder struct {
	mu    sync.Mutex
	names []string
}

func newRecordingStageRecorder() *recordingStageRecorder {
	return &recordingStageRecorder{}
}

func (r *recordingStageRecorder) RecordStage(name string) {
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
}

func (r *recordingStageRecorder) stageNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, len(r.names))
	copy(names, r.names)
	return names
}

type fakeGitAdapter struct {
	t                  *testing.T
	lastWorktree       string
	gapAnalysisContent string
	planContent        string
	commitMessages     []string
	removedWorktrees   []string
	statusCalls        []string
}

func newFakeGitAdapter(t *testing.T) *fakeGitAdapter {
	return &fakeGitAdapter{t: t}
}

func (f *fakeGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
	f.t.Helper()
	worktree := filepath.Join(f.t.TempDir(), specID)
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		f.t.Fatalf("mkdir worktree: %v", err)
	}
	if f.gapAnalysisContent != "" {
		gromitPath := filepath.Join(worktree, ".gromit", "v2")
		if err := os.MkdirAll(gromitPath, 0o755); err != nil {
			f.t.Fatalf("create gromit dir: %v", err)
		}
		path := filepath.Join(gromitPath, "gap-analysis.md")
		if err := os.WriteFile(path, []byte(f.gapAnalysisContent), 0o644); err != nil {
			f.t.Fatalf("write gap analysis: %v", err)
		}
	}
	if f.planContent != "" {
		planPath := filepath.Join(worktree, ".gromit", "v2")
		if err := os.MkdirAll(planPath, 0o755); err != nil {
			f.t.Fatalf("create plan dir: %v", err)
		}
		path := filepath.Join(planPath, "plan.md")
		if err := os.WriteFile(path, []byte(f.planContent), 0o644); err != nil {
			f.t.Fatalf("write plan content: %v", err)
		}
	}
	f.lastWorktree = worktree
	return worktree, nil
}

func (f *fakeGitAdapter) Diff(context.Context, string) (string, error) {
	return "", nil
}

func (f *fakeGitAdapter) Commit(_ context.Context, worktree, message string) (string, error) {
	f.commitMessages = append(f.commitMessages, message)
	return "fake-commit", nil
}

func (f *fakeGitAdapter) RemoveWorktree(_ context.Context, worktree string) error {
	f.removedWorktrees = append(f.removedWorktrees, worktree)
	return os.RemoveAll(worktree)
}

func (f *fakeGitAdapter) Status(_ context.Context, worktree string) (string, error) {
	f.statusCalls = append(f.statusCalls, worktree)
	return "", nil
}

func (f *fakeGitAdapter) Log(_ context.Context, _ string, _ int) ([]adapter.LogEntry, error) {
	return nil, nil
}

func (f *fakeGitAdapter) Show(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (f *fakeGitAdapter) SquashCommits(_ context.Context, _ string, _ int) error {
	return nil
}

type fakeLLMAdapter struct{}

func newFakeLLMAdapter() *fakeLLMAdapter {
	return &fakeLLMAdapter{}
}

func (f *fakeLLMAdapter) GeneratePlan(_ context.Context, specID string) (string, error) {
	return specID + "-plan", nil
}

func (f *fakeLLMAdapter) Invoke(_ context.Context, req llm.LLMInvokeRequest) (*llm.LLMInvokeResponse, error) {
	return &llm.LLMInvokeResponse{Success: true, Output: "fake-output"}, nil
}

func (f *fakeLLMAdapter) StreamInvoke(_ context.Context, req llm.LLMStreamInvokeRequest) (*llm.LLMStreamInvokeResponse, error) {
	return &llm.LLMStreamInvokeResponse{Success: true, Output: "fake-output"}, nil
}

func (f *fakeLLMAdapter) planFor(specID string) string {
	return specID + "-plan"
}

type fakeTaskTrackerAdapter struct {
	queryBeadsResponse *tasktracker.TaskTrackerQueryBeadsResponse
}

func newFakeTaskTrackerAdapter() *fakeTaskTrackerAdapter {
	return &fakeTaskTrackerAdapter{}
}

func (f *fakeTaskTrackerAdapter) NextBead(_ context.Context, _ tasktracker.TaskTrackerNextBeadRequest) (*tasktracker.TaskTrackerNextBeadResponse, error) {
	return &tasktracker.TaskTrackerNextBeadResponse{}, nil
}

func (f *fakeTaskTrackerAdapter) ShowBead(_ context.Context, _ string) (*tasktracker.Bead, error) {
	return nil, fmt.Errorf("bead not found")
}

func (f *fakeTaskTrackerAdapter) CreateBead(_ context.Context, _ tasktracker.TaskTrackerCreateBeadRequest) (*tasktracker.TaskTrackerCreateBeadResponse, error) {
	return &tasktracker.TaskTrackerCreateBeadResponse{}, nil
}

func (f *fakeTaskTrackerAdapter) CloseBead(_ context.Context, _ tasktracker.TaskTrackerCloseBeadRequest) (*tasktracker.TaskTrackerCloseBeadResponse, error) {
	return &tasktracker.TaskTrackerCloseBeadResponse{Closed: true}, nil
}

func (f *fakeTaskTrackerAdapter) QueryBeads(_ context.Context, _ tasktracker.TaskTrackerQueryBeadsRequest) (*tasktracker.TaskTrackerQueryBeadsResponse, error) {
	if f.queryBeadsResponse != nil {
		return f.queryBeadsResponse, nil
	}
	return &tasktracker.TaskTrackerQueryBeadsResponse{}, nil
}

type fakePresenterAdapter struct {
	t                *testing.T
	lastSummary      presentation.PresentationSummary
	planFileVerified bool
}

func newFakePresenterAdapter(t *testing.T) *fakePresenterAdapter {
	return &fakePresenterAdapter{t: t}
}

func (f *fakePresenterAdapter) PresentSummary(ctx context.Context, specID string, summary presentation.PresentationSummary) error {
	f.t.Helper()
	f.lastSummary = summary
	planPath := filepath.Join(summary.Worktree, ".gromit", "v2", "plan.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		f.t.Fatalf("read plan file: %v", err)
	}
	if string(data) != summary.Plan {
		f.t.Fatalf("plan file mismatch = %q, want %q", string(data), summary.Plan)
	}
	f.planFileVerified = true
	return nil
}

type noopDependencyGate struct{}

func (noopDependencyGate) EnsureSpecReady(ctx context.Context, specID string) error {
	return nil
}

type fakeRemediationRunner struct {
	calls int
	err   error
}

func (f *fakeRemediationRunner) Run(_ context.Context, _ string) error {
	f.calls++
	return f.err
}

type scriptedAcceptStage struct {
	calls   int
	results []stagepkg.Result
	err     error
}

func newScriptedAcceptStage(results ...stagepkg.Result) *scriptedAcceptStage {
	copied := append([]stagepkg.Result(nil), results...)
	return &scriptedAcceptStage{results: copied}
}

func (s *scriptedAcceptStage) Name() string { return "accept" }

func (s *scriptedAcceptStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if len(s.results) == 0 {
		return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
	}
	res := s.results[0]
	s.results = s.results[1:]
	result := res
	return &result, nil
}
