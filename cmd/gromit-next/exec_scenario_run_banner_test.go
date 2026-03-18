package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/specloop"
)

// TestScenario_RunStartPrintsRunIDAndEventsPath verifies that executing a spec
// run prints a banner containing the Run ID and events.jsonl path before any
// stage output. The banner uses single-space formatting ("Run ID: <id>") and
// appears before the terminal summary ("Run ID:  <id>" with two spaces).
//
// RED: run() does not yet write a banner to output before stages execute.
// The output contains only the terminal summary (Run ID + Status with double
// spaces). GREEN after: run() writes a "Run ID: ...\nEvents: ...\n\n" banner
// to the output writer before invoking the stage pipeline.
func TestScenario_RunStartPrintsRunIDAndEventsPath(t *testing.T) {
	// Seed: temp store dir, stub provider with no stages.
	storeDir := t.TempDir()
	provider := &testStageProvider{stages: []specloop.Stage{}}

	// Invoke: execute via cobra to capture output (since execSpecRun does not
	// yet have an `out` field for direct banner capture).
	var buf bytes.Buffer
	cmd := newExecSpecCmdWithProvider(provider)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--spec", "testdata/my-spec.md",
		"--project", "gromit",
		"--store-dir", storeDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	output := buf.String()

	// Extract runID from the summary portion (two spaces after "Run ID:").
	parts := strings.SplitN(output, "Run ID:  ", 2)
	if len(parts) < 2 {
		t.Fatalf("output missing 'Run ID:  ', got: %q", output)
	}
	runID := strings.Fields(parts[1])[0]
	if runID == "" {
		t.Fatal("extracted runID is empty")
	}

	// Assert: banner (single-space format) appears at the start of output,
	// before any stage output (trivially satisfied when provider returns no stages).
	// Banner format:
	//   Run ID: <runID>
	//   Events: <storeDir>/runs/<runID>/events.jsonl
	//
	eventsPath := filepath.Join(storeDir, "runs", runID, "events.jsonl")
	wantBanner := fmt.Sprintf("Run ID: %s\nEvents: %s\n\n", runID, eventsPath)
	if !strings.HasPrefix(output, wantBanner) {
		t.Errorf("expected output to begin with banner:\nwant:\n%s\ngot:\n%s", wantBanner, output)
	}
}