package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// TestScenario_RunStartPrintsRunIDAndEventsPath verifies that run() writes a
// start banner to e.out containing the run ID and events path before any
// stages execute, and also writes the terminal summary to e.out at completion.
func TestScenario_RunStartPrintsRunIDAndEventsPath(t *testing.T) {
	// Seed
	storeDir := t.TempDir()
	provider := &testStageProvider{stages: []specloop.Stage{}} // no stages
	var buf bytes.Buffer

	r := &execSpecRun{
		specPath:      "testdata/my-spec.md",
		projectID:     "gromit",
		storeDir:      storeDir,
		stageProvider: provider,
		out:           &buf,
		store:         runstore.NewStore(storeDir),
	}

	// Invoke
	if err := r.run(context.Background()); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	output := buf.String()

	// Extract run ID from start banner: "Run ID: <id>\n" (single space)
	parts := strings.SplitN(output, "Run ID: ", 2)
	if len(parts) < 2 {
		t.Fatalf("output missing 'Run ID: ' banner, got: %s", output)
	}
	runID := strings.Fields(parts[1])[0]
	if runID == "" {
		t.Fatal("extracted runID is empty")
	}

	// Assert: buffer begins with the start banner
	wantBanner := fmt.Sprintf("Run ID: %s\nEvents: %s/runs/%s/events.jsonl\n",
		runID, storeDir, runID)
	if !strings.HasPrefix(output, wantBanner) {
		t.Errorf("banner mismatch.\nwant prefix:\n%s\ngot:\n%s", wantBanner, output)
	}

	// Assert: terminal summary with double-space is also written to e.out
	wantSummary := fmt.Sprintf("Run ID:  %s\nStatus:", runID)
	if !strings.Contains(output, wantSummary) {
		t.Errorf("terminal summary missing 'Run ID:  %s' (double space), got:\n%s", runID, output)
	}
}
