package experiment

import "testing"

func TestLoadExperimentsEmptyDir(t *testing.T) {
    dir := t.TempDir()

    exps, err := LoadExperiments(dir)
    if err != nil {
        t.Fatalf("LoadExperiments returned error: %v", err)
    }
    if len(exps) != 0 {
        t.Fatalf("expected no experiments, got %d", len(exps))
    }
}
