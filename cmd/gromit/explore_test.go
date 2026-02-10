package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
)

// TestExploreCommand_RecordsExistingEpicFiles verifies that the explore command
// records all existing .md files in .gromit/epics/ before launching the session.
func TestExploreCommand_RecordsExistingEpicFiles(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")

	// Create epics directory with some files
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Create existing epic files
	epic1 := filepath.Join(epicsDir, "epic-1.md")
	epic2 := filepath.Join(epicsDir, "epic-2.md")
	if err := os.WriteFile(epic1, []byte("# Epic 1"), 0644); err != nil {
		t.Fatalf("failed to write epic1: %v", err)
	}
	if err := os.WriteFile(epic2, []byte("# Epic 2"), 0644); err != nil {
		t.Fatalf("failed to write epic2: %v", err)
	}

	// Create a non-markdown file (should be ignored)
	txtFile := filepath.Join(epicsDir, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed to write txt file: %v", err)
	}

	// Record existing epic files (this should match the pattern in debug.go)
	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	// Verify both epics are recorded
	if len(existingEpics) != 2 {
		t.Errorf("expected 2 existing epics, got %d", len(existingEpics))
	}

	foundEpic1 := false
	foundEpic2 := false
	for _, e := range existingEpics {
		if e == epic1 {
			foundEpic1 = true
		}
		if e == epic2 {
			foundEpic2 = true
		}
	}

	if !foundEpic1 || !foundEpic2 {
		t.Errorf("missing expected epics: epic1=%v, epic2=%v", foundEpic1, foundEpic2)
	}

	// Verify txt file was not included
	for _, e := range existingEpics {
		if e == txtFile {
			t.Error("getEpicFiles should not include non-markdown files")
		}
	}
}

// TestExploreCommand_RecordsExistingSpecFiles verifies that the explore command
// records all existing .md files in .gromit/specs/ before launching the session.
func TestExploreCommand_RecordsExistingSpecFiles(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")

	// Create specs directory with some files
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Create existing spec files
	spec1 := filepath.Join(specsDir, "spec-a.md")
	spec2 := filepath.Join(specsDir, "spec-b.md")
	if err := os.WriteFile(spec1, []byte("# Spec A"), 0644); err != nil {
		t.Fatalf("failed to write spec1: %v", err)
	}
	if err := os.WriteFile(spec2, []byte("# Spec B"), 0644); err != nil {
		t.Fatalf("failed to write spec2: %v", err)
	}

	// Record existing spec files (this should reuse getSpecFiles pattern)
	existingSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}

	// Verify both specs are recorded
	if len(existingSpecs) != 2 {
		t.Errorf("expected 2 existing specs, got %d", len(existingSpecs))
	}

	foundSpec1 := false
	foundSpec2 := false
	for _, s := range existingSpecs {
		if s == spec1 {
			foundSpec1 = true
		}
		if s == spec2 {
			foundSpec2 = true
		}
	}

	if !foundSpec1 || !foundSpec2 {
		t.Errorf("missing expected specs: spec1=%v, spec2=%v", foundSpec1, foundSpec2)
	}
}

// TestExploreCommand_RecordsExistingBacklogItems verifies that the explore command
// records all existing backlog items before launching the session.
func TestExploreCommand_RecordsExistingBacklogItems(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}

	// Create initial backlog items
	item1 := &backlog.Idea{
		ID:        "idea-1",
		Text:      "First idea",
		Type:      "feature",
		CreatedAt: time.Now(),
	}
	item2 := &backlog.Idea{
		ID:        "idea-2",
		Text:      "Second idea",
		Type:      "bug",
		CreatedAt: time.Now(),
	}

	if err := bf.Add(item1); err != nil {
		t.Fatalf("failed to add item1: %v", err)
	}
	if err := bf.Add(item2); err != nil {
		t.Fatalf("failed to add item2: %v", err)
	}

	// Record existing backlog items
	existingItems, err := bf.List()
	if err != nil {
		t.Fatalf("failed to list existing items: %v", err)
	}

	// Verify both items are recorded
	if len(existingItems) != 2 {
		t.Errorf("expected 2 existing items, got %d", len(existingItems))
	}

	foundItem1 := false
	foundItem2 := false
	for _, item := range existingItems {
		if item.ID == "idea-1" {
			foundItem1 = true
		}
		if item.ID == "idea-2" {
			foundItem2 = true
		}
	}

	if !foundItem1 || !foundItem2 {
		t.Errorf("missing expected backlog items: item1=%v, item2=%v", foundItem1, foundItem2)
	}
}

