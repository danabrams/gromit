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
// When accepted is true, the frontmatter includes "accepted: true".
func writeTestSpecFile(t *testing.T, specsDir, id string, accepted bool, depends []string) {
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
	if accepted {
		sb.WriteString("accepted: true\n")
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
	writeTestSpecFile(t, specsDir, "standalone", false, nil)

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
	writeTestSpecFile(t, specsDir, "parent", true, nil)
	writeTestSpecFile(t, specsDir, "child", false, []string{"parent"})

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
	writeTestSpecFile(t, specsDir, "prereq", false, nil) // not accepted
	writeTestSpecFile(t, specsDir, "child", false, []string{"prereq"})

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
	writeTestSpecFile(t, specsDir, "done-dep", true, nil)
	writeTestSpecFile(t, specsDir, "pending-dep", false, nil) // not accepted
	writeTestSpecFile(t, specsDir, "child", false, []string{"done-dep", "pending-dep"})

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
	writeTestSpecFile(t, specsDir, "child", false, []string{"ghost"})

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
	writeTestSpecFile(t, specsDir, "done-parent", true, nil)
	writeTestSpecFile(t, specsDir, "child-ready", false, []string{"done-parent"})
	writeTestSpecFile(t, specsDir, "pending", false, nil)
	writeTestSpecFile(t, specsDir, "blocked-child", false, []string{"pending"})
	writeTestSpecFile(t, specsDir, "independent", false, nil)

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

func TestListReady_ExcludesAcceptedSpecs(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "finished", true, nil)
	writeTestSpecFile(t, specsDir, "in-progress", false, nil)

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
			t.Fatalf("ListReady() should exclude accepted specs, but returned %q", id)
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
	writeTestSpecFile(t, specsDir, "zebra", false, nil)
	writeTestSpecFile(t, specsDir, "alpha", false, nil)
	writeTestSpecFile(t, specsDir, "middle", false, nil)

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
	writeTestSpecFile(t, specsDir, "real-spec", false, nil)

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

// --- EnsureSpecReady (error cases) ------------------------------------------

func TestEnsureSpecReady_NilGate_ReturnsError(t *testing.T) {
	t.Parallel()
	var gate *SpecDependencyGate

	err := gate.EnsureSpecReady(context.Background(), "any")
	if err == nil {
		t.Fatal("expected error for nil gate, got nil")
	}
}

func TestEnsureSpecReady_MalformedFrontmatter_ReturnsError(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)

	// Write a file with unclosed frontmatter delimiter.
	path := filepath.Join(specsDir, "bad.md")
	if err := os.WriteFile(path, []byte("---\nid: bad\nno closing delimiter"), 0o644); err != nil {
		t.Fatalf("writing malformed spec: %v", err)
	}

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	err = gate.EnsureSpecReady(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error for malformed frontmatter, got nil")
	}
}

func TestEnsureSpecReady_DuplicateDependencies_DeduplicatesBlockers(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "dep-a", false, nil) // not accepted
	writeTestSpecFile(t, specsDir, "child", false, []string{"dep-a", "dep-a", "dep-a"})

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	err = gate.EnsureSpecReady(context.Background(), "child")
	if err == nil {
		t.Fatal("expected error for unsatisfied dependency")
	}

	var depErr *SpecDependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *SpecDependencyError, got %T: %v", err, err)
	}

	blocking := depErr.BlockingIDs()
	if len(blocking) != 1 {
		t.Fatalf("duplicate deps should be deduplicated: BlockingIDs() = %v, want [dep-a]", blocking)
	}
	if blocking[0] != "dep-a" {
		t.Fatalf("BlockingIDs() = %v, want [dep-a]", blocking)
	}
}

func TestEnsureSpecReady_SpecIsDone_StillSucceeds(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "done-spec", true, nil)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	// EnsureSpecReady checks deps, not acceptance — an accepted spec with no deps should pass.
	if err := gate.EnsureSpecReady(context.Background(), "done-spec"); err != nil {
		t.Fatalf("EnsureSpecReady() = %v, want nil (done spec with no deps)", err)
	}
}

// --- ListReady (error cases) ------------------------------------------------

func TestListReady_NilGate_ReturnsError(t *testing.T) {
	t.Parallel()
	var gate *SpecDependencyGate

	_, err := gate.ListReady(context.Background())
	if err == nil {
		t.Fatal("expected error for nil gate, got nil")
	}
}

func TestListReady_NonExistentDirectory_ReturnsError(t *testing.T) {
	t.Parallel()
	gate, err := NewSpecDependencyGate("/tmp/does-not-exist-gromit-test-" + t.Name())
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	_, err = gate.ListReady(context.Background())
	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

func TestListReady_MalformedSpecFile_ReturnsError(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)

	// Write a valid spec and a malformed one.
	writeTestSpecFile(t, specsDir, "good", false, nil)
	path := filepath.Join(specsDir, "bad.md")
	if err := os.WriteFile(path, []byte("---\nid: bad\nno closing delimiter"), 0o644); err != nil {
		t.Fatalf("writing malformed spec: %v", err)
	}

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	_, err = gate.ListReady(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed spec file, got nil")
	}
}

