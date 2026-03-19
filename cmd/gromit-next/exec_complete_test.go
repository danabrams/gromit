package main

import (
	"bytes"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestExecComplete_MarksRunAsCompleted(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run in ready_for_review state
	rs := runstore.NewRunState("my-spec", "proj")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Run the complete command
	cmd := newExecCompleteCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{rs.RunID, "--store-dir", storeDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify status changed
	updated, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.Status != runstore.StatusCompleted {
		t.Errorf("status = %q, want %q", updated.Status, runstore.StatusCompleted)
	}
	if updated.EndedAt.IsZero() {
		t.Error("EndedAt should be set")
	}

	// Verify output
	want := "Run " + rs.RunID + " marked as completed\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestExecComplete_NotFoundReturnsError(t *testing.T) {
	storeDir := t.TempDir()

	cmd := newExecCompleteCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"run-nonexistent", "--store-dir", storeDir})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestExecComplete_PreservesExistingEndedAt(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("my-spec", "proj")
	rs.Status = runstore.StatusNeedsHuman
	rs.EndedAt = rs.StartedAt // Set a non-zero EndedAt
	if err := store.Save(rs); err != nil {
		t.Fatalf("save: %v", err)
	}

	cmd := newExecCompleteCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{rs.RunID, "--store-dir", storeDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	updated, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// EndedAt should remain the original value, not be overwritten
	if !updated.EndedAt.Equal(rs.StartedAt) {
		t.Errorf("EndedAt changed: got %v, want %v", updated.EndedAt, rs.StartedAt)
	}
}
