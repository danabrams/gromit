package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// pickRun lists runs for a project, filters to resumable statuses,
// prompts the user to select one, and returns the RunID.
//
// This stub satisfies the compiler so the scenario test is RED on behavior,
// not on missing symbol. Remove it once the real pickRun is implemented.
func pickRun(project string, store *runstore.Store, in io.Reader, out io.Writer) (string, error) {
	runs, err := store.List(project)
	if err != nil {
		return "", err
	}

	resumable := []runstore.RunState{}
	for _, r := range runs {
		switch r.Status {
		case runstore.StatusRunning, runstore.StatusNeedsHuman, runstore.StatusBlocked, runstore.StatusReadyForReview:
			resumable = append(resumable, *r)
		}
	}

	if len(resumable) == 0 {
		fmt.Fprintf(out, "no runs available to resume\n")
		return "", fmt.Errorf("no runs available to resume")
	}

	// Sort by StartedAt descending (most recent first)
	for i := 0; i < len(resumable); i++ {
		for j := i + 1; j < len(resumable); j++ {
			if resumable[j].StartedAt.After(resumable[i].StartedAt) {
				resumable[i], resumable[j] = resumable[j], resumable[i]
			}
		}
	}

	for i, run := range resumable {
		var label string
		switch run.Status {
		case runstore.StatusRunning:
			label = "running"
		case runstore.StatusReadyForReview:
			label = "ready_for_review"
		case runstore.StatusNeedsHuman, runstore.StatusBlocked:
			label = "needs_attention"
		default:
			label = run.Status
		}
		timestamp := run.StartedAt.Format("2006-01-02 15:04:05")
		fmt.Fprintf(out, "%d. %s [%s] %s\n", i+1, run.SpecID, label, timestamp)
	}

	scanner := bufio.NewScanner(in)
	fmt.Fprintf(out, "\nEnter run number: ")
	if !scanner.Scan() {
		return "", fmt.Errorf("failed to read input")
	}

	selection := strings.TrimSpace(scanner.Text())
	num, err := strconv.Atoi(selection)
	if err != nil {
		return "", fmt.Errorf("invalid selection: %q", selection)
	}
	if num < 1 || num > len(resumable) {
		return "", fmt.Errorf("selection out of range: %d", num)
	}

	return resumable[num-1].RunID, nil
}

// TestScenario_ResumePicker verifies that pickRun filters out completed runs,
// sorts by StartedAt descending, displays human-readable status labels, and
// returns the selected run's RunID.
//
// Scenario: resume picker (spec 0003f)
// Given: Three runs — ready_for_review, needs_human, completed
// When: pickRun is called with input "1\n"
// Then: Only two entries shown (completed excluded), entry 1 selected → run A's RunID
func TestScenario_ResumePicker(t *testing.T) {
	// --- Seed ---
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	runA := &runstore.RunState{
		RunID:     "run-aaa1111111111111",
		SpecID:    "spec-e",
		ProjectID: "gromit",
		Status:    runstore.StatusReadyForReview,
		StartedAt: time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	runB := &runstore.RunState{
		RunID:     "run-bbb2222222222222",
		SpecID:    "spec-d",
		ProjectID: "gromit",
		Status:    runstore.StatusNeedsHuman,
		StartedAt: time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	runC := &runstore.RunState{
		RunID:     "run-ccc3333333333333",
		SpecID:    "spec-c",
		ProjectID: "gromit",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 18, 8, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}

	for _, rs := range []*runstore.RunState{runA, runB, runC} {
		if err := store.Save(rs); err != nil {
			t.Fatalf("save %s: %v", rs.RunID, err)
		}
	}

	// --- Invoke ---
	in := strings.NewReader("1\n")
	var out bytes.Buffer

	runID, err := pickRun("gromit", store, in, &out)
	if err != nil {
		t.Fatalf("pickRun: %v", err)
	}

	// --- Assert ---
	output := out.String()

	// Run A (ready_for_review) should be entry 1 (most recent StartedAt)
	if !strings.Contains(output, "spec-e") {
		t.Errorf("expected spec-e in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ready_for_review") {
		t.Errorf("expected ready_for_review label in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2026-03-18 10:00:00") {
		t.Errorf("expected timestamp 2026-03-18 10:00:00 in output, got:\n%s", output)
	}

	// Run B (needs_human) should be entry 2, displayed as "needs_attention"
	if !strings.Contains(output, "spec-d") {
		t.Errorf("expected spec-d in output, got:\n%s", output)
	}
	if !strings.Contains(output, "needs_attention") {
		t.Errorf("expected needs_attention label in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2026-03-18 09:00:00") {
		t.Errorf("expected timestamp 2026-03-18 09:00:00 in output, got:\n%s", output)
	}

	// Run C (completed) must be excluded
	if strings.Contains(output, "spec-c") {
		t.Errorf("completed run spec-c should be excluded from picker, got:\n%s", output)
	}
	if strings.Contains(output, "2026-03-18 08:00:00") {
		t.Errorf("completed run timestamp should not appear, got:\n%s", output)
	}

	// Exactly two numbered entries
	if strings.Contains(output, "3.") {
		t.Errorf("expected exactly two entries, but found a third, got:\n%s", output)
	}

	// Selecting "1" should return run A's RunID
	if runID != runA.RunID {
		t.Errorf("expected runID %s, got %s", runA.RunID, runID)
	}
}