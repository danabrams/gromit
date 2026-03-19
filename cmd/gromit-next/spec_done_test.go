package main

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestParseDoneDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTime time.Time
		wantOK   bool
	}{
		{
			name:     "valid DONE prefix with date",
			input:    "DONE 2026-03-19",
			wantTime: time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
			wantOK:   true,
		},
		{
			name:     "valid DONE prefix with date and content after",
			input:    "DONE 2026-03-19\n# Spec Title",
			wantTime: time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
			wantOK:   true,
		},
		{
			name:     "DONE without date",
			input:    "DONE",
			wantTime: time.Time{},
			wantOK:   true,
		},
		{
			name:     "DONE with space but no date",
			input:    "DONE ",
			wantTime: time.Time{},
			wantOK:   true,
		},
		{
			name:     "DONE with malformed date",
			input:    "DONE invalid-date",
			wantTime: time.Time{},
			wantOK:   true,
		},
		{
			name:     "no DONE prefix",
			input:    "DRAFT 2026-03-19",
			wantTime: time.Time{},
			wantOK:   false,
		},
		{
			name:     "DONE in middle of line",
			input:    "Not DONE 2026-03-19",
			wantTime: time.Time{},
			wantOK:   false,
		},
		{
			name:     "empty string",
			input:    "",
			wantTime: time.Time{},
			wantOK:   false,
		},
		{
			name:     "DONE mid-file is not treated as done",
			input:    "# My Spec\nDONE 2026-03-19\n",
			wantTime: time.Time{},
			wantOK:   false,
		},
		{
			name:     "DONATION prefix is not treated as done",
			input:    "DONATION list\n",
			wantTime: time.Time{},
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTime, gotOK := ParseDoneDate(tt.input)
			if gotOK != tt.wantOK {
				t.Errorf("ParseDoneDate(%q) ok = %v, want %v", tt.input, gotOK, tt.wantOK)
			}
			if gotOK && gotTime != tt.wantTime {
				t.Errorf("ParseDoneDate(%q) time = %v, want %v", tt.input, gotTime, tt.wantTime)
			}
		})
	}
}

func TestDeriveSpecStatusFromContent_Done(t *testing.T) {
	tests := []struct {
		name    string
		content string
		runs    []runstore.RunState
		want    string
	}{
		{
			name:    "DONE prefix takes precedence over runs",
			content: "DONE 2026-03-19\n# Spec",
			runs: []runstore.RunState{
				{SpecID: "spec1", Status: runstore.StatusRunning},
			},
			want: "done",
		},
		{
			name:    "DONE without date still returns done",
			content: "DONE\n# Spec",
			runs:    []runstore.RunState{},
			want:    "done",
		},
		{
			name:    "DONE with malformed date still returns done",
			content: "DONE invalid\n# Spec",
			runs:    []runstore.RunState{},
			want:    "done",
		},
		{
			name:    "DONE takes precedence over DRAFT (but DONE comes first)",
			content: "DONE 2026-03-19\nDRAFT something",
			runs:    []runstore.RunState{},
			want:    "done",
		},
		{
			name:    "DRAFT without DONE returns draft",
			content: "DRAFT\n# Spec",
			runs:    []runstore.RunState{},
			want:    "draft",
		},
		{
			name:    "neither DONE nor DRAFT with no runs returns ready",
			content: "# Spec Title",
			runs:    []runstore.RunState{},
			want:    "ready",
		},
		{
			name:    "neither DONE nor DRAFT with completed run returns completed",
			content: "# Spec Title",
			runs: []runstore.RunState{
				{SpecID: "spec1", Status: runstore.StatusCompleted},
			},
			want: "completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveSpecStatusFromContent("spec1", tt.runs, tt.content)
			if got != tt.want {
				t.Errorf("DeriveSpecStatusFromContent(...) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSpecListSortsDoneToBottom(t *testing.T) {
	tests := []struct {
		name      string
		specs     []string
		contents  map[string]string
		wantOrder []string
	}{
		{
			name:  "done specs sorted to bottom with valid date",
			specs: []string{"spec-a", "spec-b", "spec-c"},
			contents: map[string]string{
				"spec-a": "DONE 2026-03-15\nContent",
				"spec-b": "# Regular spec",
				"spec-c": "DONE 2026-03-19\nContent",
			},
			wantOrder: []string{"spec-b", "spec-a", "spec-c"},
		},
		{
			name:  "done specs sorted to bottom with no date",
			specs: []string{"spec-a", "spec-b", "spec-c"},
			contents: map[string]string{
				"spec-a": "DONE\nContent",
				"spec-b": "# Regular spec",
				"spec-c": "# Another spec",
			},
			wantOrder: []string{"spec-b", "spec-c", "spec-a"},
		},
		{
			name:  "done specs sorted to bottom with malformed date",
			specs: []string{"spec-a", "spec-b"},
			contents: map[string]string{
				"spec-a": "DONE invalid-date\nContent",
				"spec-b": "# Regular spec",
			},
			wantOrder: []string{"spec-b", "spec-a"},
		},
		{
			name:  "non-done specs preserve original order",
			specs: []string{"spec-c", "spec-a", "spec-b"},
			contents: map[string]string{
				"spec-a": "# Spec A",
				"spec-b": "# Spec B",
				"spec-c": "# Spec C",
			},
			wantOrder: []string{"spec-c", "spec-a", "spec-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortSpecsByDone(tt.specs, tt.contents)
			if len(got) != len(tt.wantOrder) {
				t.Errorf("sortSpecsByDone() returned %d specs, want %d", len(got), len(tt.wantOrder))
				return
			}
			for i, spec := range got {
				if spec != tt.wantOrder[i] {
					t.Errorf("sortSpecsByDone()[%d] = %q, want %q", i, spec, tt.wantOrder[i])
				}
			}
		})
	}
}

func TestSpecListFormatsStatusWithDate(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "done with valid date shows done (YYYY-MM-DD)",
			content: "DONE 2026-03-19\n# Spec",
			want:    "done (2026-03-19)",
		},
		{
			name:    "done without date shows just done",
			content: "DONE\n# Spec",
			want:    "done",
		},
		{
			name:    "done with malformed date shows just done",
			content: "DONE invalid-date\n# Spec",
			want:    "done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSpecStatusWithDate("spec1", []runstore.RunState{}, tt.content)
			if got != tt.want {
				t.Errorf("formatSpecStatusWithDate(...) = %q, want %q", got, tt.want)
			}
		})
	}
}
