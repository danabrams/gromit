package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

type methodologyTestInvoker struct{}

func (m *methodologyTestInvoker) Run(_ context.Context, _, _ string) (*provider.Result, error) {
	return nil, fmt.Errorf("unexpected Run call")
}

func (m *methodologyTestInvoker) StreamRun(_ context.Context, _ string, _ string, _ io.Writer, _ provider.EventHandler, _ provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}

type methodologyTestRenderer struct {
	lastMethod string
}

func (m *methodologyTestRenderer) RenderBuild(_, _ string, _ []string) (string, error) {
	m.lastMethod = "build"
	return "build prompt", nil
}

func (m *methodologyTestRenderer) RenderTDDBuild(_, _ string, _ []string) (string, error) {
	m.lastMethod = "tdd"
	return "tdd prompt", nil
}

func (m *methodologyTestRenderer) RenderRefactorBuild(_, _ string, _ []string) (string, error) {
	m.lastMethod = "refactor"
	return "refactor prompt", nil
}

func TestResolveBuildStrategy_BeadLabelOverridesConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Methodology.BuildStrategy = "single_pass"

	b := &bead.Bead{Labels: []string{"build_strategy:tdd"}}

	if got := resolveBuildStrategy(cfg, b); got != "tdd" {
		t.Fatalf("resolveBuildStrategy() = %q, want %q", got, "tdd")
	}
}

func TestBuildRun_SinglePassConfig_SkipsRefactorMethodology(t *testing.T) {
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: "high",
			P1: "medium",
			P2: "low",
		},
	}
	b := &bead.Bead{
		ID:       "bead-1",
		Title:    "Implement behavior",
		Priority: 1,
		Labels:   []string{"build_strategy:single_pass", "refactor:true"},
	}

	renderer := &methodologyTestRenderer{}
	stage := execute.New(&methodologyTestInvoker{}, renderer, io.Discard)
	_, err := stage.Run(context.Background(), pipeline.Input{
		Bead:      b,
		Config:    cfg,
		Iteration: 1,
		Deadline:  time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if renderer.lastMethod != "build" {
		t.Fatalf("renderer method = %q, want %q for single_pass strategy", renderer.lastMethod, "build")
	}
}

func TestBuildRun_BuildStrategyLabels_LastTDDWins(t *testing.T) {
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: "high",
			P1: "medium",
			P2: "low",
		},
		Methodology: config.MethodologyConfig{
			BuildStrategy: "single_pass",
		},
	}
	b := &bead.Bead{
		ID:       "bead-2",
		Title:    "Implement behavior",
		Priority: 1,
		Labels:   []string{"build_strategy:single_pass", "build_strategy:tdd"},
	}

	renderer := &methodologyTestRenderer{}
	stage := execute.New(&methodologyTestInvoker{}, renderer, io.Discard)
	_, err := stage.Run(context.Background(), pipeline.Input{
		Bead:      b,
		Config:    cfg,
		Iteration: 1,
		Deadline:  time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if renderer.lastMethod != "tdd" {
		t.Fatalf("renderer method = %q, want %q when build_strategy:tdd is the last override", renderer.lastMethod, "tdd")
	}
}

func TestResolveBuildStrategy_LastSinglePassWins(t *testing.T) {
	cfg := &config.Config{}
	cfg.Methodology.BuildStrategy = "tdd"

	b := &bead.Bead{
		Labels: []string{"build_strategy:tdd", "build_strategy:single_pass"},
	}

	if got := resolveBuildStrategy(cfg, b); got != "single_pass" {
		t.Fatalf("resolveBuildStrategy() = %q, want %q when single_pass is the last explicit override", got, "single_pass")
	}
}

func TestExtractRequirementsViaLLM_ReturnsParsedItems(t *testing.T) {
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return &provider.Result{Success: true, Output: "item one\nitem two\nitem three"}, nil
	}
	got := extractRequirementsViaLLM(context.Background(), nil, "My Title", "some description", invoke)
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
	extractRequirementsViaLLM(context.Background(), nil, "Title", longDesc, invoke)
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
	extractRequirementsViaLLM(context.Background(), nil, "Title", "desc", invoke)
	if capturedTier != provider.TierLow {
		t.Errorf("got tier %q, want %q", capturedTier, provider.TierLow)
	}
}

func TestExtractRequirementsViaLLM_UsesUtilityRoutingTierWhenEnabled(t *testing.T) {
	cfg := &config.Config{
		TokenEfficiency: config.TokenEfficiencyConfig{
			Routing: config.TokenEfficiencyRoutingConfig{
				Enabled:     true,
				UtilityTier: provider.TierMedium,
			},
		},
	}
	var capturedTier string
	invoke := func(_ context.Context, _ string, tier string) (*provider.Result, error) {
		capturedTier = tier
		return &provider.Result{Success: true, Output: "req one\nreq two"}, nil
	}
	extractRequirementsViaLLM(context.Background(), cfg, "Title", "desc", invoke)
	if capturedTier != provider.TierMedium {
		t.Errorf("got tier %q, want %q", capturedTier, provider.TierMedium)
	}
}

