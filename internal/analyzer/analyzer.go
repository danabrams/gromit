package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// Category represents the type of failure
type Category string

const (
	CategorySyntax         Category = "syntax"           // Typo, missing import, wrong API
	CategoryLogic          Category = "logic"            // Algorithm wrong, edge case missed
	CategoryEnvironment    Category = "environment"      // Wrong tool version, missing dep
	CategoryUnclearSpec    Category = "unclear_spec"     // Spec is ambiguous
	CategoryMissingContext Category = "missing_context"  // Didn't know about existing code
	CategoryTestFlake      Category = "test_flake"       // Non-deterministic test failure
	CategoryTaskTooComplex Category = "task_too_complex" // Task scope too large for single iteration
	CategoryHardStopAction Category = "hard_stop_action" // Dangerous/irreversible action requiring explicit approval

	// maxFailureOutputLen is the maximum length of failure output to analyze.
	// Claude's context is large enough to handle this without performance issues,
	// but we truncate to avoid excessive token usage for extremely long outputs.
	maxFailureOutputLen = 8000
)

// Analysis represents the result of analyzing a failure
type Analysis struct {
	Category    Category `json:"category"`
	Recoverable bool     `json:"recoverable"`
	RootCause   string   `json:"root_cause"`
	Learning    *string  `json:"learning,omitempty"` // nil if no learning extracted
	Suggestion  string   `json:"suggestion"`
}

// LearningCategory maps failure categories to learning categories
func (a *Analysis) LearningCategory() string {
	if a == nil {
		return "gotchas"
	}
	switch a.Category {
	case CategoryMissingContext:
		return "conventions"
	case CategoryEnvironment:
		return "gotchas"
	case CategoryLogic:
		return "patterns"
	default:
		return "gotchas"
	}
}

// ProviderRunner is an interface for running analysis prompts.
// It matches the subset of provider.Provider methods used by Analyzer.
type ProviderRunner interface {
	Run(ctx context.Context, prompt string, tier string) (*provider.Result, error)
}

// Analyzer performs failure analysis using a Provider
type Analyzer struct {
	provider ProviderRunner
	tier     string
	renderer PromptRenderer
}

// PromptRenderer is an interface for rendering analysis prompts.
type PromptRenderer interface {
	RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error)
}

// NewAnalyzer creates a new analyzer. Returns an error if provider or renderer is nil.
func NewAnalyzer(p ProviderRunner, tier string, renderer PromptRenderer) (*Analyzer, error) {
	if p == nil {
		return nil, fmt.Errorf("provider is nil")
	}
	if renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	return &Analyzer{
		provider: p,
		tier:     tier,
		renderer: renderer,
	}, nil
}

// claudeClientAdapter adapts claude.Client to ProviderRunner interface
type claudeClientAdapter struct {
	client *claude.Client
}

// NewClaudeClientAdapter creates an adapter from a Claude client
func NewClaudeClientAdapter(client *claude.Client) ProviderRunner {
	return &claudeClientAdapter{client: client}
}

// Run implements ProviderRunner by calling the underlying Claude client and converting the result
func (a *claudeClientAdapter) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if a.client == nil {
		return nil, fmt.Errorf("claude client is nil")
	}

	claudeResult, err := a.client.Run(ctx, prompt, tier)
	if err != nil {
		return nil, err
	}

	if claudeResult == nil {
		return nil, fmt.Errorf("claude returned nil result")
	}

	// Convert claude.Result to provider.Result
	return &provider.Result{
		Success:  claudeResult.Success,
		Output:   claudeResult.Output,
		ExitCode: claudeResult.ExitCode,
		Duration: claudeResult.Duration,
	}, nil
}

