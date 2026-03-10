package gate

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestStageSkipsClosedBead(t *testing.T) {
	t.Parallel()

	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-closed": {ID: "bead-closed", Status: "closed"},
		},
	}

	stageInstance, err := New(&config.Config{}, tracker)
	if err != nil {
		t.Fatalf("unexpected stage creation error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "bead-closed"}})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Decision != stagepkg.DecisionSkip {
		t.Fatalf("decision = %v, want skip", res.Decision)
	}
}

func TestStageSkipsCompletedBead(t *testing.T) {
	t.Parallel()

	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-completed": {ID: "bead-completed", Status: "completed"},
		},
	}

	stageInstance, err := New(&config.Config{}, tracker)
	if err != nil {
		t.Fatalf("unexpected stage creation error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "bead-completed"}})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Decision != stagepkg.DecisionSkip {
		t.Fatalf("decision = %v, want skip", res.Decision)
	}
}

func TestStageBlocksWhenDependencyNotClosed(t *testing.T) {
	t.Parallel()

	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"blocked-bead": {
				ID:        "blocked-bead",
				Status:    "open",
				DependsOn: []string{"dep-bead"},
			},
			"dep-bead": {
				ID:     "dep-bead",
				Status: "open",
			},
		},
	}

	stageInstance, err := New(&config.Config{}, tracker)
	if err != nil {
		t.Fatalf("unexpected stage creation error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "blocked-bead"}})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Decision != stagepkg.DecisionBlock {
		t.Fatalf("decision = %v, want block", res.Decision)
	}
}

func TestStageProceedsWhenBlockedDependencyClosed(t *testing.T) {
	t.Parallel()

	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"ready-bead": {
				ID:        "ready-bead",
				Status:    "open",
				BlockedBy: []string{"dep-bead"},
			},
			"dep-bead": {
				ID:     "dep-bead",
				Status: "closed",
			},
		},
	}

	stageInstance, err := New(&config.Config{}, tracker)
	if err != nil {
		t.Fatalf("unexpected stage creation error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "ready-bead"}})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", res.Decision)
	}
}

func TestGateBlocksBeadWithBlockedByDependencies(t *testing.T) {
	t.Parallel()

	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"my-bead": {
				ID:        "my-bead",
				Status:    "open",
				BlockedBy: []string{"dep-1"},
			},
			"dep-1": {
				ID:     "dep-1",
				Status: "open",
			},
		},
	}

	stageInstance, err := New(&config.Config{}, tracker)
	if err != nil {
		t.Fatalf("unexpected stage creation error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "my-bead"}})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Decision != stagepkg.DecisionBlock {
		t.Fatalf("decision = %v, want block", res.Decision)
	}
}

func TestGateClosesSatisfiedBead(t *testing.T) {
	t.Parallel()

	const beadID = "bead-satisfied"

	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			beadID: {ID: beadID, Status: "open"},
		},
	}

	llm := &fakeLLM{responses: []string{
		`{"pass": true, "summary": "already satisfied"}`,
	}}
	git := &fakeGitDiffer{diff: "existing diff"}

	stageInstance, err := New(
		&config.Config{},
		tracker,
		WithSatisfactionCheck(llm, git),
	)
	if err != nil {
		t.Fatalf("unexpected stage creation error: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:          beadID,
			Title:       "satisfied bead",
			Description: "Acceptance Criteria\n- already done",
			Labels:      []string{"gen:1"},
		},
		Worktree: "/tmp/worktree",
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Decision != stagepkg.DecisionSkip {
		t.Fatalf("decision = %v, want skip", res.Decision)
	}
	if !tracker.closedBeads[beadID] {
		t.Fatalf("bead %s not closed", beadID)
	}
}

// fakeTaskTracker provides a minimal TaskTracker implementation for gate tests.
type fakeTaskTracker struct {
	beads       map[string]*tasktracker.Bead
	closedBeads map[string]bool
}

func (f *fakeTaskTracker) NextBead(context.Context, tasktracker.NextBeadRequest) (*tasktracker.NextBeadResponse, error) {
	return nil, nil
}

func (f *fakeTaskTracker) ShowBead(ctx context.Context, beadID string) (*tasktracker.Bead, error) {
	if f == nil || f.beads == nil {
		return nil, nil
	}
	bead, _ := f.beads[beadID]
	return bead, nil
}

func (f *fakeTaskTracker) CreateBead(context.Context, tasktracker.CreateBeadRequest) (*tasktracker.CreateBeadResponse, error) {
	return &tasktracker.CreateBeadResponse{}, nil
}

func (f *fakeTaskTracker) CloseBead(_ context.Context, req tasktracker.CloseBeadRequest) (*tasktracker.CloseBeadResponse, error) {
	if f.closedBeads == nil {
		f.closedBeads = make(map[string]bool)
	}
	f.closedBeads[req.BeadID] = true
	return &tasktracker.CloseBeadResponse{Closed: true}, nil
}

func (f *fakeTaskTracker) QueryBeads(context.Context, tasktracker.QueryBeadsRequest) (*tasktracker.QueryBeadsResponse, error) {
	return &tasktracker.QueryBeadsResponse{}, nil
}

// fakeGitDiffer implements SatisfactionDiffer for gate tests.
type fakeGitDiffer struct {
	diff string
}

func (f *fakeGitDiffer) DiffFromBase(_ context.Context, _ string) (string, error) {
	return f.diff, nil
}

