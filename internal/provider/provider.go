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

// Compile-time interface satisfaction checks
// These will be implemented by concrete provider types (ClaudeProvider, CodexProvider, etc.)
// and verified in their respective files
