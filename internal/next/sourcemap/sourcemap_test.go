package sourcemap

import (
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
)

func TestSourceMap_NormalizeNilFields(t *testing.T) {
	var sm SourceMap
	sm.NormalizeNilFields()
	if sm.Entries == nil {
		t.Error("Entries should be initialized, not nil")
	}
}

func TestBuildFromFacts(t *testing.T) {
	facts := []fact.Fact{
		fact.New("1", fact.Observed, `{"path":"main.go","language":"go","lines":10}`, "file-tree"),
		fact.New("2", fact.Observed, `{"path":"lib/util.py","language":"python","lines":42}`, "file-tree"),
		fact.New("3", fact.Observed, `{"path":"README.md","language":"markdown","lines":5}`, "file-tree-extra"),
	}

	sm := BuildFromFacts(facts)

	if len(sm.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(sm.Entries))
	}

	want := []Entry{
		{Path: "main.go", Language: "go", Lines: 10},
		{Path: "lib/util.py", Language: "python", Lines: 42},
		{Path: "README.md", Language: "markdown", Lines: 5},
	}
	for i, w := range want {
		got := sm.Entries[i]
		if got != w {
			t.Errorf("entry %d: got %+v, want %+v", i, got, w)
		}
	}
}

func TestBuildFromFacts_ExcludesNonFileTreeSources(t *testing.T) {
	facts := []fact.Fact{
		fact.New("1", fact.Observed, `{"path":"main.go","language":"go","lines":10}`, "file-tree"),
		fact.New("2", fact.Declared, `{"path":"config.yaml","language":"yaml","lines":20}`, "user-config"),
		fact.New("3", fact.Inferred, `some inferred content`, "analysis"),
		fact.New("4", fact.Observed, `{"path":"go.mod","language":"go","lines":3}`, "dependency-scan"),
	}

	sm := BuildFromFacts(facts)

	if len(sm.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(sm.Entries))
	}
	if sm.Entries[0].Path != "main.go" {
		t.Errorf("expected path main.go, got %s", sm.Entries[0].Path)
	}
}

func TestBuildFromFacts_SkipsMalformedJSON(t *testing.T) {
	facts := []fact.Fact{
		fact.New("1", fact.Observed, `{"path":"good.go","language":"go","lines":5}`, "file-tree"),
		fact.New("2", fact.Observed, `{not valid json`, "file-tree"),
		fact.New("3", fact.Observed, ``, "file-tree"),
		fact.New("4", fact.Observed, `{"path":"also-good.go","language":"go","lines":8}`, "file-tree"),
	}

	sm := BuildFromFacts(facts)

	if len(sm.Entries) != 2 {
		t.Fatalf("expected 2 entries (skipping malformed), got %d", len(sm.Entries))
	}
	if sm.Entries[0].Path != "good.go" {
		t.Errorf("entry 0: expected path good.go, got %s", sm.Entries[0].Path)
	}
	if sm.Entries[1].Path != "also-good.go" {
		t.Errorf("entry 1: expected path also-good.go, got %s", sm.Entries[1].Path)
	}
}

func TestBuildFromFacts_EmptyInput(t *testing.T) {
	sm := BuildFromFacts([]fact.Fact{})

	if sm.Entries == nil {
		t.Fatal("Entries should be non-nil empty slice, got nil")
	}
	if len(sm.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(sm.Entries))
	}
}

func TestBuildFromFacts_NilInput(t *testing.T) {
	sm := BuildFromFacts(nil)

	if sm.Entries == nil {
		t.Fatal("Entries should be non-nil empty slice, got nil")
	}
	if len(sm.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(sm.Entries))
	}
}
