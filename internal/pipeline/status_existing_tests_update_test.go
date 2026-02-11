//go:build acceptance

package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
)

// TestExistingReadStatusTests_UpdatedToPassNil verifies existing tests updated for new signature
// This test duplicates the structure of existing tests in status_test.go but with the new signature
func TestExistingReadStatusTests_UpdatedToPassNil(t *testing.T) {
	// Expected failure: All calls to ReadStatus in status_test.go need to be updated to pass nil as fourth parameter
	// Current: ReadStatus(gromitDir, specsDir, plansDir)
	// Updated: ReadStatus(gromitDir, specsDir, plansDir, nil)

	t.Run("empty directories with nil startedAt", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0755)
		os.MkdirAll(plansDir, 0755)

		// This call should use the new signature with nil
		status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err != nil {
			t.Fatalf("ReadStatus() error = %v", err)
		}

		if status.UnrefinedCount != 0 {
			t.Errorf("UnrefinedCount = %d, want 0", status.UnrefinedCount)
		}
	})

	t.Run("unrefined ideas with nil startedAt", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0755)
		os.MkdirAll(plansDir, 0755)

		bf, _ := backlog.NewFile(gromitDir)
		bf.Add(&backlog.Idea{
			ID:     "idea-1",
			Text:   "Add user authentication",
			Status: "",
		})

		// This call should use the new signature with nil
		status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err != nil {
			t.Fatalf("ReadStatus() error = %v", err)
		}

		if status.UnrefinedCount != 1 {
			t.Errorf("UnrefinedCount = %d, want 1", status.UnrefinedCount)
		}
	})

	t.Run("missing directories with nil startedAt", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		// Don't create directories
		// This call should use the new signature with nil
		status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err != nil {
			t.Fatalf("ReadStatus() with missing directories should not error, got: %v", err)
		}

		if status.UnrefinedCount != 0 {
			t.Errorf("UnrefinedCount = %d, want 0", status.UnrefinedCount)
		}
	})
}

// TestReadStatus_ExistingBehaviorPreservedWithNilStartedAt verifies backward compatibility
func TestReadStatus_ExistingBehaviorPreservedWithNilStartedAt(t *testing.T) {
	// Expected failure: ReadStatus signature change requires all existing tests to pass nil
	// This verifies that passing nil preserves all original behavior

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Add test data
	bf, _ := backlog.NewFile(gromitDir)
	bf.Add(&backlog.Idea{
		ID:     "idea-1",
		Text:   "Test idea",
		Status: "",
	})

	os.WriteFile(filepath.Join(specsDir, "test-spec.md"), []byte("# Test Spec"), 0644)
	os.WriteFile(filepath.Join(plansDir, "test-plan.md"), []byte("---\ndecomposed: false\n---\n# Plan"), 0644)

	// Call with nil startedAt
	status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// Verify all original fields still work
	if status.UnrefinedCount != 1 {
		t.Errorf("UnrefinedCount = %d, want 1", status.UnrefinedCount)
	}

	if len(status.UnrefinedIdeas) != 1 {
		t.Errorf("len(UnrefinedIdeas) = %d, want 1", len(status.UnrefinedIdeas))
	}

	if len(status.UnplannedSpecs) != 1 {
		t.Errorf("len(UnplannedSpecs) = %d, want 1", len(status.UnplannedSpecs))
	}

	if len(status.UndecomposedPlans) != 1 {
		t.Errorf("len(UndecomposedPlans) = %d, want 1", len(status.UndecomposedPlans))
	}

	// Verify recommendation still generated
	if status.Recommendation == "" {
		t.Error("Recommendation should not be empty")
	}

	// Verify new fields exist and have correct defaults when startedAt is nil
	if status.HasRunInfo {
		t.Error("HasRunInfo should be false when startedAt is nil")
	}

	if status.ClosedThisRunCount != 0 {
		t.Errorf("ClosedThisRunCount should be 0 when startedAt is nil, got %d", status.ClosedThisRunCount)
	}
}

// TestReadStatus_ErrorHandlingWithNilStartedAt verifies error cases still work
func TestReadStatus_ErrorHandlingWithNilStartedAt(t *testing.T) {
	// Expected failure: ReadStatus signature needs to be updated in error handling tests too

	t.Run("corrupt backlog file", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0755)
		os.MkdirAll(plansDir, 0755)

		// Write corrupt JSON to backlog
		backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
		os.WriteFile(backlogPath, []byte("{invalid json\n"), 0644)

		// Call with nil startedAt - should still return error
		_, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err == nil {
			t.Fatal("expected error with corrupt backlog file, got nil")
		}
	})

	t.Run("corrupt plan frontmatter", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0755)
		os.MkdirAll(plansDir, 0755)

		// Create plan with malformed frontmatter
		planPath := filepath.Join(plansDir, "bad-plan.md")
		os.WriteFile(planPath, []byte("---\nthis is not valid yaml: {[\n---\n# Plan"), 0644)

		// Call with nil startedAt - should still return error
		_, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err == nil {
			t.Fatal("expected error with corrupt plan frontmatter, got nil")
		}
	})
}

// TestReadStatus_CountingAccuracyWithNilStartedAt verifies counts work with new signature
func TestReadStatus_CountingAccuracyWithNilStartedAt(t *testing.T) {
	// Expected failure: ReadStatus signature needs nil parameter in all counting tests

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Add multiple items
	bf, _ := backlog.NewFile(gromitDir)
	bf.Add(&backlog.Idea{ID: "idea-1", Text: "Idea one", Status: ""})
	bf.Add(&backlog.Idea{ID: "idea-2", Text: "Idea two", Status: ""})
	bf.Add(&backlog.Idea{ID: "idea-3", Text: "Idea three", Status: "refined"})

	os.WriteFile(filepath.Join(specsDir, "spec-a.md"), []byte("# Spec A"), 0644)
	os.WriteFile(filepath.Join(specsDir, "spec-b.md"), []byte("# Spec B"), 0644)

	os.WriteFile(filepath.Join(plansDir, "spec-a.md"), []byte("---\ndecomposed: false\n---\n# Plan A"), 0644)

	// Call with nil startedAt
	status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	if status.UnrefinedCount != 2 {
		t.Errorf("UnrefinedCount = %d, want 2", status.UnrefinedCount)
	}

	if len(status.UnplannedSpecs) != 1 {
		t.Errorf("len(UnplannedSpecs) = %d, want 1", len(status.UnplannedSpecs))
	}

	if len(status.UndecomposedPlans) != 1 {
		t.Errorf("len(UndecomposedPlans) = %d, want 1", len(status.UndecomposedPlans))
	}
}
