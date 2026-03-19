package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// TestScenario_RunStartPrintsRunIDAndEventsPath verifies that r.run(ctx) writes
// a start banner to e.out containing the Run ID and events.jsonl path before
// any stages execute. The run ID is extracted from the terminal summary line
// (printed after stages complete) and used to verify the banner format.
//
// Scenario: run start prints run ID and events path
// Given: an execSpecRun with a stub stageProvider that returns no stages
// When: r.run(ctx) is called directly
// Then: run() returns nil, and e.out begins with "Run ID: <runID>\nEvents: <storeDir>/runs/<runID>/events.jsonl\n\n"
func TestScenario_RunStartPrintsRunIDAndEventsPath(t *testing.T) {
	// --- Seed ---
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	provider := &testStageProvider{stages: []specloop.Stage{}} // no stages

	var buf bytes.Buffer
	r := &execSpecRun{
		specPath:      "testdata/my-spec.md",
		projectID:     "gromit",
		storeDir:      storeDir,
		store:         store,
		stageProvider: provider,
		out:           &buf,
	}

	// --- Invoke ---
	err := r.run(context.Background())

	// --- Assert ---
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()

	// Extract runID from the terminal summary line ("Run ID:  " with two spaces).
	parts := strings.SplitN(output, "Run ID:  ", 2)
	if len(parts) < 2 {
		t.Fatalf("expected terminal summary with 'Run ID:  ' (two spaces), got:\n%s", output)
	}
	runID := strings.Fields(parts[1])[0]
	if runID == "" {
		t.Fatal("extracted empty runID from terminal summary")
	}

	// The banner (written before stages) must appear at the start of the buffer.
	wantBanner := "Run ID: " + runID + "\n" +
		"Events: " + storeDir + "/runs/" + runID + "/events.jsonl\n\n"

	if !strings.HasPrefix(output, wantBanner) {
		t.Errorf("expected buffer to begin with banner:\n%s\ngot:\n%s", wantBanner, output)
	}
}