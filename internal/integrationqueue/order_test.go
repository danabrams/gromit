package integrationqueue

import (
	"testing"
	"time"
)

func TestOrderEntriesForDisplay(t *testing.T) {
	base := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	entries := []*Entry{
		{
			Branch:    "gromit/ready-second",
			State:     StateReady,
			FifoSeq:   4,
			UpdatedAt: base.Add(1 * time.Minute),
		},
		{
			Branch:    "gromit/integrating-first",
			State:     StateIntegrating,
			FifoSeq:   2,
			UpdatedAt: base.Add(2 * time.Minute),
		},
		{
			Branch:    "gromit/blocked-latest-low",
			State:     StateConflict,
			FifoSeq:   3,
			UpdatedAt: base.Add(3 * time.Minute),
		},
		{
			Branch:    "gromit/blocked-latest-high",
			State:     StateFailedGates,
			FifoSeq:   5,
			UpdatedAt: base.Add(3 * time.Minute),
		},
		{
			Branch:    "gromit/ready-first",
			State:     StateReady,
			FifoSeq:   1,
			UpdatedAt: base.Add(4 * time.Minute),
		},
		{
			Branch:    "gromit/integrating-second",
			State:     StateIntegrating,
			FifoSeq:   6,
			UpdatedAt: base.Add(5 * time.Minute),
		},
		{
			Branch:    "gromit/blocked-older",
			State:     StateLaneViolation,
			FifoSeq:   7,
			UpdatedAt: base.Add(2 * time.Minute),
		},
	}

	ordered := orderEntriesForDisplay(entries)
	want := []string{
		"gromit/integrating-first",
		"gromit/integrating-second",
		"gromit/ready-first",
		"gromit/ready-second",
		"gromit/blocked-latest-high",
		"gromit/blocked-latest-low",
		"gromit/blocked-older",
	}

	if len(ordered) != len(want) {
		t.Fatalf("ordered length = %d, want %d", len(ordered), len(want))
	}
	for i, entry := range ordered {
		if entry.Branch != want[i] {
			t.Fatalf("entry %d branch = %q, want %q", i, entry.Branch, want[i])
		}
	}
}
