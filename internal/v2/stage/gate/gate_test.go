package gate

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
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

// fakeTaskTracker provides a minimal TaskTracker implementation for gate tests.
type fakeTaskTracker struct {
	beads map[string]*tasktracker.Bead
}

func (f *fakeTaskTracker) NextBead(context.Context) (*tasktracker.Bead, error) {
	return nil, nil
}

func (f *fakeTaskTracker) ShowBead(ctx context.Context, beadID string) (*tasktracker.Bead, error) {
	if f == nil || f.beads == nil {
		return nil, nil
	}
	bead, _ := f.beads[beadID]
	return bead, nil
}

func (f *fakeTaskTracker) CreateBead(context.Context, string, string, int, []string, []string) (*tasktracker.Bead, error) {
	return nil, nil
}

func (f *fakeTaskTracker) CloseBead(context.Context, string) error {
	return nil
}

func (f *fakeTaskTracker) QueryBeads(context.Context, []string, string, string) ([]tasktracker.Bead, error) {
	return nil, nil
}
