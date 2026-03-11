// Package sourcemap provides utilities for working with the source-map.json
// artifact produced by inspection.
//
// Source maps are used to select relevant files for context compilation,
// estimate token budgets, and detect structural changes between inspections.
//
// TODO: implement source map diffing (detect added/removed/renamed files)
// TODO: implement file selection by language, path glob, or role
// TODO: implement token budget estimation from file sizes
package sourcemap

import (
	"encoding/json"
	"strings"

	"github.com/danabrams/gromit/internal/next/fact"
)

// Entry represents a single file in the source map.
type Entry struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
}

// SourceMap holds the collection of file entries discovered during inspection.
type SourceMap struct {
	Entries []Entry `json:"entries"`
}

// NormalizeNilFields maps nil slices/maps to empty values.
// See CLAUDE.md nil-field normalization visibility convention.
func (sm *SourceMap) NormalizeNilFields() {
	if sm.Entries == nil {
		sm.Entries = []Entry{}
	}
}

// BuildFromFacts constructs a SourceMap from a slice of facts, filtering for
// facts whose Source begins with "file-tree" and unmarshalling their Content
// as Entry structs.
func BuildFromFacts(facts []fact.Fact) SourceMap {
	var entries []Entry
	for _, f := range facts {
		if !strings.HasPrefix(f.Source, "file-tree") {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(f.Content), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if entries == nil {
		entries = []Entry{}
	}
	return SourceMap{Entries: entries}
}
