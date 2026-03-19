package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestScenario_ResumePicker_BlockedAndRunning verifies that pickRun includes
// both blocked and running runs in the picker menu, displays correct labels
// and timestamps, and returns the selected run's RunID.
//
// Scenario: resume picker includes blocked and running runs (spec 0003f)
// Given: Two runs — blocked (spec-f) and running (spec-g)
// When: pickRun is called with input "2\n"
// Then: Both entries shown; entry 1 = run D (blocked, 11:00); entry 2 = run E (running, 10:30);
//
//	selecting "2" returns run E's RunID
func TestScenario_ResumePicker_BlockedAndRunning(t *testing.T) {
	// --- Seed ---
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	runD := &runstore.RunState{
		RunID:     "run-ddd4444444444444",
		SpecID:    "spec-f",
		ProjectID: "gromit",
		Status:    runstore.StatusBlocked,
		StartedAt: time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	runE := &runstore.RunState{
		RunID:     "run-eee5555555555555",
		SpecID:    "spec-g",
		ProjectID: "gromit",
		Status:    runstore.StatusRunning,
		StartedAt: time.Date(2026, 3, 18, 10, 30, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}

	for _, rs := range []*runstore.RunState{runD, runE} {
		if err := store.Save(rs); err != nil {
			t.Fatalf("save %s: %v", rs.RunID, err)
		}
	}

	// --- Invoke ---
	in := strings.NewReader("2\n")
	var out bytes.Buffer

	runID, err := pickRun("gromit", store, in, &out)
	if err != nil {
		t.Fatalf("pickRun: %v", err)
	}

	// --- Assert ---
	output := out.String()

	// Entry 1: Run D (blocked, most recent StartedAt)
	if !strings.Contains(output, "spec-f") {
		t.Errorf("expected spec-f in output, got:\n%s", output)
	}
	if !strings.Contains(output, "blocked") {
		t.Errorf("expected 'blocked' label in output for blocked run, got:\n%s", output)
	}
	if !strings.Contains(output, "2026-03-18 11:00:00") {
		t.Errorf("expected timestamp 2026-03-18 11:00:00 in output, got:\n%s", output)
	}

	// Entry 2: Run E (running)
	if !strings.Contains(output, "spec-g") {
		t.Errorf("expected spec-g in output, got:\n%s", output)
	}
	if !strings.Contains(output, "running") {
		t.Errorf("expected running label in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2026-03-18 10:30:00") {
		t.Errorf("expected timestamp 2026-03-18 10:30:00 in output, got:\n%s", output)
	}

	// Exactly two numbered entries — no third entry.
	if strings.Contains(output, "3.") {
		t.Errorf("expected exactly two entries, but found a third, got:\n%s", output)
	}

	// Selecting "2" returns run E's RunID.
	if runID != runE.RunID {
		t.Errorf("expected runID %s, got %s", runE.RunID, runID)
	}
}
