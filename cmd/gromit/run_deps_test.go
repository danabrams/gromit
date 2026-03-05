package main

import (
    "fmt"
    "testing"

    "github.com/danabrams/gromit/internal/config"
)

func TestRunDepsResolveLegacyRunSpecScopeFallsBackToLegacyLabel(t *testing.T) {
    deps := runDeps{
        runHasOpenBeadsForLabelFn: func(label string) (bool, error) {
            if label != "spec:legacy-spec" {
                t.Fatalf("unexpected label %s", label)
            }
            return true, nil
        },
    }

    cfg := &config.Config{}
    labels, err := deps.resolveLegacyRunSpecScope(cfg, ".gromit/specs", "legacy-spec", fmt.Errorf("spec not found"))
    if err != nil {
        t.Fatalf("expected nil error, got %v", err)
    }
    if len(labels) != 1 || labels[0] != "spec:legacy-spec" {
        t.Fatalf("labels = %v, want [spec:legacy-spec]", labels)
    }
}
