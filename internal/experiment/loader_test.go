package experiment

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

func TestLoadExperimentsParsesYAML(t *testing.T) {
	dir := t.TempDir()
	expPath := filepath.Join(dir, "exp.yaml")
	expYAML := `id: exp-1
phase: build
description: build prompt
created: 2026-02-24T00:00:00Z
control:
  id: control
  template: PROMPT_build.md
variants:
  - id: variant-1
    template: PROMPT_build.md
`

	if err := os.WriteFile(expPath, []byte(expYAML), 0o644); err != nil {
		t.Fatalf("failed to write experiment file: %v", err)
	}

	exps, err := LoadExperiments(dir)
	if err != nil {
		t.Fatalf("LoadExperiments returned error: %v", err)
	}
	if len(exps) != 1 {
		t.Fatalf("expected one experiment, got %d", len(exps))
	}

	exp := exps[0]
	if exp.ID != "exp-1" {
		t.Fatalf("unexpected experiment ID: %q", exp.ID)
	}
	if exp.Phase != "build" {
		t.Fatalf("unexpected phase: %q", exp.Phase)
	}
	if exp.Created.Format(time.RFC3339) != "2026-02-24T00:00:00Z" {
		t.Fatalf("unexpected created time: %s", exp.Created)
	}
	if exp.Control == nil {
		t.Fatalf("control variant missing")
	}
	if exp.Control.Template != "PROMPT_build.md" {
		t.Fatalf("unexpected control template: %q", exp.Control.Template)
	}
	if len(exp.Variants) != 1 {
		t.Fatalf("expected one variant, got %d", len(exp.Variants))
	}
}
