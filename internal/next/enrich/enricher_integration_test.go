package enrich

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
	"github.com/danabrams/gromit/internal/provider"
)

// FakeProvider implements provider.Provider for testing LLMEnricher.
type FakeProvider struct {
	LastPrompt string
	LastTier   string
	Response   string
	CostUSD    float64
	InTok      int
	OutTok     int
}

func (f *FakeProvider) Name() string                       { return "fake" }
func (f *FakeProvider) ModelForTier(tier string) string     { return "fake-" + tier }
func (f *FakeProvider) IsUsageLimitError(*provider.Result, error) bool { return false }
func (f *FakeProvider) IsValidationPassed(*provider.Result) bool      { return true }
func (f *FakeProvider) IsScopeTooLarge(*provider.Result) (bool, string) {
	return false, ""
}
func (f *FakeProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return nil, provider.ErrStreamNotSupported
}
func (f *FakeProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *FakeProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	f.LastPrompt = prompt
	f.LastTier = tier
	return &provider.Result{
		Success:      true,
		Output:       f.Response,
		CostUSD:      f.CostUSD,
		InputTokens:  f.InTok,
		OutputTokens: f.OutTok,
	}, nil
}

func TestLLMEnricher_ImplementsInterface(t *testing.T) {
	var _ CategoryEnricher = (*LLMEnricher)(nil)
}

func TestLLMEnricher_EnrichCallsProviderAndParsesResponse(t *testing.T) {
	fp := &FakeProvider{
		Response: `[
			{
				"statement": "cmd/gromit is the CLI entrypoint",
				"rationale": "main.go lives in cmd/gromit",
				"evidence_refs": ["cmd/gromit/main.go"],
				"confidence": "high",
				"scope": "cmd/gromit"
			},
			{
				"statement": "internal/runner orchestrates the loop",
				"rationale": "runner package contains loop logic",
				"evidence_refs": ["internal/runner/runner.go", "internal/runner/loop.go"],
				"confidence": "medium",
				"scope": "internal/runner"
			}
		]`,
		CostUSD: 0.0042,
		InTok:   1500,
		OutTok:  300,
	}

	enricher := NewLLMEnricher(fp, "sonnet", "medium")

	observed := []fact.Fact{
		fact.New("obs-1", fact.Observed, "main.go exists in cmd/gromit", "file_scan"),
	}
	input := EnrichInput{
		ProjectName: "gromit",
		FileTree:    []string{"cmd/gromit/main.go", "internal/runner/runner.go"},
	}

	result, err := enricher.Enrich(context.Background(), CategoryEntrypoint, observed, input)
	if err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	// Verify provider was called with correct tier
	if fp.LastTier != "medium" {
		t.Errorf("provider tier = %q, want %q", fp.LastTier, "medium")
	}

	// Verify prompt contains category and observed facts
	if !strings.Contains(fp.LastPrompt, "entrypoint") {
		t.Error("prompt should contain category name 'entrypoint'")
	}
	if !strings.Contains(fp.LastPrompt, "main.go exists in cmd/gromit") {
		t.Error("prompt should contain observed fact content")
	}

	// Verify result fields
	if !result.Success {
		t.Errorf("Success = false, want true")
	}
	if result.Category != CategoryEntrypoint {
		t.Errorf("Category = %q, want %q", result.Category, CategoryEntrypoint)
	}
	if result.FactCount != 2 {
		t.Errorf("FactCount = %d, want 2", result.FactCount)
	}
	if len(result.Facts) != 2 {
		t.Fatalf("len(Facts) = %d, want 2", len(result.Facts))
	}

	// Verify cost/token tracking
	if result.CostUSD != 0.0042 {
		t.Errorf("CostUSD = %f, want 0.0042", result.CostUSD)
	}
	if result.InputTokens != 1500 {
		t.Errorf("InputTokens = %d, want 1500", result.InputTokens)
	}
	if result.OutputTokens != 300 {
		t.Errorf("OutputTokens = %d, want 300", result.OutputTokens)
	}

	// Verify fact fields
	f0 := result.Facts[0]
	if f0.Statement != "cmd/gromit is the CLI entrypoint" {
		t.Errorf("fact[0].Statement = %q", f0.Statement)
	}
	if f0.SourceType != "inferred" {
		t.Errorf("fact[0].SourceType = %q, want %q", f0.SourceType, "inferred")
	}
	if f0.Category != CategoryEntrypoint {
		t.Errorf("fact[0].Category = %q, want %q", f0.Category, CategoryEntrypoint)
	}
	if f0.Confidence != "high" {
		t.Errorf("fact[0].Confidence = %q, want %q", f0.Confidence, "high")
	}
	if len(f0.EvidenceRefs) != 1 || f0.EvidenceRefs[0] != "cmd/gromit/main.go" {
		t.Errorf("fact[0].EvidenceRefs = %v", f0.EvidenceRefs)
	}

	// Verify content-hash IDs are set and non-empty
	if f0.FactID == "" {
		t.Error("fact[0].FactID should be non-empty (content hash)")
	}
	f1 := result.Facts[1]
	if f1.FactID == "" {
		t.Error("fact[1].FactID should be non-empty (content hash)")
	}
	if f0.FactID == f1.FactID {
		t.Error("fact IDs should differ for different statements")
	}
}

