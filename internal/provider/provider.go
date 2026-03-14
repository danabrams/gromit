package provider

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

// Tier constants for abstract model tiers
const (
	TierXHigh  = "xhigh"
	TierHigh   = "high"
	TierMedium = "medium"
	TierLow    = "low"
)

const (
	FailureCategoryNone                = ""
	FailureCategoryTransportDisconnect = "transport_disconnect"
	FailureCategoryRateLimited         = "rate_limited"
	FailureCategoryAuth                = "auth"
	FailureCategoryStartupError        = "startup_error"
	FailureCategoryOther               = "other"
)

var legacyModelToTier = map[string]string{
	// Claude models
	"opus":   TierHigh,
	"sonnet": TierMedium,
	"haiku":  TierLow,
	// OpenAI models
	"o3":          TierHigh,
	"gpt-4o":      TierMedium,
	"gpt-4o-mini": TierLow,
	// Codex models
	"gpt-5.3-codex":      TierMedium,
	"gpt-5.1-codex-mini": TierLow,
	// Gemini models
	"gemini-3.1-pro": TierHigh,
	"gemini-3-pro":   TierHigh,
	"gemini-3-flash": TierMedium,
}

var tierToLegacyModel = map[string]string{
	TierXHigh:  "opus",
	TierHigh:   "opus",
	TierMedium: "sonnet",
	TierLow:    "haiku",
}

// modelToProvider maps known model names to their owning provider.
// This enforces the known attribution of models to providers for usage accounting.
var modelToProvider = map[string]string{
	// Claude models
	"opus":   "claude",
	"sonnet": "claude",
	"haiku":  "claude",
	// Codex models
	"gpt-5.3-codex":      "codex",
	"gpt-5.1-codex-mini": "codex",
}

// ErrStreamNotSupported signals that a provider cannot fulfill streaming requests.
var ErrStreamNotSupported = errors.New("stream run not supported")

// Result represents the outcome of a provider invocation
type Result struct {
	Success                 bool          `json:"success"`
	Output                  string        `json:"output"`
	Stderr                  string        `json:"stderr"`
	Diagnostics             string        `json:"diagnostics"`
	FailureCategory         string        `json:"failure_category"`
	ExitCode                int           `json:"exit_code"`
	Duration                time.Duration `json:"duration"`
	Model                   string        `json:"model"`
	ReasoningEffort         string        `json:"reasoning_effort,omitempty"`
	CostUSD                 float64       `json:"cost_usd"`
	InputTokens             int           `json:"input_tokens"`
	CachedInputTokens       int           `json:"cached_input_tokens"`
	OutputTokens            int           `json:"output_tokens"`
	CacheHit                bool          `json:"cache_hit,omitempty"`
	CacheMiss               bool          `json:"cache_miss,omitempty"`
	CacheWrite              bool          `json:"cache_write,omitempty"`
	CacheClass              string        `json:"cache_class,omitempty"`
	CacheKey                string        `json:"cache_key,omitempty"`
	CacheInvalidationReason string        `json:"cache_invalidation_reason,omitempty"`
	CacheVersionMarker      string        `json:"cache_version_marker,omitempty"`
}

// ToolEvent represents a tool call event from the provider
type ToolEvent struct {
	ToolName  string    `json:"tool_name"`
	FilePath  string    `json:"file_path"`
	Timestamp time.Time `json:"timestamp"`
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

// ProviderCacheHooks is an optional extension that exposes provider-level cache
// adapter capability without forcing every Provider implementation to support it.
type ProviderCacheHooks interface {
	CacheAdapter() CacheAdapter
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
//   - gpt-5.1-codex-mini → low
//
// Unrecognized model names are returned unchanged for forward compatibility.
// Case-insensitive matching allows for flexible config formats (e.g., "Opus" or "OPUS").
func TierFromLegacyModel(modelName string) string {
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
	if model, ok := tierToLegacyModel[tier]; ok {
		return model
	}

	// Pass through unrecognized tiers unchanged
	return tier
}

// ValidateModelProviderAttribution validates that a model name belongs to the specified provider.
// This enforces known attribution mappings for usage accounting data quality.
// Returns (true, "") for valid attribution, (false, reason) for invalid attribution.
// Both model and provider names are matched case-insensitively.
// Empty model or provider values are treated as invalid.
func ValidateModelProviderAttribution(model, provider string) (valid bool, reason string) {
	// Check for empty values
	modelLower := strings.TrimSpace(strings.ToLower(model))
	providerLower := strings.TrimSpace(strings.ToLower(provider))

	if modelLower == "" {
		return false, "empty model name"
	}
	if providerLower == "" {
		return false, "empty provider name"
	}

	// Look up expected provider for this model
	expectedProvider, isKnownModel := modelToProvider[modelLower]
	if !isKnownModel {
		return false, "model not in known attribution mapping: " + model
	}

	// Verify the provider matches
	if providerLower != expectedProvider {
		return false, "model " + model + " belongs to " + expectedProvider + " not " + provider
	}

	return true, ""
}

// DirStreamRunner is an optional extension interface for providers that support
// streaming invocations with a working directory. Providers that implement this
// can run the LLM process in a specific directory (e.g., a worktree).
type DirStreamRunner interface {
	StreamRunInDir(ctx context.Context, prompt string, tier string, dir string, output io.Writer,
		handler EventHandler, onToolCall ToolCallHandler) (*Result, error)
}

// Compile-time interface satisfaction checks
// These will be implemented by concrete provider types (ClaudeProvider, CodexProvider, etc.)
// and verified in their respective files