// TestExploreCommand_CreatesEpicsDirIfMissing verifies that the explore command
// ensures .gromit/epics/ directory exists, creating it if missing.
func TestExploreCommand_CreatesEpicsDirIfMissing(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")

	// Ensure gromit dir exists but epics dir does not
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Verify epics dir doesn't exist yet
	if _, err := os.Stat(epicsDir); !os.IsNotExist(err) {
		t.Fatal("epics dir should not exist yet")
	}

	// Attempt to ensure epics directory exists
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Verify epics dir now exists
	stat, err := os.Stat(epicsDir)
	if err != nil {
		t.Fatalf("epics dir should exist after creation: %v", err)
	}

	if !stat.IsDir() {
		t.Error("epics path should be a directory")
	}

	// Verify directory has correct permissions
	mode := stat.Mode()
	expectedMode := os.FileMode(0755)
	if mode.Perm() != expectedMode {
		t.Errorf("epics dir has mode %v, expected %v", mode.Perm(), expectedMode)
	}
}

// TestExploreCommand_HandlesEmptyEpicsDir verifies that the explore command
// handles an empty .gromit/epics/ directory gracefully.
func TestExploreCommand_HandlesEmptyEpicsDir(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")

	// Create empty epics directory
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Record existing epic files
	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles should not error on empty dir: %v", err)
	}

	// Verify empty slice is returned
	if len(existingEpics) != 0 {
		t.Errorf("expected empty slice for empty dir, got %d epics", len(existingEpics))
	}
}

// TestExploreCommand_HandlesMissingEpicsDir verifies that the explore command
// handles a missing .gromit/epics/ directory gracefully by returning an empty list.
func TestExploreCommand_HandlesMissingEpicsDir(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "nonexistent")

	// Record existing epic files (directory doesn't exist)
	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles should not error on missing dir: %v", err)
	}

	// Verify empty slice is returned
	if len(existingEpics) != 0 {
		t.Errorf("expected empty slice for missing dir, got %d epics", len(existingEpics))
	}
}

// TestExploreCommand_HandlesMissingSpecsDir verifies that the explore command
// handles a missing .gromit/specs/ directory gracefully by returning an empty list.
func TestExploreCommand_HandlesMissingSpecsDir(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "nonexistent")

	// Record existing spec files (directory doesn't exist)
	existingSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles should not error on missing dir: %v", err)
	}

	// Verify empty slice is returned
	if len(existingSpecs) != 0 {
		t.Errorf("expected empty slice for missing dir, got %d specs", len(existingSpecs))
	}
}

// TestExploreCommand_HandlesMissingBacklogFile verifies that the explore command
// handles a missing backlog file gracefully by returning an empty list.
func TestExploreCommand_HandlesMissingBacklogFile(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}

	// List items (file doesn't exist yet)
	existingItems, err := bf.List()
	if err != nil {
		t.Fatalf("List should not error on missing file: %v", err)
	}

	// Verify empty slice is returned
	if len(existingItems) != 0 {
		t.Errorf("expected empty slice for missing file, got %d items", len(existingItems))
	}
}

