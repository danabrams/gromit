package spec_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/v2/spec"
)

func TestSpecCheckDependenciesReportsBlockingSpecs(t *testing.T) {
	t.Parallel()

	specsDir := filepath.Join(t.TempDir(), "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("setup specs dir: %v", err)
	}

	writeSpecFile(t, specsDir, "prereq", map[string]string{
		"id":       "prereq",
		"accepted": "false",
	})
	writeSpecFile(t, specsDir, "child", map[string]string{
		"id":         "child",
		"depends_on": "[prereq]",
	})

	childPath := filepath.Join(specsDir, "child.md")
	child, err := spec.Load(childPath)
	if err != nil {
		t.Fatalf("load child spec: %v", err)
	}

	err = child.CheckDependencies(specsDir)
	var depErr *spec.SpecDependenciesError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected SpecDependenciesError, got %v", err)
	}
	if len(depErr.Blocking) != 1 || depErr.Blocking[0] != "prereq" {
		t.Fatalf("blocking deps = %v, want [prereq]", depErr.Blocking)
	}
}

func TestLoadParsesDependenciesFrontMatter(t *testing.T) {
	t.Parallel()

	specsDir := filepath.Join(t.TempDir(), "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("setup specs dir: %v", err)
	}

	writeSpecFile(t, specsDir, "prereq", map[string]string{"id": "prereq", "accepted": "true"})

	child := `---
id: child
dependencies:
  - prereq
accepted: false
---
# child spec
`
	writeRawSpecFile(t, specsDir, "child.md", child)

	childPath := filepath.Join(specsDir, "child.md")
	loaded, err := spec.Load(childPath)
	if err != nil {
		t.Fatalf("load child spec: %v", err)
	}

	want := []string{"prereq"}
	if !reflect.DeepEqual(loaded.DependsOn, want) {
		t.Fatalf("depends_on = %v, want %v", loaded.DependsOn, want)
	}
}

func TestLoadExtractsArchitectureDirectionAndTestStrategy(t *testing.T) {
	t.Parallel()

	specsDir := filepath.Join(t.TempDir(), "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("setup specs dir: %v", err)
	}

	body := `
## Architecture Direction
Prefer modular pipeline execution.

## Test Strategy
Run unit and integration specs.
`
	writeSpecFileWithBody(t, specsDir, "sections", map[string]string{"id": "sections"}, body)

	path := filepath.Join(specsDir, "sections.md")
	loaded, err := spec.Load(path)
	if err != nil {
		t.Fatalf("load sections spec: %v", err)
	}

	if got := loaded.ArchitectureDirection; got != "Prefer modular pipeline execution." {
		t.Fatalf("ArchitectureDirection = %q, want %q", got, "Prefer modular pipeline execution.")
	}
	if got := loaded.TestStrategy; got != "Run unit and integration specs." {
		t.Fatalf("TestStrategy = %q, want %q", got, "Run unit and integration specs.")
	}
}

func TestSpecCheckDependenciesDedupsBlockingIDs(t *testing.T) {
	t.Parallel()

	specsDir := filepath.Join(t.TempDir(), "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("setup specs dir: %v", err)
	}

	writeSpecFile(t, specsDir, "a", map[string]string{"id": "a"})
	writeSpecFile(t, specsDir, "b", map[string]string{"id": "b"})

	child := `---
id: child
depends_on:
  - b
  - a
  - b
accepted: false
---
# child spec
`
	writeRawSpecFile(t, specsDir, "child.md", child)

	childPath := filepath.Join(specsDir, "child.md")
	loadedChild, err := spec.Load(childPath)
	if err != nil {
		t.Fatalf("load child spec: %v", err)
	}

	err = loadedChild.CheckDependencies(specsDir)
	var depErr *spec.SpecDependenciesError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected SpecDependenciesError, got %v", err)
	}
	if !reflect.DeepEqual(depErr.Blocking, []string{"a", "b"}) {
		t.Fatalf("blocking deps = %v, want [a b]", depErr.Blocking)
	}
}

func TestListReadyReturnsSpecsSortedByID(t *testing.T) {
	t.Parallel()

	specsDir := filepath.Join(t.TempDir(), "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("setup specs dir: %v", err)
	}

	writeSpecFile(t, specsDir, "beta", map[string]string{"id": "beta"})
	writeSpecFile(t, specsDir, "alpha", map[string]string{"id": "alpha"})
	writeSpecFile(t, specsDir, "done", map[string]string{"id": "done", "accepted": "true"})

	readySpecs, err := spec.ListReady(specsDir)
	if err != nil {
		t.Fatalf("ListReady error = %v", err)
	}
	if want := 2; len(readySpecs) != want {
		t.Fatalf("ready specs = %d, want %d", len(readySpecs), want)
	}
	if readySpecs[0].ID != "alpha" || readySpecs[1].ID != "beta" {
		t.Fatalf("ready IDs = %v, want [alpha beta]", []string{readySpecs[0].ID, readySpecs[1].ID})
	}
	if readySpecs[0].Path != filepath.Join(specsDir, "alpha.md") {
		t.Fatalf("alpha path = %q, want %q", readySpecs[0].Path, filepath.Join(specsDir, "alpha.md"))
	}
}

func writeSpecFile(t *testing.T, specsDir, id string, frontmatter map[string]string) {
	writeSpecFileWithBody(t, specsDir, id, frontmatter, "# spec body")
}

func writeSpecFileWithBody(t *testing.T, specsDir, id string, frontmatter map[string]string, body string) {
	t.Helper()

	path := filepath.Join(specsDir, id+".md")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create spec file: %v", err)
	}
	defer f.Close()

	if len(frontmatter) > 0 {
		fmt.Fprintln(f, "---")
		for key, value := range frontmatter {
			fmt.Fprintf(f, "%s: %s\n", key, value)
		}
		fmt.Fprintln(f, "---")
	}
	if body != "" {
		fmt.Fprintln(f, body)
	}
}

func writeRawSpecFile(t *testing.T, specsDir, filename, content string) {
	t.Helper()

	path := filepath.Join(specsDir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec file %s: %v", filename, err)
	}
}
