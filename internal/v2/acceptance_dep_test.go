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