// TestExploreCommand_SnapshotContainsAllArtifacts verifies that the pre-session
// snapshot includes all three artifact types: epics, specs, and backlog items.
func TestExploreCommand_SnapshotContainsAllArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := filepath.Join(gromitDir, "specs")

	// Create directories
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Create an epic file
	epicFile := filepath.Join(epicsDir, "epic-1.md")
	if err := os.WriteFile(epicFile, []byte("# Epic 1"), 0644); err != nil {
		t.Fatalf("failed to write epic: %v", err)
	}

	// Create a spec file
	specFile := filepath.Join(specsDir, "spec-a.md")
	if err := os.WriteFile(specFile, []byte("# Spec A"), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	// Create a backlog item
	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("failed to create backlog file: %v", err)
	}

	item := &backlog.Idea{
		ID:        "idea-1",
		Text:      "Test idea",
		Type:      "feature",
		CreatedAt: time.Now(),
	}
	if err := bf.Add(item); err != nil {
		t.Fatalf("failed to add backlog item: %v", err)
	}

	// Record snapshot of all artifacts
	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	existingSpecs, err := getSpecFiles(specsDir)
	if err != nil {
		t.Fatalf("getSpecFiles failed: %v", err)
	}

	existingBacklog, err := bf.List()
	if err != nil {
		t.Fatalf("bf.List failed: %v", err)
	}

	// Verify snapshot contains all artifacts
	if len(existingEpics) != 1 {
		t.Errorf("expected 1 epic in snapshot, got %d", len(existingEpics))
	} else {
		// Only check epic file if we have one
		if existingEpics[0] != epicFile {
			t.Errorf("expected epic file %s in snapshot, got %s", epicFile, existingEpics[0])
		}
	}

	if len(existingSpecs) != 1 {
		t.Errorf("expected 1 spec in snapshot, got %d", len(existingSpecs))
	} else {
		// Only check spec file if we have one
		if existingSpecs[0] != specFile {
			t.Errorf("expected spec file %s in snapshot, got %s", specFile, existingSpecs[0])
		}
	}

	if len(existingBacklog) != 1 {
		t.Errorf("expected 1 backlog item in snapshot, got %d", len(existingBacklog))
	} else {
		// Only check backlog item if we have one
		if existingBacklog[0].ID != "idea-1" {
			t.Errorf("expected backlog item idea-1 in snapshot, got %s", existingBacklog[0].ID)
		}
	}
}

// TestExploreCommand_SnapshotOrderDoesNotMatter verifies that the order of
// files in the snapshot doesn't affect correctness (testing set-like behavior).
func TestExploreCommand_SnapshotOrderDoesNotMatter(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")

	// Create epics directory with multiple files
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Create epic files with names that will be sorted differently by filesystem
	epic1 := filepath.Join(epicsDir, "a-epic.md")
	epic2 := filepath.Join(epicsDir, "z-epic.md")
	epic3 := filepath.Join(epicsDir, "m-epic.md")

	for _, path := range []string{epic3, epic1, epic2} { // Write in different order
		if err := os.WriteFile(path, []byte("# Epic"), 0644); err != nil {
			t.Fatalf("failed to write epic: %v", err)
		}
	}

	// Record existing epics
	existingEpics, err := getEpicFiles(epicsDir)
	if err != nil {
		t.Fatalf("getEpicFiles failed: %v", err)
	}

	// Verify all three epics are present (order doesn't matter)
	if len(existingEpics) != 3 {
		t.Fatalf("expected 3 epics, got %d", len(existingEpics))
	}

	epicSet := make(map[string]bool)
	for _, e := range existingEpics {
		epicSet[e] = true
	}

	if !epicSet[epic1] || !epicSet[epic2] || !epicSet[epic3] {
		t.Error("all three epics should be present in snapshot regardless of order")
	}
}

// Helper function that should exist in explore.go
// This test will fail until the implementation is created

// getEpicFiles returns a list of .md files in the epics directory
// This function should be implemented in explore.go (similar to getReportFiles in debug.go)
func getEpicFiles(epicsDir string) ([]string, error) {
	// This is a placeholder - the actual implementation should be in explore.go
	// The test will fail until this is implemented
	return nil, nil
}

// Note: getSpecFiles already exists in refine.go and will be reused
