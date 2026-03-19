package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestScenario_ResumePicker_BlockedAndRunning verifies that pickRun includes
// both blocked and running runs, sorts by StartedAt descending, and returns
// the correct RunID for the selected entry.
//
// Scenario: resume picker includes blocked and running runs (spec 0003f)
// Given: Two runs — blocked (11:00) and running (10:30)
// When: pickRun is called with input "2\n"
// Then: Entry 1 is run D (blocked, 11:00), entry 2 is run E (running, 10:30),
//
//	selection "2" returns run E's RunID.
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

	// Run D (blocked) should be entry 1 (most recent StartedAt)
	if !strings.Contains(output, "spec-f") {
		t.Errorf("expected spec-f in output, got:\n%s", output)
	}
	if !strings.Contains(output, "blocked") {
		t.Errorf("expected blocked label in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2026-03-18 11:00:00") {
		t.Errorf("expected timestamp 2026-03-18 11:00:00 in output, got:\n%s", output)
	}

	// Run E (running) should be entry 2
	if !strings.Contains(output, "spec-g") {
		t.Errorf("expected spec-g in output, got:\n%s", output)
	}
	if !strings.Contains(output, "running") {
		t.Errorf("expected running label in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2026-03-18 10:30:00") {
		t.Errorf("expected timestamp 2026-03-18 10:30:00 in output, got:\n%s", output)
	}

	// Exactly two numbered entries
	if strings.Contains(output, "3.") {
		t.Errorf("expected exactly two entries, but found a third, got:\n%s", output)
	}

	// Selecting "2" should return run E's RunID
	if runID != runE.RunID {
		t.Errorf("expected runID %s, got %s", runE.RunID, runID)
	}
}
