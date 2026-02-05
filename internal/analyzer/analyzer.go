package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/claude"
)

// Category represents the type of failure
type Category string

const (
	CategorySyntax         Category = "syntax"          // Typo, missing import, wrong API
	CategoryLogic          Category = "logic"           // Algorithm wrong, edge case missed
	CategoryEnvironment    Category = "environment"     // Wrong tool version, missing dep
	CategoryUnclearSpec    Category = "unclear_spec"    // Spec is ambiguous
	CategoryMissingContext Category = "missing_context" // Didn't know about existing code
	CategoryTestFlake      Category = "test_flake"      // Non-deterministic test failure
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
	claude *claude.Client
	model  string
}

// NewAnalyzer creates a new analyzer
func NewAnalyzer(claudeClient *claude.Client, model string) *Analyzer {
	return &Analyzer{
		claude: claudeClient,
		model:  model,
	}
}

// Analyze analyzes a failure and returns structured insights
func (a *Analyzer) Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*Analysis, error) {
	prompt := buildAnalysisPrompt(b, failureOutput)

	result, err := a.claude.Run(ctx, prompt, a.model)
	if err != nil {
		return nil, fmt.Errorf("running analysis: %w", err)
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

func buildAnalysisPrompt(b *bead.Bead, failureOutput string) string {
	// Truncate failure output if too long
	maxLen := 8000
	if len(failureOutput) > maxLen {
		failureOutput = failureOutput[:maxLen] + "\n\n[... truncated ...]"
	}

	return fmt.Sprintf(`# Failure Analysis

A task just failed. Analyze what went wrong and extract any learnings.

## Task

**ID:** %s
**Title:** %s

%s

## Error Output

%s

## Your Job

1. **Categorize** the failure:
   - syntax: Typo, missing import, wrong API usage
   - logic: Algorithm wrong, edge case missed
   - environment: Wrong tool version, missing dependency, config issue
   - unclear_spec: The specification is ambiguous or contradictory
   - missing_context: Didn't know about existing code/patterns in the codebase
   - test_flake: Non-deterministic test failure (timing, random, external)

2. **Determine if recoverable** without escalating to a stronger model:
   - true: Can fix with more context or a simple retry
   - false: Needs deeper reasoning or human intervention

3. **Extract a learning** if this insight would help future tasks:
   - Should be generalizable (not specific to this one task)
   - Should be actionable (tells what to do or avoid)
   - Should be concise (1-2 sentences)
   - Set to null if no generalizable learning

4. **Suggest** what to try next

## Output Format

Respond with ONLY a JSON object (no markdown, no explanation):

{"category": "missing_context", "recoverable": true, "root_cause": "Brief description", "learning": "The insight or null", "suggestion": "What to try next"}
`, b.ID, b.Title, b.Description, failureOutput)
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
		CategoryUnclearSpec, CategoryMissingContext, CategoryTestFlake:
		// Valid
	default:
		analysis.Category = CategoryLogic // Default
	}

	return &analysis, nil
}
