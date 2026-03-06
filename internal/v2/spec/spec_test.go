package spec_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func writeSpecFile(t *testing.T, specsDir, id string, frontmatter map[string]string) {
    t.Helper()

	path := filepath.Join(specsDir, id+".md")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create spec file: %v", err)
	}
	defer f.Close()

	if len(frontmatter) > 0 {
		f.WriteString("---\n")
		for key, value := range frontmatter {
			f.WriteString(key)
			f.WriteString(": ")
			f.WriteString(value)
			f.WriteString("\n")
		}
		f.WriteString("---\n")
	}
    fmt.Fprintln(f, "# spec body")
}

func TestListReadyOnlyIncludesEligibleSpecs(t *testing.T) {
    t.Parallel()

    specsDir := filepath.Join(t.TempDir(), "specs")
    if err := os.MkdirAll(specsDir, 0o755); err != nil {
        t.Fatalf("setup specs dir: %v", err)
    }

    writeSpecFile(t, specsDir, "done", map[string]string{
        "id":       "done",
        "accepted": "true",
    })
    writeSpecFile(t, specsDir, "ready", map[string]string{
        "id": "ready",
    })
    writeSpecFile(t, specsDir, "pending", map[string]string{
        "id":         "pending",
        "depends_on": "[done]",
    })
    writeSpecFile(t, specsDir, "blocked", map[string]string{
        "id":         "blocked",
        "depends_on": "[missing]",
    })

    readySpecs, err := spec.ListReady(specsDir)
    if err != nil {
        t.Fatalf("ListReady error = %v", err)
    }

    readySet := make(map[string]spec.ReadySpec)
    for _, entry := range readySpecs {
        readySet[entry.ID] = entry
    }

    if ready := readySet["ready"]; ready.ID != "ready" || ready.Path == "" {
        t.Fatalf("ready spec missing from list: %+v", ready)
    }
    if pending := readySet["pending"]; pending.ID != "pending" || pending.Path == "" {
        t.Fatalf("pending spec missing from list: %+v", pending)
    }
    if _, found := readySet["done"]; found {
        t.Fatalf("accepted spec should not be reported as ready")
    }
    if _, found := readySet["blocked"]; found {
        t.Fatalf("blocked spec should not be reported as ready")
    }
}
