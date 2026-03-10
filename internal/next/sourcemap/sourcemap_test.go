package sourcemap

import (
	"encoding/json"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
)

func TestBuildFromFacts(t *testing.T) {
	// Create facts that match the file-tree extractor output format
	// Content is JSON with path, language, lines fields
	content1, _ := json.Marshal(map[string]any{"path": "main.go", "language": "go", "lines": 25})
	content2, _ := json.Marshal(map[string]any{"path": "internal/auth/auth.go", "language": "go", "lines": 100})
	content3, _ := json.Marshal(map[string]any{"path": "README.md", "language": "markdown", "lines": 10})

	facts := []fact.Fact{
		fact.New("file-tree-main.go", fact.Observed, string(content1), "file-tree"),
		fact.New("file-tree-internal-auth-auth.go", fact.Observed, string(content2), "file-tree"),
		fact.New("file-tree-README.md", fact.Observed, string(content3), "file-tree"),
		// Non-file-tree fact should be ignored
		fact.New("gomod-001", fact.Observed, "module example.com", "go-module"),
	}

	sm := BuildFromFacts(facts)
	if len(sm.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(sm.Entries))
	}

	// Verify one entry
	found := false
	for _, e := range sm.Entries {
		if e.Path == "main.go" {
			found = true
			if e.Language != "go" {
				t.Errorf("language = %q, want %q", e.Language, "go")
			}
			if e.Lines != 25 {
				t.Errorf("lines = %d, want %d", e.Lines, 25)
			}
		}
	}
	if !found {
		t.Error("main.go entry not found in source map")
	}
}
