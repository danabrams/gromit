package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/claude"
	"github.com/danabrams/ralph-runner/internal/prompt"
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

// Analyzer performs failure analysis using Claude
type Analyzer struct {
	claude   *claude.Client
	model    string
	renderer *prompt.Renderer
}

// NewAnalyzer creates a new analyzer. Returns nil if claudeClient or renderer is nil.
func NewAnalyzer(claudeClient *claude.Client, model string, renderer *prompt.Renderer) *Analyzer {
	if claudeClient == nil || renderer == nil {
		return nil
	}
	return &Analyzer{
		claude:   claudeClient,
		model:    model,
		renderer: renderer,
	}
}

// Analyze analyzes a failure and returns structured insights
func (a *Analyzer) Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*Analysis, error) {
	if a == nil {
		return nil, fmt.Errorf("analyzer is nil")
	}
	if b == nil {
		return nil, fmt.Errorf("bead is nil")
	}
	if a.claude == nil {
		return nil, fmt.Errorf("claude client is nil")
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

	result, err := a.claude.Run(ctx, analysisPrompt, a.model)
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
	// Try to find JSON in the output
	output = strings.TrimSpace(output)

	// Look for JSON object
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")

	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found in output")
	}

	jsonStr := output[start : end+1]

	var analysis Analysis
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	// Validate category
	switch analysis.Category {
	case CategorySyntax, CategoryLogic, CategoryEnvironment,
		CategoryUnclearSpec, CategoryMissingContext, CategoryTestFlake,
		CategoryTaskTooComplex:
		// Valid
	default:
		analysis.Category = CategoryLogic // Default
	}

	return &analysis, nil
}
