package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidationCommandsExtractor_Makefile(t *testing.T) {
	dir := t.TempDir()
	makefile := `.PHONY: test lint build

test:
	go test ./...

lint:
	golangci-lint run ./...

build:
	go build -o bin/app ./cmd/app
`
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0o644)

	ext := NewValidationCommandsExtractor()
	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) < 3 {
		t.Fatalf("expected at least 3 facts (test, lint, build), got %d", len(facts))
	}

	for _, f := range facts {
		if f.Category.String() != "observed" {
			t.Errorf("fact %q has category %v, want observed", f.ID, f.Category)
		}
	}
}

func TestValidationCommandsExtractor_CIWorkflow(t *testing.T) {
	dir := t.TempDir()
	workflowDir := filepath.Join(dir, ".github", "workflows")
	os.MkdirAll(workflowDir, 0o755)

	workflow := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go test ./...
      - run: golangci-lint run
`
	os.WriteFile(filepath.Join(workflowDir, "ci.yml"), []byte(workflow), 0o644)

	ext := NewValidationCommandsExtractor()
	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) < 2 {
		t.Fatalf("expected at least 2 facts from CI workflow, got %d", len(facts))
	}
}

func TestValidationCommandsExtractor_NoFiles(t *testing.T) {
	dir := t.TempDir()
	ext := NewValidationCommandsExtractor()

	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract should not error on missing files: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(facts))
	}
}

func TestValidationCommandsExtractor_MultiCommandTarget(t *testing.T) {
	dir := t.TempDir()
	makefile := `.PHONY: test

test:
	go test ./...
	go test -race ./...
`
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0o644)

	ext := NewValidationCommandsExtractor()
	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) < 2 {
		t.Fatalf("expected at least 2 facts for multi-command target, got %d", len(facts))
	}
}

func TestValidationCommandsExtractor_Name(t *testing.T) {
	ext := NewValidationCommandsExtractor()
	if ext.Name() != "validation-commands" {
		t.Errorf("Name = %q, want %q", ext.Name(), "validation-commands")
	}
}
