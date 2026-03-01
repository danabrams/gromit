package integrationqueue

import (
	"fmt"
	"testing"
	"time"
)

func TestProjectStatusSummariesAndOrdering(t *testing.T) {
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	snapshot := &Snapshot{
		Entries: []Entry{
			{
				Branch:    "gromit/integrating-branch",
				State:     StateIntegrating,
				Lane:      "code_lane",
				FifoSeq:   1,
				UpdatedAt: base.Add(5 * time.Minute),
			},
			{
				Branch:    "gromit/ready-second",
				State:     StateReady,
				Lane:      "code_lane",
				FifoSeq:   3,
				UpdatedAt: base.Add(3 * time.Minute),
			},
			{
				Branch:    "gromit/ready-first",
				State:     StateReady,
				Lane:      "safe_lane",
				FifoSeq:   2,
				UpdatedAt: base.Add(2 * time.Minute),
			},
			{
				Branch:    "gromit/blocked-newer",
				State:     StateConflict,
				Lane:      "safe_lane",
				FifoSeq:   4,
				UpdatedAt: base.Add(4 * time.Minute),
			},
			{
				Branch:    "gromit/blocked-older",
				State:     StateFailedGates,
				Lane:      "code_lane",
				FifoSeq:   5,
				UpdatedAt: base,
			},
			{
				Branch:    "gromit/merged-branch",
				State:     StateMerged,
				Lane:      "code_lane",
				FifoSeq:   6,
				UpdatedAt: base.Add(6 * time.Minute),
			},
		},
	}

	status := ProjectStatus(snapshot)
	if status == nil {
		t.Fatalf("ProjectStatus returned nil")
	}

	if status.QueueLength != len(snapshot.Entries) {
		t.Fatalf("QueueLength = %d, want %d", status.QueueLength, len(snapshot.Entries))
	}
	if status.ReadyCount != 2 {
		t.Fatalf("ReadyCount = %d, want 2", status.ReadyCount)
	}
	if status.IntegratingCount != 1 {
		t.Fatalf("IntegratingCount = %d, want 1", status.IntegratingCount)
	}
	if status.BlockedCount != 2 {
		t.Fatalf("BlockedCount = %d, want 2", status.BlockedCount)
	}
	if status.MergedCount != 1 {
		t.Fatalf("MergedCount = %d, want 1", status.MergedCount)
	}

	if len(status.Entries) != 5 {
		t.Fatalf("entries len = %d, want 5", len(status.Entries))
	}

	order := []string{
		"gromit/integrating-branch",
		"gromit/ready-first",
		"gromit/ready-second",
		"gromit/blocked-newer",
		"gromit/blocked-older",
	}
	for i, entry := range status.Entries {
		if entry.Entry.Branch != order[i] {
			t.Fatalf("entry %d branch = %q, want %q", i, entry.Entry.Branch, order[i])
		}
	}

	readyPositions := []int{0, 1, 2, 0, 0}
	for i, entry := range status.Entries {
		if entry.ReadyPosition != readyPositions[i] {
			t.Fatalf("entry %d ready position = %d, want %d", i, entry.ReadyPosition, readyPositions[i])
		}
	}
}

func TestProjectStatusLimitsDisplayedEntries(t *testing.T) {
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	totalReady := displayLimit + 3
	entries := make([]Entry, 0, totalReady+1)
	for i := 0; i < totalReady; i++ {
		entries = append(entries, Entry{
			Branch:    fmt.Sprintf("gromit/ready-%d", i),
			State:     StateReady,
			Lane:      "code_lane",
			FifoSeq:   i + 1,
			UpdatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	entries = append(entries, Entry{
		Branch:    "gromit/merged-branch",
		State:     StateMerged,
		Lane:      "safe_lane",
		FifoSeq:   totalReady + 1,
		UpdatedAt: base.Add(time.Duration(totalReady) * time.Minute),
	})

	snapshot := &Snapshot{Entries: entries}
	status := ProjectStatus(snapshot)
	if status == nil {
		t.Fatalf("ProjectStatus returned nil")
	}

	if len(status.Entries) != displayLimit {
		t.Fatalf("entries len = %d, want %d", len(status.Entries), displayLimit)
	}

	for i := 0; i < displayLimit; i++ {
		entry := status.Entries[i]
		wantBranch := fmt.Sprintf("gromit/ready-%d", i)
		if entry.Entry.Branch != wantBranch {
			t.Fatalf("entry %d branch = %q, want %q", i, entry.Entry.Branch, wantBranch)
		}
		if entry.ReadyPosition != i+1 {
			t.Fatalf("entry %d ready position = %d, want %d", i, entry.ReadyPosition, i+1)
		}
	}

	for _, entry := range status.Entries {
		if entry.Entry.State == StateMerged {
			t.Fatalf("merged entry %q should not be displayed", entry.Entry.Branch)
		}
	}

	if status.MergedCount != 1 {
		t.Fatalf("MergedCount = %d, want 1", status.MergedCount)
	}
}

func TestProjectStatusCountsPushFailureAsBlocked(t *testing.T) {
	snapshot := &Snapshot{
		Entries: []Entry{
			{
				Branch: "gromit/ready-branch",
				State:  StateReady,
				Lane:   "code_lane",
			},
			{
				Branch: "gromit/push-failure-branch",
				State:  StatePushFailure,
				Lane:   "code_lane",
			},
		},
	}

	status := ProjectStatus(snapshot)
	if status.BlockedCount != 1 {
		t.Fatalf("BlockedCount = %d, want 1", status.BlockedCount)
	}

	found := false
	for _, entry := range status.Entries {
		if entry.Entry.Branch == "gromit/push-failure-branch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("push failure entry not included in status entries")
	}
}
