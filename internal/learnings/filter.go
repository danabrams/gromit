package learnings

import (
	"context"
	"fmt"
	"strings"
)

// ClaudeRunner is an interface for invoking Claude with a prompt.
// This enables testing without calling the actual Claude CLI.
type ClaudeRunner interface {
	Run(ctx context.Context, prompt string, model string) (*Result, error)
}

// Result represents the outcome of a Claude invocation (local alias to avoid import cycles).
// In production, this will be satisfied by claude.Result from the claude package.
type Result struct {
	Success bool
	Output  string
}

// NewLLMFilter creates a FilterFunc that uses an LLM (haiku) to classify learnings
// as generic engineering advice or project-specific patterns.
//
// The filter sends the learning content to Claude with a prompt asking it to classify
// the learning as "specific" (project-specific) or "generic" (generic engineering advice).
// If classified as generic, the FilterFunc returns true (should be filtered).
// If classified as specific, it returns false (should not be filtered).
//
// Parameters:
// - runner: ClaudeRunner interface for invoking Claude (injected for testability)
// - projectName: Name of the project (used in the classification prompt)
// - projectDesc: Brief description of what qualifies as project-specific for this project
//
// Returns a FilterFunc that can be passed to File.SetFilter().
func NewLLMFilter(runner ClaudeRunner, projectName, projectDesc string) FilterFunc {
	return func(content string) (bool, error) {
		if runner == nil {
			return false, fmt.Errorf("claude runner is nil")
		}

		// Build the classification prompt
		prompt := buildClassificationPrompt(content, projectName, projectDesc)

		// Call Claude with haiku for cost efficiency
		result, err := runner.Run(context.Background(), prompt, "haiku")
		if err != nil {
			return false, fmt.Errorf("calling claude: %w", err)
		}

		// Parse the result
		classification := parseClassification(result.Output)

		// Return true if generic (should be filtered), false if specific
		return classification == "generic", nil
	}
}

// buildClassificationPrompt constructs the prompt for classifying a learning.
func buildClassificationPrompt(content, projectName, projectDesc string) string {
	return fmt.Sprintf(`You are classifying a learning entry for the %s project.

Project context: %s

Learning to classify:
"""
%s
"""

Classify this learning as either "specific" or "generic":

- **specific**: The learning references project-specific patterns, files, packages, conventions, bead IDs, specific error patterns, or behaviors unique to this codebase. Examples:
  - "The runner's escalation chain skips haiku when the bead has complexity:high label"
  - "Use bead.Ready() for actionable beads, not bead.List() which returns all open beads"
  - "Shell scripts in internal/prompt must use quoted <<'EOF' heredocs to prevent variable expansion"

- **generic**: The learning restates universally-known engineering principles that apply to any software project. Examples:
  - "Always verify tests pass before marking a task complete"
  - "Follow DRY principles to avoid code duplication"
  - "Use single responsibility principle for functions"
  - "Handle errors properly in Go"

Output only one word: "specific" or "generic".
`, projectName, projectDesc, content)
}

// parseClassification extracts the classification from Claude's output.
// Returns "specific", "generic", or "unknown" if parsing fails.
func parseClassification(output string) string {
	// Normalize output: lowercase and trim whitespace
	normalized := strings.ToLower(strings.TrimSpace(output))

	// Check for exact single-word matches first (most reliable)
	if normalized == "specific" {
		return "specific"
	}
	if normalized == "generic" {
		return "generic"
	}

	// Check for the classification keywords in the output
	hasSpecific := strings.Contains(normalized, "specific")
	hasGeneric := strings.Contains(normalized, "generic")

	// If both appear or neither appears, it's ambiguous
	if (hasSpecific && hasGeneric) || (!hasSpecific && !hasGeneric) {
		return "unknown"
	}

	// Only one keyword present
	if hasSpecific {
		return "specific"
	}
	if hasGeneric {
		return "generic"
	}

	return "unknown"
}