func TestExtractRequirementsViaLLM_Integration_UsesUtilityCategoryTaskOverrideTier(t *testing.T) {
	cfg := &config.Config{
		TokenEfficiency: config.TokenEfficiencyConfig{
			Routing: config.TokenEfficiencyRoutingConfig{
				Enabled:     true,
				UtilityTier: provider.TierLow,
				TaskOverrides: map[string]string{
					"summarization": provider.TierMedium,
				},
			},
		},
	}

	var capturedTier string
	invoke := func(_ context.Context, _ string, tier string) (*provider.Result, error) {
		capturedTier = tier
		return &provider.Result{Success: true, Output: "req one\nreq two"}, nil
	}

	extractRequirementsViaLLM(context.Background(), cfg, "Title", "desc", invoke)

	if capturedTier != provider.TierMedium {
		t.Fatalf("captured tier = %q, want %q", capturedTier, provider.TierMedium)
	}
}

func TestExtractRequirementsViaLLM_SkipsBlankLines(t *testing.T) {
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return &provider.Result{Success: true, Output: "item one\n\n\nitem two\n\nitem three\n"}, nil
	}
	got := extractRequirementsViaLLM(context.Background(), nil, "Title", "desc", invoke)
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
	got := extractRequirementsViaLLM(context.Background(), nil, "Title", "desc", invoke)
	if got != nil {
		t.Fatalf("expected nil for single item, got %v", got)
	}
}

func TestExtractRequirementsViaLLM_ReturnsNilOnError(t *testing.T) {
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return nil, fmt.Errorf("provider unavailable")
	}
	got := extractRequirementsViaLLM(context.Background(), nil, "Title", "desc", invoke)
	if got != nil {
		t.Fatalf("expected nil on error, got %v", got)
	}
}

func TestApplyLayer3Requirements_TriggersAndReplacesOutputsWhenOnlyOneItem(t *testing.T) {
	invoke := func(_ context.Context, _ string, _ string) (*provider.Result, error) {
		return &provider.Result{Success: true, Output: "llm req one\nllm req two"}, nil
	}
	got, activated := applyLayer3Requirements(context.Background(), nil, []string{"My Title"}, "My Title", "no parseable list", invoke)
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
	got, activated := applyLayer3Requirements(context.Background(), nil, outputs, "Title", "desc", invoke)
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
	got, activated := applyLayer3Requirements(context.Background(), nil, outputs, "My Title", "no parseable list", invoke)
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

func TestExtractRequirementsFromDescription_CommaSeparated(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple comma list",
			input: "formatRun, formatDuration, formatHealth",
			want:  []string{"formatRun", "formatDuration", "formatHealth"},
		},
		{
			name:  "Oxford comma with and",
			input: "formatRun, formatDuration, and formatHealth",
			want:  []string{"formatRun", "formatDuration", "formatHealth"},
		},
		{
			name:  "single item no comma should not split",
			input: "formatRun",
			want:  nil,
		},
		{
			name:  "two items with comma",
			input: "formatRun, formatDuration",
			want:  []string{"formatRun", "formatDuration"},
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

func TestExtractRequirementsFromDescription_HeaderWithCommasOnSameLine(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "Functions header with comma list",
			input: "Functions: formatRun, formatDuration, formatHealth",
			want:  []string{"formatRun", "formatDuration", "formatHealth"},
		},
		{
			name:  "Requirements header with comma list",
			input: "Requirements: auth, logging, caching",
			want:  []string{"auth", "logging", "caching"},
		},
		{
			name:  "Lowercase header with comma list",
			input: "functions: formatRun, formatDuration",
			want:  []string{"formatRun", "formatDuration"},
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

func TestExtractRequirementsFromDescription_FunctionsHeader(t *testing.T) {
	input := "Functions:\nformatRun\nformatDuration\nformatHealth"
	got := extractRequirementsFromDescription(input)
	want := []string{"formatRun", "formatDuration", "formatHealth"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractRequirementsFromDescription_MixedBulletsAndCommas(t *testing.T) {
	input := "- alpha\n- beta\nExtras: gamma, delta"
	got := extractRequirementsFromDescription(input)
	want := []string{"alpha", "beta", "gamma", "delta"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestExtractRequirementsViaLLM_PromptRequestsIndividualItems verifies that
// the prompt sent to the LLM asks for individual, fine-grained items rather
// than summaries, so haiku enumerates each deliverable separately.
func TestExtractRequirementsViaLLM_PromptRequestsIndividualItems(t *testing.T) {
	var capturedPrompt string
	invoke := func(_ context.Context, prompt string, _ string) (*provider.Result, error) {
		capturedPrompt = prompt
		return &provider.Result{Success: true, Output: "item one\nitem two"}, nil
	}
	extractRequirementsViaLLM(context.Background(), nil, "Title", "desc with formatRun, formatDuration", invoke)

	requiredPhrases := []string{
		"individual",
		"do not summarize",
		"each function",
	}
	promptLower := strings.ToLower(capturedPrompt)
	for _, phrase := range requiredPhrases {
		if !strings.Contains(promptLower, phrase) {
			t.Errorf("prompt missing required phrase %q.\nPrompt was:\n%s", phrase, capturedPrompt)
		}
	}
}

// TestExtractRequirementsViaLLM_PromptDoesNotGroupItems verifies the prompt
// explicitly instructs against grouping or summarizing.
func TestExtractRequirementsViaLLM_PromptDoesNotGroupItems(t *testing.T) {
	var capturedPrompt string
	invoke := func(_ context.Context, prompt string, _ string) (*provider.Result, error) {
		capturedPrompt = prompt
		return &provider.Result{Success: true, Output: "a\nb"}, nil
	}
	extractRequirementsViaLLM(context.Background(), nil, "Title", "desc", invoke)

	promptLower := strings.ToLower(capturedPrompt)
	if !strings.Contains(promptLower, "do not group") {
		t.Errorf("prompt should instruct against grouping.\nPrompt was:\n%s", capturedPrompt)
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
