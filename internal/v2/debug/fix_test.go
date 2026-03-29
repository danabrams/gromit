package debug

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyPlanWritesFile(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "foo.txt")
	if err := os.WriteFile(filePath, []byte("original"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	plan := FixPlan{
		Scope: ScopeLearnable,
		Edits: []FileEdit{
			{Path: "foo.txt", NewContent: []byte("updated")},
		},
	}

	result, err := ApplyPlan(root, plan)
	if err != nil {
		t.Fatalf("ApplyPlan error: %v", err)
	}
	if result.Scope != ScopeLearnable {
		t.Fatalf("scope = %v, want %v", result.Scope, ScopeLearnable)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "updated" {
		t.Fatalf("file content = %q, want %q", string(data), "updated")
	}
}
