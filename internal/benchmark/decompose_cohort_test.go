package benchmark

import (
    "context"
    "os"
    "path/filepath"
    "reflect"
    "testing"

    "github.com/danabrams/gromit/internal/bead"
)

func TestDecomposeCohortSelector_Select(t *testing.T) {
    ctx := context.Background()
    plansDir := t.TempDir()
    specs := []string{
        "spec-omega",
        "spec-yankee",
        "spec-alpha",
        "spec-bravo",
        "spec-charlie",
        "spec-delta",
        "spec-echo",
        "spec-open",
        "spec-empty",
    }
    for _, spec := range specs {
        createPlanFile(t, plansDir, spec)
    }

    store := &fakeLabelLister{
        beads: map[string][]*bead.Bead{
            "spec:spec-omega":  makeBeads("closed", "closed", "closed", "closed", "closed"),
            "spec:spec-yankee": makeBeads("closed", "closed", "closed", "closed"),
            "spec:spec-alpha": makeBeads("closed", "closed", "closed"),
            "spec:spec-bravo": makeBeads("closed", "closed", "closed"),
            "spec:spec-charlie": makeBeads("closed", "closed"),
            "spec:spec-delta":  makeBeads("closed"),
            "spec:spec-echo":   makeBeads("closed"),
            "spec:spec-open":   makeBeads("open"),
            "spec:spec-empty":  nil,
        },
    }

    selector := NewDecomposeCohortSelector(store, plansDir)
    candidates := []string{
        "spec-delta",
        "spec-echo",
        "spec-open",
        "spec-omega",
        "spec-alpha",
        "spec-yankee",
        "spec-bravo",
        "spec-charlie",
        "spec-empty",
    }

    got, err := selector.Select(ctx, candidates)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    want := []string{"spec-omega", "spec-yankee", "spec-alpha", "spec-bravo", "spec-charlie"}
    if !reflect.DeepEqual(got, want) {
        t.Fatalf("Select() = %v, want %v", got, want)
    }
}

func createPlanFile(t *testing.T, plansDir, spec string) {
    t.Helper()
    path := filepath.Join(plansDir, spec+".md")
    if err := os.WriteFile(path, []byte("# Plan"), 0o644); err != nil {
        t.Fatalf("write plan file: %v", err)
    }
}

func makeBeads(statuses ...string) []*bead.Bead {
    beads := make([]*bead.Bead, len(statuses))
    for i, status := range statuses {
        beads[i] = &bead.Bead{Status: status}
    }
    return beads
}

type fakeLabelLister struct {
    beads map[string][]*bead.Bead
}

func (f *fakeLabelLister) ListWithLabel(ctx context.Context, label string) ([]*bead.Bead, error) {
    return f.beads[label], nil
}
