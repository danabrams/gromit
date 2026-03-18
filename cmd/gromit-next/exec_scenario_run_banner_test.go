package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/specloop"
)

// TestScenario_RunStartPrintsRunIDAndEventsPath verifies that run() writes a
// banner to e.out (the buffer) containing the run ID and events path before
// any stages execute, and returns the run ID in the summary string.
//
// RED: execSpecRun has no `out` field — banner is not yet written to a buffer.
// GREEN after: execSpecRun gains an `out io.Writer` field, and run() writes
// the banner to e.out before calling loop.Run().
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
	}

	// Invoke
	summary, err := r.run(context.Background())
	if err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	// Assert: summary (return value) contains "Run ID:  " with two spaces
	parts := strings.SplitN(summary, "Run ID:  ", 2)
	if len(parts) < 2 {
		t.Fatalf("summary missing 'Run ID:  ', got: %s", summary)
	}
	fields := strings.Fields(parts[1])
	if len(fields) == 0 {
		t.Fatalf("no run ID token after 'Run ID:  ' in summary: %s", summary)
	}
	runID := fields[0]
	if runID == "" {
		t.Fatal("extracted runID is empty")
	}

	// Assert: buffer (e.out) begins with the banner — distinct from summary
	wantBanner := fmt.Sprintf("Run ID: %s\nEvents: %s/runs/%s/events.jsonl\n",
		runID, storeDir, runID)
	banner := buf.String()
	if !strings.HasPrefix(banner, wantBanner) {
		t.Errorf("banner mismatch.\nwant prefix:\n%s\ngot:\n%s", wantBanner, banner)
	}
}