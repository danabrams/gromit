package provider

import (
	"context"
	"io"
	"strings"
	"time"
)

// Tier constants for abstract model tiers
const (
	TierHigh   = "high"
	TierMedium = "medium"
	TierLow    = "low"
)

const (
	FailureCategoryNone                = ""
	FailureCategoryTransportDisconnect = "transport_disconnect"
	FailureCategoryRateLimited         = "rate_limited"
	FailureCategoryAuth                = "auth"
	FailureCategoryOther               = "other"
)

// Result represents the outcome of a provider invocation
type Result struct {
	Success           bool
	Output            string
	Stderr            string
	Diagnostics       string
	FailureCategory   string
	ExitCode          int
	Duration          time.Duration
	Model             string
	CostUSD           float64
	InputTokens       int
	CachedInputTokens int
	OutputTokens      int
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
	ModelForTier(tier string) string
	Run(ctx context.Context, prompt string, tier string) (*Result, error)
	StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
		handler EventHandler, onToolCall ToolCallHandler) (*Result, error)
	RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error)
	IsUsageLimitError(result *Result, err error) bool
	IsValidationPassed(result *Result) bool
	IsScopeTooLarge(result *Result) (bool, string)
}

// TierFromLegacyModel maps known model names to abstract tier constants.
// Known mappings:
//   - opus → high
//   - sonnet → medium
//   - haiku → low
//   - o3 → high
//   - gpt-4o → medium
//   - gpt-4o-mini → low
//   - gpt-5.3-codex → medium
//   - gpt-5.3-codex-spark → low
//
// Unrecognized model names are returned unchanged for forward compatibility.
// Case-insensitive matching allows for flexible config formats (e.g., "Opus" or "OPUS").
func TierFromLegacyModel(modelName string) string {
	// Map of known model names (lowercase) to tiers
	legacyModelToTier := map[string]string{
		// Claude models
		"opus":   TierHigh,
		"sonnet": TierMedium,
		"haiku":  TierLow,
		// OpenAI models
		"o3":          TierHigh,
		"gpt-4o":      TierMedium,
		"gpt-4o-mini": TierLow,
		// Codex models
		"gpt-5.3-codex":       TierMedium,
		"gpt-5.3-codex-spark": TierLow,
	}

	// Check for known model (case-insensitive)
	if tier, ok := legacyModelToTier[strings.ToLower(modelName)]; ok {
		return tier
	}

	// Pass through unrecognized names unchanged
	return modelName
}

// TierToLegacyModel maps abstract tier constants to Claude legacy model names.
// This is used for backward compatibility and display purposes.
// Returns "opus" for high, "sonnet" for medium, "haiku" for low.
// Unrecognized tier values are passed through unchanged.
func TierToLegacyModel(tier string) string {
	tierToLegacyModel := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	if model, ok := tierToLegacyModel[tier]; ok {
		return model
	}

	// Pass through unrecognized tiers unchanged
	return tier
}

// Compile-time interface satisfaction checks
// These will be implemented by concrete provider types (ClaudeProvider, CodexProvider, etc.)
// and verified in their respective files