func TestListReady_IgnoresSubdirectories(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "real", false, nil)

	// Create a subdirectory — should be skipped.
	subDir := filepath.Join(specsDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("creating subdirectory: %v", err)
	}

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	ready, err := gate.ListReady(context.Background())
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	if len(ready) != 1 || ready[0] != "real" {
		t.Fatalf("ListReady() = %v, want [real]", ready)
	}
}

func TestListReady_AllSpecsAccepted_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "a", true, nil)
	writeTestSpecFile(t, specsDir, "b", true, nil)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	ready, err := gate.ListReady(context.Background())
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	if len(ready) != 0 {
		t.Fatalf("ListReady() = %v, want empty", ready)
	}
}

func TestListReady_ChainedDependencies_OnlyLeafReady(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	// Chain: c depends on b, b depends on a. Only a has no deps.
	writeTestSpecFile(t, specsDir, "a", false, nil)
	writeTestSpecFile(t, specsDir, "b", false, []string{"a"})
	writeTestSpecFile(t, specsDir, "c", false, []string{"b"})

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	ready, err := gate.ListReady(context.Background())
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	// Only 'a' should be ready (no deps, not done).
	// 'b' blocked by 'a' (not done), 'c' blocked by 'b' (not done).
	if len(ready) != 1 || ready[0] != "a" {
		t.Fatalf("ListReady() = %v, want [a]", ready)
	}
}

// --- blockingDependencies (direct tests) ------------------------------------

func TestBlockingDependencies_NilMetadata_ReturnsNil(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	blockers, err := gate.blockingDependencies(nil)
	if err != nil {
		t.Fatalf("blockingDependencies(nil) error = %v", err)
	}
	if blockers != nil {
		t.Fatalf("blockingDependencies(nil) = %v, want nil", blockers)
	}
}

func TestBlockingDependencies_EmptyDependsOn_ReturnsNil(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	meta := &specMetadata{ID: "test", DependsOn: nil}
	blockers, err := gate.blockingDependencies(meta)
	if err != nil {
		t.Fatalf("blockingDependencies() error = %v", err)
	}
	if blockers != nil {
		t.Fatalf("blockingDependencies() = %v, want nil", blockers)
	}
}

func TestBlockingDependencies_AllDone_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "dep-x", true, nil)
	writeTestSpecFile(t, specsDir, "dep-y", true, nil)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	meta := &specMetadata{ID: "child", DependsOn: []string{"dep-x", "dep-y"}}
	blockers, err := gate.blockingDependencies(meta)
	if err != nil {
		t.Fatalf("blockingDependencies() error = %v", err)
	}
	if len(blockers) != 0 {
		t.Fatalf("blockingDependencies() = %v, want empty", blockers)
	}
}

func TestBlockingDependencies_MissingDepFile_TreatedAsBlocker(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	meta := &specMetadata{ID: "child", DependsOn: []string{"nonexistent"}}
	blockers, err := gate.blockingDependencies(meta)
	if err != nil {
		t.Fatalf("blockingDependencies() error = %v", err)
	}
	if len(blockers) != 1 || blockers[0] != "nonexistent" {
		t.Fatalf("blockingDependencies() = %v, want [nonexistent]", blockers)
	}
}

func TestBlockingDependencies_DuplicateDeps_Deduplicated(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "dep", false, nil) // not accepted

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	meta := &specMetadata{ID: "child", DependsOn: []string{"dep", "dep", "dep"}}
	blockers, err := gate.blockingDependencies(meta)
	if err != nil {
		t.Fatalf("blockingDependencies() error = %v", err)
	}
	if len(blockers) != 1 || blockers[0] != "dep" {
		t.Fatalf("blockingDependencies() = %v, want [dep]", blockers)
	}
}

func TestBlockingDependencies_WhitespaceOnlyDeps_Skipped(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	meta := &specMetadata{ID: "child", DependsOn: []string{"", "  ", "\t"}}
	blockers, err := gate.blockingDependencies(meta)
	if err != nil {
		t.Fatalf("blockingDependencies() error = %v", err)
	}
	if len(blockers) != 0 {
		t.Fatalf("blockingDependencies() = %v, want empty (whitespace-only deps skipped)", blockers)
	}
}

func TestBlockingDependencies_ResultIsSorted(t *testing.T) {
	t.Parallel()
	specsDir := createTestSpecsDir(t)
	writeTestSpecFile(t, specsDir, "zzz", false, nil) // not accepted
	writeTestSpecFile(t, specsDir, "aaa", false, nil) // not accepted
	writeTestSpecFile(t, specsDir, "mmm", false, nil) // not accepted

	gate, err := NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}

	meta := &specMetadata{ID: "child", DependsOn: []string{"zzz", "aaa", "mmm"}}
	blockers, err := gate.blockingDependencies(meta)
	if err != nil {
		t.Fatalf("blockingDependencies() error = %v", err)
	}

	want := []string{"aaa", "mmm", "zzz"}
	if len(blockers) != len(want) {
		t.Fatalf("blockingDependencies() = %v, want %v", blockers, want)
	}
	for i, b := range blockers {
		if b != want[i] {
			t.Fatalf("blockingDependencies()[%d] = %q, want %q", i, b, want[i])
		}
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
