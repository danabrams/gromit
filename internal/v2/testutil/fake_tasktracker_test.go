package testutil

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
)

var _ = tracker.StatusOpen

func TestFakeTaskTracker_CreatesBeadsAndObservesDependencies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fake := NewFakeTaskTracker()

	firstResp, err := fake.CreateBead(ctx, tasktracker.CreateBeadRequest{
		Title:       "first",
		Description: "desc",
		Priority:    1,
		Labels:      []string{"alpha"},
	})
	if err != nil {
		t.Fatalf("CreateBead failed: %v", err)
	}
	first := firstResp.Bead
	if first == nil {
		t.Fatal("CreateBead returned nil bead")
	}

	secondResp, err := fake.CreateBead(ctx, tasktracker.CreateBeadRequest{
		Title:        "second",
		Description:  "desc",
		Priority:     2,
		Labels:       []string{"beta"},
		Dependencies: []string{first.ID},
	})
	if err != nil {
		t.Fatalf("CreateBead failed: %v", err)
	}
	second := secondResp.Bead

	refreshed, err := fake.ShowBead(ctx, first.ID)
	if err != nil {
		t.Fatalf("ShowBead failed: %v", err)
	}
	if !contains(refreshed.Dependents, second.ID) {
		t.Fatalf("expected first bead to list second as dependent, got %v", refreshed.Dependents)
	}

	queriedResp, err := fake.QueryBeads(ctx, tasktracker.QueryBeadsRequest{Labels: []string{"alpha"}, Status: tracker.StatusOpen})
	if err != nil {
		t.Fatalf("QueryBeads failed: %v", err)
	}
	if queriedResp == nil || len(queriedResp.Beads) != 1 || queriedResp.Beads[0].ID != first.ID {
		t.Fatalf("QueryBeads result mismatch: %+v", queriedResp)
	}

	pendingResp, err := fake.NextBead(ctx, tasktracker.NextBeadRequest{})
	if err != nil {
		t.Fatalf("NextBead failed: %v", err)
	}
	if pendingResp == nil || pendingResp.Bead == nil || pendingResp.Bead.ID != first.ID {
		t.Fatalf("NextBead returned %v, want %s", pendingResp, first.ID)
	}

	closeResp, err := fake.CloseBead(ctx, tasktracker.CloseBeadRequest{BeadID: first.ID})
	if err != nil {
		t.Fatalf("CloseBead failed: %v", err)
	}
	if closeResp == nil || !closeResp.Closed {
		t.Fatalf("CloseBead response = %#v", closeResp)
	}

	unlockedResp, err := fake.NextBead(ctx, tasktracker.NextBeadRequest{})
	if err != nil {
		t.Fatalf("NextBead failed: %v", err)
	}
	if unlockedResp == nil || unlockedResp.Bead == nil || unlockedResp.Bead.ID != second.ID {
		t.Fatalf("NextBead returned %v, want %s", unlockedResp, second.ID)
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
