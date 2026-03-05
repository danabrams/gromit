//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/dep"
)

func TestRun2BlocksSpecWhenDependenciesIncomplete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specsDir := createSpecsDir(t)

	writeSpecFile(t, specsDir, "prereq", "", nil)
	writeSpecFile(t, specsDir, "child", "", []string{"prereq"})

	gate, err := dep.NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new spec gate: %v", err)
	}

	err = gate.EnsureSpecReady(ctx, "child")
	if err == nil {
		t.Fatal("EnsureSpecReady() expected error when dependency is incomplete")
	}

	var blockingErr *dep.SpecDependencyError
	if !errors.As(err, &blockingErr) {
		t.Fatalf("expected SpecDependencyError, got %T: %v", err, err)
	}

	if got := blockingErr.BlockingIDs(); len(got) != 1 || got[0] != "prereq" {
		t.Fatalf("blocking IDs = %v, want [prereq]", got)
	}
}

func TestListReadyOnlyIncludesEligibleSpecs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specsDir := createSpecsDir(t)

	writeSpecFile(t, specsDir, "done-parent", "done", nil)
	writeSpecFile(t, specsDir, "child-ready", "", []string{"done-parent"})
	writeSpecFile(t, specsDir, "pending", "", nil)
	writeSpecFile(t, specsDir, "blocked-child", "", []string{"pending"})
	writeSpecFile(t, specsDir, "independent", "", nil)

	gate, err := dep.NewSpecDependencyGate(specsDir)
	if err != nil {
		t.Fatalf("new spec gate: %v", err)
	}

	ready, err := gate.ListReady(ctx)
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}

	readySet := make(map[string]bool, len(ready))
	for _, id := range ready {
		readySet[id] = true
	}

	if !readySet["child-ready"] {
		t.Fatalf("child-ready not reported as ready: %v", ready)
	}
	if !readySet["independent"] {
		t.Fatalf("independent not reported as ready: %v", ready)
	}
	if readySet["blocked-child"] {
		t.Fatalf("blocked-child should be excluded from ready list: %v", ready)
	}
	if readySet["done-parent"] {
		t.Fatalf("done-parent should not be returned once marked done: %v", ready)
	}
}

func TestBeadLoopSkipsBlockedBeadsUntilDependenciesComplete(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "leaf"},
		{ID: "mid", DependsOn: []bead.Dependency{{ID: "leaf"}}},
		{ID: "root", DependsOn: []bead.Dependency{{ID: "mid"}}},
	}

	scheduler := dep.NewBeadScheduler(beads)

	pick := scheduler.Next()
	if pick == nil || pick.ID != "leaf" {
		t.Fatalf("expected leaf first, got %v", pick)
	}
	scheduler.MarkComplete(pick.ID)

	pick = scheduler.Next()
	if pick == nil || pick.ID != "mid" {
		t.Fatalf("expected mid second, got %v", pick)
	}
	scheduler.MarkComplete(pick.ID)

	pick = scheduler.Next()
	if pick == nil || pick.ID != "root" {
		t.Fatalf("expected root third, got %v", pick)
	}
	scheduler.MarkComplete(pick.ID)

	if scheduler.Next() != nil {
		t.Fatal("expected no beads after all dependencies complete")
	}
}

func createSpecsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	specsDir := filepath.Join(dir, ".gromit", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	return specsDir
}

func writeSpecFile(t *testing.T, specsDir, id, stage string, depends []string) {
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
