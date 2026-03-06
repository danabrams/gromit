package dep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- helpers ----------------------------------------------------------------

// createTestSpecsDir creates a temporary specs directory for testing.
func createTestSpecsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	specsDir := filepath.Join(dir, ".gromit", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	return specsDir
}

// writeTestSpecFile writes a spec markdown file with YAML frontmatter.
func writeTestSpecFile(t *testing.T, specsDir, id, stage string, depends []string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", id))
	if len(depends) > 0 {
		sb.WriteString("depends_on:\n")
		for _, dep := range depends {
			sb.WriteString(fmt.Sprintf("  - %s\n", dep))
		}
	}
	if stage != "" {
		sb.WriteString(fmt.Sprintf("stage: %s\n", stage))
	}
	sb.WriteString("---\n")
	sb.WriteString("# spec body\n")

	path := filepath.Join(specsDir, id+".md")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("writing spec file: %v", err)
	}
}

// --- NewSpecDependencyGate --------------------------------------------------

func TestNewSpecDependencyGate_EmptyDir_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := NewSpecDependencyGate("")
	if err == nil {
		t.Fatal("expected error for empty specs dir, got nil")
	}
}

func TestNewSpecDependencyGate_ValidDir_Succeeds(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gate == nil {
		t.Fatal("expected non-nil gate")
	}
}

// --- EnsureSpecReady --------------------------------------------------------

func TestEnsureSpecReady_NoDependencies_Succeeds(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "standalone", "", nil)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	if err := gate.EnsureSpecReady(context.Background(), "standalone"); err != nil {
		t.Fatalf("EnsureSpecReady() = %v, want nil", err)
	}
}

func TestEnsureSpecReady_AllDependenciesSatisfied_Succeeds(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "parent", "done", nil)
	writeTestSpecFile(t, specsDir, "child", "", []string{"parent"})

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	if err := gate.EnsureSpecReady(context.Background(), "child"); err != nil {
		t.Fatalf("EnsureSpecReady() = %v, want nil", err)
	}
}

func TestEnsureSpecReady_UnsatisfiedDependency_ReturnsErrorWithBlockingIDs(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "prereq", "", nil) // not done
	writeTestSpecFile(t, specsDir, "child", "", []string{"prereq"})

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	err = gate.EnsureSpecReady(context.Background(), "child")
	if err == nil {
		t.Fatal("expected error for unsatisfied dependency, got nil")
	}

	var depErr *SpecDependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *SpecDependencyError, got %T: %v", err, err)
	}

	blocking := depErr.BlockingIDs()
	if len(blocking) != 1 || blocking[0] != "prereq" {
		t.Fatalf("BlockingIDs() = %v, want [prereq]", blocking)
	}

	if depErr.SpecID != "child" {
		t.Fatalf("SpecID = %q, want %q", depErr.SpecID, "child")
	}
}

func TestEnsureSpecReady_MultipleDependencies_MixedSatisfaction(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "done-dep", "done", nil)
	writeTestSpecFile(t, specsDir, "pending-dep", "", nil) // not done
	writeTestSpecFile(t, specsDir, "child", "", []string{"done-dep", "pending-dep"})

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	err = gate.EnsureSpecReady(context.Background(), "child")
	if err == nil {
		t.Fatal("expected error when one dependency is unsatisfied")
	}

	var depErr *SpecDependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *SpecDependencyError, got %T: %v", err, err)
	}

	blocking := depErr.BlockingIDs()
	if len(blocking) != 1 || blocking[0] != "pending-dep" {
		t.Fatalf("BlockingIDs() = %v, want [pending-dep]", blocking)
	}
}

func TestEnsureSpecReady_DependencyOnNonExistentSpec_ReturnsErrorWithBlockingID(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "child", "", []string{"ghost"})

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	err = gate.EnsureSpecReady(context.Background(), "child")
	if err == nil {
		t.Fatal("expected error for dependency on non-existent spec, got nil")
	}

	var depErr *SpecDependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *SpecDependencyError, got %T: %v", err, err)
	}

	blocking := depErr.BlockingIDs()
	if len(blocking) != 1 || blocking[0] != "ghost" {
		t.Fatalf("BlockingIDs() = %v, want [ghost]", blocking)
	}
}

func TestEnsureSpecReady_SpecNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	err = gate.EnsureSpecReady(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing spec file, got nil")
	}
}

// --- ListReady --------------------------------------------------------------

