package runner

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/provider"
)

func TestExtractRequirementsViaLLM_ReturnsParsedItems(t *testing.T) {
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return &provider.Result{Success: true, Output: "item one\nitem two\nitem three"}, nil
	}
	got := extractRequirementsViaLLM(context.Background(), "My Title", "some description", invoke)
	want := []string{"item one", "item two", "item three"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractRequirementsViaLLM_ReturnsNilOnError(t *testing.T) {
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return nil, fmt.Errorf("provider unavailable")
	}
	got := extractRequirementsViaLLM(context.Background(), "Title", "desc", invoke)
	if got != nil {
		t.Fatalf("expected nil on error, got %v", got)
	}
}

func TestExtractRequirementsFromDescription_BulletedList(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "dash bullets",
			input: "- alpha\n- beta\n- gamma",
			want:  []string{"alpha", "beta", "gamma"},
		},
		{
			name:  "asterisk bullets",
			input: "* one\n* two",
			want:  []string{"one", "two"},
		},
		{
			name:  "plus bullets",
			input: "+ first\n+ second",
			want:  []string{"first", "second"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRequirementsFromDescription(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d items, want %d: %v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("item %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestExtractRequirementsFromDescription_HeaderPrefixedList(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "Requirements header",
			input: "Requirements:\nfoo\nbar",
			want:  []string{"foo", "bar"},
		},
		{
			name:  "Includes header",
			input: "Includes:\nalpha\nbeta",
			want:  []string{"alpha", "beta"},
		},
		{
			name:  "Delivers header",
			input: "Delivers:\nfeature A\nfeature B",
			want:  []string{"feature A", "feature B"},
		},
		{
			name:  "header line itself is not included",
			input: "Requirements:\nonly item",
			want:  []string{"only item"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRequirementsFromDescription(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d items, want %d: %v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("item %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestExtractRequirementsFromDescription_SemicolonSeparated(t *testing.T) {
	input := "do this; do that; do the other"
	got := extractRequirementsFromDescription(input)
	want := []string{"do this", "do that", "do the other"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractRequirementsFromDescription_NumberedList(t *testing.T) {
	input := "Some preamble\n1. do this\n2. do that\n3. do the other thing"
	got := extractRequirementsFromDescription(input)
	want := []string{"do this", "do that", "do the other thing"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestTddExpectedOutputsOrTitle_Layer2UsedWhenExpectedOutputsEmpty verifies
// that when ExpectedOutputs is empty and the description contains parseable
// requirements, those parsed requirements are returned instead of the title.
func TestTddExpectedOutputsOrTitle_Layer2UsedWhenExpectedOutputsEmpty(t *testing.T) {
	b := &bead.Bead{
		ID:          "test-bead",
		Title:       "Bead Title",
		Description: "- req one\n- req two",
	}
	got := tddExpectedOutputsOrTitle(b)
	want := []string{"req one", "req two"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestTddExpectedOutputsOrTitle_Layer1TakesPriorityOverDescription verifies
// that ExpectedOutputs (Layer 1) takes priority over description parsing (Layer 2).
func TestTddExpectedOutputsOrTitle_Layer1TakesPriorityOverDescription(t *testing.T) {
	b := &bead.Bead{
		ID:              "test-bead",
		Title:           "Bead Title",
		Description:     "- desc req one\n- desc req two",
		ExpectedOutputs: []string{"explicit output"},
	}
	got := tddExpectedOutputsOrTitle(b)
	want := []string{"explicit output"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	if got[0] != want[0] {
		t.Errorf("got %q, want %q", got[0], want[0])
	}
}

// TestTddExpectedOutputsOrTitle_TitleFallbackWhenDescriptionUnparseable verifies
// that the bead title is used when both ExpectedOutputs is empty and the
// description contains no parseable requirements.
func TestTddExpectedOutputsOrTitle_TitleFallbackWhenDescriptionUnparseable(t *testing.T) {
	b := &bead.Bead{
		ID:          "test-bead",
		Title:       "My Bead Title",
		Description: "some prose that contains no parseable list items",
	}
	got := tddExpectedOutputsOrTitle(b)
	want := []string{"My Bead Title"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	if got[0] != want[0] {
		t.Errorf("got %q, want %q", got[0], want[0])
	}
}
