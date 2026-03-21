package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ReviewShow_RefusesNonTerminalRun(t *testing.T) {
	// Seed: create a run that is still in "running" state
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("some-spec", "some-project")
	rs.RunID = "run-008"
	rs.Status = runstore.StatusRunning
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Invoke: load the run and check terminal-state guard
	// This mirrors what `review show --run run-008` must do before rendering.
	loaded, err := store.Get("run-008")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}

	var reviewErr error
	if !loaded.IsTerminal() {
		reviewErr = fmt.Errorf("run %s is in non-terminal state %q; review is only available after the run completes", loaded.RunID, loaded.Status)
	}

	// Assert: must produce an error about non-terminal state
	if reviewErr == nil {
		t.Fatal("expected error for non-terminal run, got nil")
	}
	errMsg := reviewErr.Error()
	if !strings.Contains(errMsg, "non-terminal") {
		t.Errorf("expected error to mention 'non-terminal', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "running") {
		t.Errorf("expected error to mention 'running' status, got: %s", errMsg)
	}
}
