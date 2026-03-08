//go:build integration

package gate

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestIntegration_GateClosesAlreadySatisfiedBead(t *testing.T) {
	t.Parallel()

	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"stale-bead": {
				ID:     "stale-bead",
				Status: "open",
				Labels: []string{"gen:1", "spec:test-spec"},
			},
		},
		closedBeads: make(map[string]bool),
	}

	llm := &gateFakeLLM{
		responses: []string{`{"pass": true, "summary": "debug command already exists in debug2.go"}`},
	}
	git := &fakeGitDiffer{diff: "+func (a *ExecGitAdapter) Debug2(...) { // implementation }"}

	stageInstance, err := New(&config.Config{}, tracker, WithSatisfactionCheck(llm, git))
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:          "stale-bead",
			Title:       "Implement debug command entry point",
			Description: "## Acceptance Criteria\n- debug command can diagnose failures from event log",
			Labels:      []string{"gen:1"},
		},
		Worktree: "/tmp/wt",
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if res.Decision != stagepkg.DecisionSkip {
		t.Fatalf("stale bead should be skipped, got %v", res.Decision)
	}

	if !tracker.closedBeads["stale-bead"] {
		t.Fatal("expected stale bead to be closed via tracker")
	}
}
