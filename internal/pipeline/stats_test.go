package pipeline

import (
	"context"
	"testing"
)

func TestPipelineStatsAggregatesIdeaTypes(t *testing.T) {
	ctx := context.Background()
	ideas := []*Idea{
		{ID: "idea-1", Text: "Add cost tracing", Type: "feature"},
		{ID: "idea-2", Text: "Fix a crash", Type: ""},
		{ID: "idea-3", Text: "Refactor the auth layer", Type: "unknown"},
	}

	client := &fakeBacklogClient{ideas: ideas}
	p := &Pipeline{
		deps:  &Deps{BacklogClient: client},
		paths: &Paths{GromitDir: t.TempDir()},
	}

	stats, err := p.Stats(ctx, nil, StatsOptions{})
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	if stats.Backlog == nil {
		t.Fatal("backlog summary missing")
	}

	if stats.Backlog.Total != len(ideas) {
		t.Fatalf("Total = %d, want %d", stats.Backlog.Total, len(ideas))
	}

	if stats.Backlog.ByType["feature"] != 1 {
		t.Errorf("feature count = %d, want 1", stats.Backlog.ByType["feature"])
	}
	if stats.Backlog.ByType["bug"] != 1 {
		t.Errorf("bug count = %d, want 1", stats.Backlog.ByType["bug"])
	}
	if stats.Backlog.ByType["chore"] != 1 {
		t.Errorf("chore count = %d, want 1", stats.Backlog.ByType["chore"])
	}
}

type fakeBacklogClient struct {
	ideas []*Idea
}

func (f *fakeBacklogClient) List() ([]*Idea, error) {
	return f.ideas, nil
}

func (f *fakeBacklogClient) Get(id string) (*Idea, error) {
	for _, idea := range f.ideas {
		if idea.ID == id {
			return idea, nil
		}
	}
	return nil, nil
}

func (f *fakeBacklogClient) Add(item *Idea) error {
	return nil
}

func (f *fakeBacklogClient) Update(id string, fn func(*Idea)) error {
	return nil
}
