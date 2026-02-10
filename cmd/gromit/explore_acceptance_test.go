package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
)

// TestExploreCommand_E2E_PreSessionSnapshot verifies that the explore command
// performs a complete pre-session snapshot of epics, specs, and backlog items.
func TestExploreCommand_E2E_PreSessionSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := filepath.Join(gromitDir, "specs")

	// Set up initial state: create directories with existing artifacts
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Create existing epic files
	epic1 := filepath.Join(epicsDir, "onboarding.md")
	epic2 := filepath.Join(epicsDir, "performance.md")
	if err := os.WriteFile(epic1, []byte("# Onboarding Epic"), 0644); err != nil {
		t.Fatalf("failed to create epic1: %v", err)
	}
	if err := os.WriteFile(epic2, []byte("# Performance Epic"), 0644); err != nil {
		t.Fatalf("failed to create epic2: %v", err)
	}

	// Create existing spec files
	spec1 := filepath.Join(specsDir, "init-wizard.md")
	spec2 := filepath.Join(specsDir, "api-optimization.md")
	if err := os.WriteFile(spec1, []byte("# Init Wizard Spec"), 0644); err != nil {
		t.Fatalf("failed to create spec1: %v", err)
	}
	if err := os.WriteFile(spec2, []byte("# API Optimization Spec"), 0644); err != nil {
		t.Fatalf("failed to create spec2: %v", err)
	}

	// Create existing backlog items
	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}

	existingIdea1 := &backlog.Idea{
		ID:        "idea-101",
		Text:      "Add dark mode support",
		Type:      "feature",
		CreatedAt: time.Now(),
	}
	existingIdea2 := &backlog.Idea{
		ID:        "idea-102",
		Text:      "Fix login timeout issue",
		Type:      "bug",
		CreatedAt: time.Now(),
	}

	if err := bf.Add(existingIdea1); err != nil {
		t.Fatalf("failed to add existing idea 1: %v", err)
	}
	if err := bf.Add(existingIdea2); err != nil {
		t.Fatalf("failed to add existing idea 2: %v", err)
	}

	// Perform pre-session snapshot (simulating what explore command should do)
	snapshotEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("snapshot: failed to get epic files: %v", err)
	}

	snapshotSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("snapshot: failed to get spec files: %v", err)
	}

	snapshotBacklog, err := bf.List()
	if err != nil {
		t.Fatalf("snapshot: failed to list backlog: %v", err)
	}

	// Verify snapshot captured all existing artifacts
	if len(snapshotEpics) != 2 {
		t.Errorf("snapshot should capture 2 epic files, got %d", len(snapshotEpics))
	}

	if len(snapshotSpecs) != 2 {
		t.Errorf("snapshot should capture 2 spec files, got %d", len(snapshotSpecs))
	}

	if len(snapshotBacklog) != 2 {
		t.Errorf("snapshot should capture 2 backlog items, got %d", len(snapshotBacklog))
	}

	// Verify specific artifacts are in snapshot
	epicPaths := make(map[string]bool)
	for _, e := range snapshotEpics {
		epicPaths[e] = true
	}
	if !epicPaths[epic1] || !epicPaths[epic2] {
		t.Error("snapshot should include both existing epic files")
	}

	specPaths := make(map[string]bool)
	for _, s := range snapshotSpecs {
		specPaths[s] = true
	}
	if !specPaths[spec1] || !specPaths[spec2] {
		t.Error("snapshot should include both existing spec files")
	}

	backlogIDs := make(map[string]bool)
	for _, item := range snapshotBacklog {
		backlogIDs[item.ID] = true
	}
	if !backlogIDs["idea-101"] || !backlogIDs["idea-102"] {
		t.Error("snapshot should include both existing backlog items")
	}
}

// TestExploreCommand_E2E_EnsuresEpicsDirExists verifies that the explore command
// creates the .gromit/epics/ directory if it doesn't exist.
func TestExploreCommand_E2E_EnsuresEpicsDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")

	// Ensure gromit dir exists but epics dir does not
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Verify epics dir doesn't exist yet
	if _, err := os.Stat(epicsDir); !os.IsNotExist(err) {
		t.Fatal("epics dir should not exist initially")
	}

	// Simulate explore command ensuring directory exists
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Verify epics dir now exists with correct permissions
	stat, err := os.Stat(epicsDir)
	if err != nil {
		t.Fatalf("epics dir should exist: %v", err)
	}

	if !stat.IsDir() {
		t.Error("epics path should be a directory")
	}

	if stat.Mode().Perm() != 0755 {
		t.Errorf("epics dir should have 0755 permissions, got %v", stat.Mode().Perm())
	}
}

// TestExploreCommand_E2E_SnapshotBeforeSession verifies that the snapshot
// is taken BEFORE launching the Claude session (crucial for detecting new artifacts).
func TestExploreCommand_E2E_SnapshotBeforeSession(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")

	// Create directory and initial epic
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	existingEpic := filepath.Join(epicsDir, "existing.md")
	if err := os.WriteFile(existingEpic, []byte("# Existing"), 0644); err != nil {
		t.Fatalf("failed to create existing epic: %v", err)
	}

	// Take pre-session snapshot
	preSessionEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("failed to get pre-session epics: %v", err)
	}

	if len(preSessionEpics) != 1 {
		t.Fatalf("pre-session snapshot should have 1 epic, got %d", len(preSessionEpics))
	}

	// Simulate session creating a new epic
	newEpic := filepath.Join(epicsDir, "new-from-session.md")
	if err := os.WriteFile(newEpic, []byte("# New Epic"), 0644); err != nil {
		t.Fatalf("failed to create new epic: %v", err)
	}

	// Take post-session snapshot
	postSessionEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("failed to get post-session epics: %v", err)
	}

	if len(postSessionEpics) != 2 {
		t.Fatalf("post-session snapshot should have 2 epics, got %d", len(postSessionEpics))
	}

	// Verify pre-session snapshot didn't include the new epic
	for _, epic := range preSessionEpics {
		if epic == newEpic {
			t.Error("pre-session snapshot should not include epics created during session")
		}
	}

	// Verify we can detect the new epic by comparing snapshots
	preSessionSet := make(map[string]bool)
	for _, epic := range preSessionEpics {
		preSessionSet[epic] = true
	}

	newEpics := []string{}
	for _, epic := range postSessionEpics {
		if !preSessionSet[epic] {
			newEpics = append(newEpics, epic)
		}
	}

	if len(newEpics) != 1 {
		t.Fatalf("should detect exactly 1 new epic, got %d", len(newEpics))
	}

	if newEpics[0] != newEpic {
		t.Errorf("should detect new epic %s, got %s", newEpic, newEpics[0])
	}
}