func TestLLMEnricher_EnrichHandlesProviderError(t *testing.T) {
	fp := &FakeProvider{}
	// Override Run to return error
	enricher := NewLLMEnricher(fp, "sonnet", "medium")

	// Use a provider that returns error
	errProvider := &ErrorProvider{Err: fmt.Errorf("API rate limited")}
	enricher.provider = errProvider

	result, err := enricher.Enrich(context.Background(), CategoryRiskyArea, nil, EnrichInput{})
	if err == nil {
		t.Fatal("expected error from Enrich")
	}
	if result.Success {
		t.Error("Success should be false on error")
	}
	if result.Category != CategoryRiskyArea {
		t.Errorf("Category = %q, want %q", result.Category, CategoryRiskyArea)
	}
}

func TestLLMEnricher_EnrichHandlesMalformedJSON(t *testing.T) {
	fp := &FakeProvider{
		Response: `not valid json at all`,
		CostUSD:  0.001,
		InTok:    100,
		OutTok:   50,
	}

	enricher := NewLLMEnricher(fp, "haiku", "low")
	result, err := enricher.Enrich(context.Background(), CategoryGlossaryTerm, nil, EnrichInput{})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if result.Success {
		t.Error("Success should be false for malformed JSON")
	}
	// Cost should still be tracked even on parse failure
	if result.CostUSD != 0.001 {
		t.Errorf("CostUSD = %f, want 0.001 (should track cost even on parse error)", result.CostUSD)
	}
}

func TestLLMEnricher_BuildPromptPerCategory(t *testing.T) {
	// Verify that different categories produce different prompts
	observed := []fact.Fact{
		fact.New("o1", fact.Observed, "some fact", "test"),
	}
	input := EnrichInput{ProjectName: "myproject"}

	p1 := buildPrompt(CategoryEntrypoint, observed, input)
	p2 := buildPrompt(CategoryRiskyArea, observed, input)

	if p1 == p2 {
		t.Error("different categories should produce different prompts")
	}
	if !strings.Contains(p1, "entrypoint") {
		t.Error("entrypoint prompt should mention 'entrypoint'")
	}
	if !strings.Contains(p2, "risky_area") {
		t.Error("risky_area prompt should mention 'risky_area'")
	}
}

// ErrorProvider always returns an error from Run.
type ErrorProvider struct {
	Err error
}

func (e *ErrorProvider) Name() string                       { return "error" }
func (e *ErrorProvider) ModelForTier(tier string) string     { return "error-" + tier }
func (e *ErrorProvider) IsUsageLimitError(*provider.Result, error) bool { return false }
func (e *ErrorProvider) IsValidationPassed(*provider.Result) bool      { return false }
func (e *ErrorProvider) IsScopeTooLarge(*provider.Result) (bool, string) {
	return false, ""
}
func (e *ErrorProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return nil, provider.ErrStreamNotSupported
}
func (e *ErrorProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return nil, e.Err
}
func (e *ErrorProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return nil, e.Err
}
