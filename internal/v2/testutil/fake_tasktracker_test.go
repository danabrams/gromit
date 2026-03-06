package testutil

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/tracker"
)

var _ = tracker.StatusOpen

func TestFakeTaskTracker_CreatesBeadsAndObservesDependencies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fake := NewFakeTaskTracker()

	first, err := fake.CreateBead(ctx, "first", "desc", 1, []string{"alpha"}, nil)
	if err != nil {
		t.Fatalf("CreateBead failed: %v", err)
	}
	if first == nil {
		t.Fatal("CreateBead returned nil bead")
	}

	second, err := fake.CreateBead(ctx, "second", "desc", 2, []string{"beta"}, []string{first.ID})
	if err != nil {
		t.Fatalf("CreateBead failed: %v", err)
	}

	refreshed, err := fake.ShowBead(ctx, first.ID)
	if err != nil {
		t.Fatalf("ShowBead failed: %v", err)
	}
	if !contains(refreshed.Dependents, second.ID) {
		t.Fatalf("expected first bead to list second as dependent, got %v", refreshed.Dependents)
	}

	queried, err := fake.QueryBeads(ctx, []string{"alpha"}, tracker.StatusOpen, "")
	if err != nil {
		t.Fatalf("QueryBeads failed: %v", err)
	}
	if len(queried) != 1 || queried[0].ID != first.ID {
		t.Fatalf("QueryBeads result mismatch: %+v", queried)
	}

	if pending, err := fake.NextBead(ctx); err != nil {
		t.Fatalf("NextBead failed: %v", err)
	} else if pending == nil || pending.ID != first.ID {
		t.Fatalf("NextBead returned %v, want %s", pending, first.ID)
	}

	if err := fake.CloseBead(ctx, first.ID); err != nil {
		t.Fatalf("CloseBead failed: %v", err)
	}

	unlocked, err := fake.NextBead(ctx)
	if err != nil {
		t.Fatalf("NextBead failed: %v", err)
	}
	if unlocked == nil || unlocked.ID != second.ID {
		t.Fatalf("NextBead returned %v, want %s", unlocked, second.ID)
	}

	shownSecond, err := fake.ShowBead(ctx, second.ID)
	if err != nil {
		t.Fatalf("ShowBead failed: %v", err)
	}
	if !contains(shownSecond.DependsOn, first.ID) || !contains(shownSecond.BlockedBy, first.ID) {
		t.Fatalf("second bead dependency info missing: %+v", shownSecond)
	}
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}
