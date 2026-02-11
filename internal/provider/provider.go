package provider

import (
	"context"
	"io"
	"time"
)

// Tier constants for abstract model tiers
const (
	TierHigh   = "high"
	TierMedium = "medium"
	TierLow    = "low"
)

// Result represents the outcome of a provider invocation
type Result struct {
	Success  bool
	Output   string
	ExitCode int
	Duration time.Duration
	Model    string
}

// ToolEvent represents a tool call event from the provider
type ToolEvent struct {
	ToolName  string
	FilePath  string
	Timestamp time.Time
}

// EventHandler is called for each line of streaming JSON output
type EventHandler func(line []byte)

// ToolCallHandler is called when a tool call event is detected
type ToolCallHandler func(event ToolEvent)

// Provider executes LLM invocations via a CLI tool
type Provider interface {
	Name() string
	Run(ctx context.Context, prompt string, tier string) (*Result, error)
	StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
		handler EventHandler, onToolCall ToolCallHandler) (*Result, error)
	RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error)
	IsUsageLimitError(result *Result, err error) bool
}

// TierFromLegacyModel maps known model names to abstract tier constants.
// Known mappings:
//   - opus → high
//   - sonnet → medium
//   - haiku → low
//   - o3 → high
//   - gpt-4o → medium
//   - gpt-4o-mini → low
//
// Unrecognized model names are returned unchanged for forward compatibility.
// Matching is case-insensitive for known models.
func TierFromLegacyModel(modelName string) string {
	// Normalize to lowercase for case-insensitive matching
	normalized := toLower(modelName)

	// Claude models
	if normalized == "opus" {
		return TierHigh
	}
	if normalized == "sonnet" {
		return TierMedium
	}
	if normalized == "haiku" {
		return TierLow
	}

	// OpenAI models
	if normalized == "o3" {
		return TierHigh
	}
	if normalized == "gpt-4o" {
		return TierMedium
	}
	if normalized == "gpt-4o-mini" {
		return TierLow
	}

	// Pass through unrecognized names unchanged
	return modelName
}

// toLower converts a string to lowercase for case-insensitive comparison
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + ('a' - 'A')
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// Compile-time interface satisfaction checks
// These will be implemented by concrete provider types (ClaudeProvider, CodexProvider, etc.)
// and verified in their respective files