// gateFakeLLM implements llmtypes.LLMProvider for gate tests.
type gateFakeLLM struct {
	responses []string
	callIndex int
	called    bool
}

func (f *gateFakeLLM) Invoke(_ context.Context, req llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	f.called = true
	if f.callIndex >= len(f.responses) {
		return nil, fmt.Errorf("unexpected call %d", f.callIndex)
	}
	resp := f.responses[f.callIndex]
	f.callIndex++
	return &llmtypes.LLMInvokeResponse{
		Success: true,
		Output:  resp,
	}, nil
}

func (f *gateFakeLLM) StreamInvoke(_ context.Context, _ llmtypes.LLMStreamInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestGateSatisfactionCheck_Gen0_SkipsCheck(t *testing.T) {
	t.Parallel()

	llm := &gateFakeLLM{responses: []string{`{"pass": true, "summary": "ok"}`}}
	git := &fakeGitDiffer{diff: "some diff"}
	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-gen0": {
				ID:          "bead-gen0",
				Status:      "open",
				Title:       "Implement feature",
				Description: "## Acceptance Criteria\n- feature works",
				Labels:      []string{"gen:0"},
			},
		},
	}

	stageInstance, err := New(&config.Config{}, tracker, WithSatisfactionCheck(llm, git))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "bead-gen0", Title: "Implement feature", Description: "## Acceptance Criteria\n- feature works", Labels: []string{"gen:0"}},
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", res.Decision)
	}
	if llm.called {
		t.Error("LLM should not have been called for gen:0 bead")
	}
}

func TestGateSatisfactionCheck_Gen1_SatisfiedBeadSkipped(t *testing.T) {
	t.Parallel()

	llm := &gateFakeLLM{responses: []string{`{"pass": true, "summary": "criterion met"}`}}
	git := &fakeGitDiffer{diff: "some meaningful diff"}
	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-gen1": {
				ID:          "bead-gen1",
				Status:      "open",
				Title:       "Implement feature",
				Description: "## Acceptance Criteria\n- feature A works",
				Labels:      []string{"gen:1"},
			},
		},
	}

	stageInstance, err := New(&config.Config{}, tracker, WithSatisfactionCheck(llm, git))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "bead-gen1", Title: "Implement feature", Description: "## Acceptance Criteria\n- feature A works", Labels: []string{"gen:1"}},
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionSkip {
		t.Fatalf("decision = %v, want skip", res.Decision)
	}
	if !tracker.closedBeads["bead-gen1"] {
		t.Error("expected bead to be closed via tracker")
	}
}

func TestGateSatisfactionCheck_NilLLM_SkipsCheck(t *testing.T) {
	t.Parallel()

	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-gen1": {
				ID:          "bead-gen1",
				Status:      "open",
				Title:       "Implement feature",
				Description: "## Acceptance Criteria\n- feature A works",
				Labels:      []string{"gen:1"},
			},
		},
	}

	// No WithSatisfactionCheck option
	stageInstance, err := New(&config.Config{}, tracker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "bead-gen1", Title: "Implement feature", Description: "## Acceptance Criteria\n- feature A works", Labels: []string{"gen:1"}},
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", res.Decision)
	}
}

func TestGateSatisfactionCheck_RefactorBead_SkipsCheck(t *testing.T) {
	t.Parallel()

	llm := &gateFakeLLM{responses: []string{`{"pass": true, "summary": "ok"}`}}
	git := &fakeGitDiffer{diff: "some diff"}
	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-refactor": {
				ID:          "bead-refactor",
				Status:      "open",
				Title:       "Refactor debug command",
				Description: "## Acceptance Criteria\n- code is cleaner",
				Labels:      []string{"gen:1"},
			},
		},
	}

	stageInstance, err := New(&config.Config{}, tracker, WithSatisfactionCheck(llm, git))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "bead-refactor", Title: "Refactor debug command", Description: "## Acceptance Criteria\n- code is cleaner", Labels: []string{"gen:1"}},
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", res.Decision)
	}
	if llm.called {
		t.Error("LLM should not have been called for structural bead")
	}
}

func TestGateSatisfactionCheck_UnsatisfiedBead_Proceeds(t *testing.T) {
	t.Parallel()

	llm := &gateFakeLLM{responses: []string{`{"pass": false, "summary": "not done"}`}}
	git := &fakeGitDiffer{diff: "some diff"}
	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-gen1": {
				ID:          "bead-gen1",
				Status:      "open",
				Title:       "Implement feature",
				Description: "## Acceptance Criteria\n- feature A works",
				Labels:      []string{"gen:1"},
			},
		},
	}

	stageInstance, err := New(&config.Config{}, tracker, WithSatisfactionCheck(llm, git))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "bead-gen1", Title: "Implement feature", Description: "## Acceptance Criteria\n- feature A works", Labels: []string{"gen:1"}},
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", res.Decision)
	}
}

func TestExtractCriteria(t *testing.T) {
	t.Parallel()

	input := "## Acceptance Criteria\n- feature A works\n- feature B works\n\n## Other"
	got := extractCriteria(input)

	if len(got) != 2 {
		t.Fatalf("got %d criteria, want 2: %v", len(got), got)
	}
	if got[0] != "feature A works" {
		t.Errorf("criteria[0] = %q, want %q", got[0], "feature A works")
	}
	if got[1] != "feature B works" {
		t.Errorf("criteria[1] = %q, want %q", got[1], "feature B works")
	}
}