// Analyze analyzes a failure and returns structured insights
func (a *Analyzer) Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*Analysis, error) {
	if a == nil {
		return nil, fmt.Errorf("analyzer is nil")
	}
	if b == nil {
		return nil, fmt.Errorf("bead is nil")
	}
	if a.provider == nil {
		return nil, fmt.Errorf("provider is nil")
	}
	if a.renderer == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	// Truncate failure output if too long
	if len(failureOutput) > maxFailureOutputLen {
		failureOutput = failureOutput[:maxFailureOutputLen] + "\n\n[... truncated ...]"
	}

	analyzeCtx := &prompt.AnalyzeContext{
		BeadID:          b.ID,
		BeadTitle:       b.Title,
		BeadDescription: b.Description,
		FailureOutput:   failureOutput,
	}

	analysisPrompt, err := a.renderer.RenderAnalyze(analyzeCtx)
	if err != nil {
		return nil, fmt.Errorf("rendering analysis prompt: %w", err)
	}

	result, err := a.provider.Run(ctx, analysisPrompt, a.tier)
	if err != nil {
		return nil, fmt.Errorf("running analysis: %w", err)
	}

	if result == nil {
		return &Analysis{
			Category:    CategoryLogic,
			Recoverable: false,
			RootCause:   "Analysis returned no result",
			Suggestion:  "Escalate to stronger model",
		}, nil
	}

	if !result.Success {
		// Analysis itself failed - return a default
		return &Analysis{
			Category:    CategoryLogic,
			Recoverable: false,
			RootCause:   "Analysis failed to complete",
			Suggestion:  "Escalate to stronger model",
		}, nil
	}

	// Parse JSON from output
	analysis, err := parseAnalysisOutput(result.Output)
	if err != nil {
		if fallback := parseAnalysisOutputHeuristic(result.Output); fallback != nil {
			return fallback, nil
		}
		// Parsing failed - return a default
		return &Analysis{
			Category:    CategoryLogic,
			Recoverable: false,
			RootCause:   "Could not parse analysis output",
			Suggestion:  "Escalate to stronger model",
		}, nil
	}

	return analysis, nil
}

func parseAnalysisOutput(output string) (*Analysis, error) {
	var analysis Analysis
	if err := jsonutil.ExtractObject(output, &analysis); err != nil {
		return nil, fmt.Errorf("parsing analysis output: %w", err)
	}

	// Validate category
	switch analysis.Category {
	case CategorySyntax, CategoryLogic, CategoryEnvironment,
		CategoryUnclearSpec, CategoryMissingContext, CategoryTestFlake,
		CategoryTaskTooComplex, CategoryHardStopAction:
		// Valid
	default:
		analysis.Category = CategoryLogic // Default
	}

	return &analysis, nil
}

func parseAnalysisOutputHeuristic(output string) *Analysis {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}

	category := inferCategoryFromText(trimmed)
	recoverable, hasRecoverable := inferRecoverableFromText(trimmed)
	rootCause := extractPrefixedLineValue(trimmed, "root cause:", "root_cause:", "cause:")
	suggestion := extractPrefixedLineValue(trimmed, "suggestion:", "fix:", "next step:")

	// If we can't infer anything useful, don't claim success.
	if rootCause == "" && suggestion == "" && category == CategoryLogic && !hasRecoverable {
		return nil
	}

	if rootCause == "" {
		rootCause = "Analysis output was not valid JSON"
	}
	if suggestion == "" {
		suggestion = "Escalate to stronger model"
	}

	return &Analysis{
		Category:    category,
		Recoverable: recoverable,
		RootCause:   rootCause,
		Suggestion:  suggestion,
	}
}

func inferCategoryFromText(text string) Category {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "hard_stop_action"),
		strings.Contains(lower, "hard stop action"),
		strings.Contains(lower, "hard_stop"),
		strings.Contains(lower, "hard stop"):
		return CategoryHardStopAction
	case strings.Contains(lower, "task_too_complex"), strings.Contains(lower, "task too complex"):
		return CategoryTaskTooComplex
	case strings.Contains(lower, "unclear_spec"), strings.Contains(lower, "unclear spec"):
		return CategoryUnclearSpec
	case strings.Contains(lower, "missing_context"), strings.Contains(lower, "missing context"):
		return CategoryMissingContext
	case strings.Contains(lower, "test_flake"), strings.Contains(lower, "test flake"):
		return CategoryTestFlake
	case strings.Contains(lower, "environment"):
		return CategoryEnvironment
	case strings.Contains(lower, "syntax"):
		return CategorySyntax
	case strings.Contains(lower, "logic"):
		return CategoryLogic
	default:
		return CategoryLogic
	}
}

func inferRecoverableFromText(text string) (recoverable bool, found bool) {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "recoverable=true"),
		strings.Contains(lower, "recoverable: true"),
		strings.Contains(lower, "recoverable = true"):
		return true, true
	case strings.Contains(lower, "recoverable=false"),
		strings.Contains(lower, "recoverable: false"),
		strings.Contains(lower, "recoverable = false"):
		return false, true
	default:
		return false, false
	}
}

func extractPrefixedLineValue(text string, prefixes ...string) string {
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		line = strings.TrimLeft(line, "-*0123456789. \t")
		lower := strings.ToLower(line)
		for _, prefix := range prefixes {
			if strings.HasPrefix(lower, prefix) {
				value := strings.TrimSpace(line[len(prefix):])
				if value != "" {
					return value
				}
			}
		}
	}
	return ""
}
