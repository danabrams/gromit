package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestExecComplete_PrependsDoneToSpecFile(t *testing.T) {
	storeDir := t.TempDir()
	specsDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a spec file
	specName := "test-spec"
	specPath := filepath.Join(specsDir, specName+".md")
	originalContent := "# Test Spec\n\nSome content"
	if err := os.WriteFile(specPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Create a run with this spec
	rs := runstore.NewRunState(specName, "proj")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Run the complete command with specs-dir override
	cmd := newExecCompleteCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{rs.RunID, "--store-dir", storeDir, "--specs-dir", specsDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify spec file was updated with DONE prefix
	content, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec file: %v", err)
	}
	contentStr := string(content)

	// Check that content starts with DONE
	if !bytes.HasPrefix(content, []byte("DONE")) {
		t.Errorf("spec file doesn't start with DONE. Content: %q", contentStr)
	}

	// Check that the date is today
	today := time.Now().Format("2006-01-02")
	if !bytes.Contains(content, []byte(today)) {
		t.Errorf("spec file doesn't contain today's date (%s). Content: %q", today, contentStr)
	}

	// Check that original content is preserved
	if !bytes.Contains(content, []byte(originalContent)) {
		t.Errorf("original content not preserved. Content: %q", contentStr)
	}
}

func TestExecComplete_SkipsIfAlreadyDone(t *testing.T) {
	storeDir := t.TempDir()
	specsDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a spec file that already has DONE
	specName := "test-spec"
	specPath := filepath.Join(specsDir, specName+".md")
	originalContent := "DONE 2026-03-01\n# Test Spec\n\nSome content"
	if err := os.WriteFile(specPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Create a run with this spec
	rs := runstore.NewRunState(specName, "proj")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Run the complete command
	cmd := newExecCompleteCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{rs.RunID, "--store-dir", storeDir, "--specs-dir", specsDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify spec file wasn't modified (idempotent)
	content, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec file: %v", err)
	}
	if string(content) != originalContent {
		t.Errorf("spec file was modified. Got %q, want %q", string(content), originalContent)
	}
}

func TestExecComplete_HandlesSpecFileNotFound(t *testing.T) {
	storeDir := t.TempDir()
	specsDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a run with a spec that doesn't exist
	specName := "nonexistent-spec"
	rs := runstore.NewRunState(specName, "proj")
	rs.Status = runstore.StatusReadyForReview
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Run the complete command - should still succeed (run is marked as completed)
	// but spec file update should be skipped gracefully
	cmd := newExecCompleteCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{rs.RunID, "--store-dir", storeDir, "--specs-dir", specsDir})

	// Should not error; spec file just doesn't exist
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify run was still marked as completed
	updated, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updated.Status != runstore.StatusCompleted {
		t.Errorf("run status = %q, want %q", updated.Status, runstore.StatusCompleted)
	}
}
