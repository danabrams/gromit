package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
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

func TestExtractRequirementsViaLLM_TruncatesDescriptionTo2000Chars(t *testing.T) {
	longDesc := strings.Repeat("x", 3000)
	var capturedPrompt string
	invoke := func(_ context.Context, prompt string, _ string) (*provider.Result, error) {
		capturedPrompt = prompt
		return &provider.Result{Success: true, Output: "req one\nreq two"}, nil
	}
	extractRequirementsViaLLM(context.Background(), "Title", longDesc, invoke)
	truncated := strings.Repeat("x", 2000)
	if !strings.Contains(capturedPrompt, truncated) {
		t.Errorf("expected prompt to contain 2000-char description")
	}
	extra := strings.Repeat("x", 2001)
	if strings.Contains(capturedPrompt, extra) {
		t.Errorf("expected prompt to NOT contain more than 2000 chars of description")
	}
}

func TestExtractRequirementsViaLLM_InvokesAtTierLow(t *testing.T) {
	var capturedTier string
	invoke := func(_ context.Context, _ string, tier string) (*provider.Result, error) {
		capturedTier = tier
		return &provider.Result{Success: true, Output: "req one\nreq two"}, nil
	}
	extractRequirementsViaLLM(context.Background(), "Title", "desc", invoke)
	if capturedTier != provider.TierLow {
		t.Errorf("got tier %q, want %q", capturedTier, provider.TierLow)
	}
}

func TestExtractRequirementsViaLLM_SkipsBlankLines(t *testing.T) {
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return &provider.Result{Success: true, Output: "item one\n\n\nitem two\n\nitem three\n"}, nil
	}
	got := extractRequirementsViaLLM(context.Background(), "Title", "desc", invoke)
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

func TestExtractRequirementsViaLLM_ReturnsNilForFewerThanTwoItems(t *testing.T) {
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return &provider.Result{Success: true, Output: "only one item"}, nil
	}
	got := extractRequirementsViaLLM(context.Background(), "Title", "desc", invoke)
	if got != nil {
		t.Fatalf("expected nil for single item, got %v", got)
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

func TestApplyLayer3Requirements_TriggersAndReplacesOutputsWhenOnlyOneItem(t *testing.T) {
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return &provider.Result{Success: true, Output: "llm req one\nllm req two"}, nil
	}
	got, activated := applyLayer3Requirements(context.Background(), []string{"My Title"}, "My Title", "no parseable list", invoke)
	if !activated {
		t.Fatal("expected layer3 to be activated")
	}
	want := []string{"llm req one", "llm req two"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestApplyLayer3Requirements_DoesNotTriggerWhenOutputsHasMoreThanOneItem(t *testing.T) {
	outputs := []string{"req one", "req two"}
	called := false
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		called = true
		return nil, nil
	}
	got, activated := applyLayer3Requirements(context.Background(), outputs, "Title", "desc", invoke)
	if activated {
		t.Fatal("expected layer3 NOT to be activated when outputs > 1")
	}
	if called {
		t.Fatal("expected invoke NOT to be called when outputs > 1")
	}
	if len(got) != 2 {
		t.Fatalf("expected original outputs preserved, got %v", got)
	}
}

func TestApplyLayer3Requirements_TitleFallbackPreservedOnLayer3Failure(t *testing.T) {
	outputs := []string{"My Title"}
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return nil, fmt.Errorf("provider unavailable")
	}
	got, activated := applyLayer3Requirements(context.Background(), outputs, "My Title", "no parseable list", invoke)
	if activated {
		t.Fatal("expected layer3 NOT to be activated on invoke failure")
	}
	if len(got) != 1 || got[0] != "My Title" {
		t.Fatalf("expected title fallback preserved, got %v", got)
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

// TestAggregateTDDPhaseMetricsToResult_SumsCostAndTokens verifies that
// aggregateTDDPhaseMetricsToResult sums CostUSD, InputTokens, and OutputTokens
// from all PhaseMetrics into bc.Result, so the iteration log reflects totals
// across all TDD fresh-context cycle invocations rather than the last one only.
func TestAggregateTDDPhaseMetricsToResult_SumsCostAndTokens(t *testing.T) {
	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{
			PhaseMetrics: []runtypes.PhaseMetric{
				{CostUSD: 0.01, InputTokens: 100, OutputTokens: 50, Tier: "medium", Model: "sonnet"},
				{CostUSD: 0.02, InputTokens: 200, OutputTokens: 75, Tier: "medium", Model: "sonnet"},
			},
		},
	}

	aggregateTDDPhaseMetricsToResult(bc)

	wantCost := 0.01 + 0.02
	if bc.Result.CostUSD != wantCost {
		t.Errorf("CostUSD = %f, want %f", bc.Result.CostUSD, wantCost)
	}
	if bc.Result.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", bc.Result.InputTokens)
	}
	if bc.Result.OutputTokens != 125 {
		t.Errorf("OutputTokens = %d, want 125", bc.Result.OutputTokens)
	}
}

// TestAggregateTDDPhaseMetricsToResult_SetsModelToHighestTierModel verifies that
// aggregateTDDPhaseMetricsToResult sets bc.Result.Model to the model from the
// highest-tier PhaseMetric, so the iteration log names the most capable model used.
func TestAggregateTDDPhaseMetricsToResult_SetsModelToHighestTierModel(t *testing.T) {
	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{
			PhaseMetrics: []runtypes.PhaseMetric{
				{Tier: "low", Model: "claude-haiku-4-5"},
				{Tier: "medium", Model: "claude-sonnet-4-6"},
				{Tier: "high", Model: "claude-opus-4-6"},
				{Tier: "medium", Model: "claude-sonnet-4-6"},
			},
		},
	}

	aggregateTDDPhaseMetricsToResult(bc)

	if bc.Result.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q (highest-tier model)", bc.Result.Model, "claude-opus-4-6")
	}
}