func TestListReady_MixOfReadyAndBlocked(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "done-parent", "done", nil)
	writeTestSpecFile(t, specsDir, "child-ready", "", []string{"done-parent"})
	writeTestSpecFile(t, specsDir, "pending", "", nil)
	writeTestSpecFile(t, specsDir, "blocked-child", "", []string{"pending"})
	writeTestSpecFile(t, specsDir, "independent", "", nil)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	ready, err := gate.ListReady(context.Background())
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	readySet := make(map[string]bool, len(ready))
	for _, id := range ready {
		readySet[id] = true
	}

	if !readySet["child-ready"] {
		t.Errorf("child-ready should be ready (dep is done): got %v", ready)
	}
	if !readySet["independent"] {
		t.Errorf("independent should be ready (no deps): got %v", ready)
	}
	if !readySet["pending"] {
		t.Errorf("pending should be ready (no deps, not done): got %v", ready)
	}
	if readySet["blocked-child"] {
		t.Errorf("blocked-child should NOT be ready (dep pending): got %v", ready)
	}
}

func TestListReady_ExcludesDoneSpecs(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "finished", "done", nil)
	writeTestSpecFile(t, specsDir, "in-progress", "", nil)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	ready, err := gate.ListReady(context.Background())
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	for _, id := range ready {
		if id == "finished" {
			t.Fatalf("ListReady() should exclude done specs, but returned %q", id)
		}
	}

	if len(ready) != 1 || ready[0] != "in-progress" {
		t.Fatalf("ListReady() = %v, want [in-progress]", ready)
	}
}

func TestListReady_EmptySpecsDirectory(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	ready, err := gate.ListReady(context.Background())
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	if len(ready) != 0 {
		t.Fatalf("ListReady() = %v, want empty slice", ready)
	}
}

func TestListReady_ResultIsSorted(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "zebra", "", nil)
	writeTestSpecFile(t, specsDir, "alpha", "", nil)
	writeTestSpecFile(t, specsDir, "middle", "", nil)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	ready, err := gate.ListReady(context.Background())
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	want := []string{"alpha", "middle", "zebra"}
	if len(ready) != len(want) {
		t.Fatalf("ListReady() = %v, want %v", ready, want)
	}
	for i, id := range ready {
		if id != want[i] {
			t.Fatalf("ListReady()[%d] = %q, want %q", i, id, want[i])
		}
	}
}

func TestListReady_IgnoresNonMarkdownFiles(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "real-spec", "", nil)

	// Write a non-markdown file that should be ignored.
	nonMDPath := filepath.Join(specsDir, "notes.txt")
	if err := os.WriteFile(nonMDPath, []byte("not a spec"), 0o644); err != nil {
		t.Fatalf("writing non-md file: %v", err)
	}

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	ready, err := gate.ListReady(context.Background())
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	if len(ready) != 1 || ready[0] != "real-spec" {
		t.Fatalf("ListReady() = %v, want [real-spec]", ready)
	}
}

// --- SpecDependencyError ----------------------------------------------------

func TestSpecDependencyError_ErrorMessage(t *testing.T) {
	t.Parallel()
	err := &SpecDependencyError{
		SpecID:   "child",
		Blocking: []string{"dep-a", "dep-b"},
	}

	msg := err.Error()
	if !strings.Contains(msg, "child") {
		t.Errorf("error message should contain spec ID: %s", msg)
	}
	if !strings.Contains(msg, "dep-a") || !strings.Contains(msg, "dep-b") {
		t.Errorf("error message should contain blocking IDs: %s", msg)
	}
}

func TestSpecDependencyError_BlockingIDs_ReturnsCopy(t *testing.T) {
	t.Parallel()
	err := &SpecDependencyError{
		SpecID:   "child",
		Blocking: []string{"dep-a"},
	}

	ids := err.BlockingIDs()
	ids[0] = "mutated"

	if err.Blocking[0] == "mutated" {
		t.Fatal("BlockingIDs() should return a copy, not a reference to internal state")
	}
}

func TestSpecDependencyError_NilReceiver(t *testing.T) {
	t.Parallel()
	var err *SpecDependencyError

	if err.Error() != "" {
		t.Errorf("nil SpecDependencyError.Error() should return empty string")
	}
	if ids := err.BlockingIDs(); ids != nil {
		t.Errorf("nil SpecDependencyError.BlockingIDs() should return nil, got %v", ids)
	}
}
