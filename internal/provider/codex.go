package provider

import (
	"context"
	"io"
)

const (
	// providerNameCodex is the name identifier for the Codex provider
	providerNameCodex = "codex"
)

// CodexProvider wraps the Codex CLI and implements the Provider interface
type CodexProvider struct {
	binaryPath     string
	flags          []string
	promptDelivery string
	promptFlag     string
	tierToModel    map[string]string
}

// Compile-time check to verify CodexProvider implements Provider interface
var _ Provider = (*CodexProvider)(nil)

// NewCodexProvider creates a new CodexProvider with the given configuration
func NewCodexProvider(binaryPath string, flags []string, promptDelivery string, promptFlag string, tierToModel map[string]string) *CodexProvider {
	return &CodexProvider{
		binaryPath:     binaryPath,
		flags:          flags,
		promptDelivery: promptDelivery,
		promptFlag:     promptFlag,
		tierToModel:    tierToModel,
	}
}

// Name returns the provider name "codex"
func (cp *CodexProvider) Name() string {
	return providerNameCodex
}

// ModelForTier returns the model name for a given tier without invoking the LLM
func (cp *CodexProvider) ModelForTier(tier string) string {
	if modelName, ok := cp.tierToModel[tier]; ok {
		return modelName
	}
	return tier
}

// Run executes an LLM invocation with the given prompt and tier
func (cp *CodexProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	return nil, nil
}

// StreamRun executes an LLM invocation with streaming output
func (cp *CodexProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	return nil, nil
}

// RunValidation executes validation commands using the LLM
func (cp *CodexProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	return nil, nil
}

// IsUsageLimitError detects Codex-specific usage limit errors
func (cp *CodexProvider) IsUsageLimitError(result *Result, err error) bool {
	return false
}
