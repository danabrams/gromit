package execution

import (
	"context"
	"io"

	"github.com/danabrams/gromit/internal/provider"
)

// Router selects a provider and model for a given phase and tier.
// This is a narrow interface — the full provider.Router has additional methods.
type Router interface {
	Select(phase, tier string) (Provider, string)
	MarkUnavailable(name string)
	RecordOutcome(providerName, failureCategory string)
}

// Provider executes a single LLM invocation with streaming.
// This is a narrow interface — the full provider.Provider has additional methods.
type Provider interface {
	Name() string
	StreamRun(ctx context.Context, prompt, tier string, output io.Writer,
		handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	IsUsageLimitError(result *provider.Result, err error) bool
}

// OverwriteWriter extends io.Writer with in-place terminal updates.
type OverwriteWriter interface {
	io.Writer
	WriteOverwrite(p []byte) (int, error)
}

type noopRouter struct{}

func (*noopRouter) Select(phase, tier string) (Provider, string) {
	return &noopProvider{}, ""
}

func (*noopRouter) MarkUnavailable(name string) {}

func (*noopRouter) RecordOutcome(providerName, failureCategory string) {}

type noopProvider struct{}

func (*noopProvider) Name() string {
	return "noop"
}

func (*noopProvider) StreamRun(
	ctx context.Context,
	prompt, tier string,
	output io.Writer,
	handler provider.EventHandler,
	onToolCall provider.ToolCallHandler,
) (*provider.Result, error) {
	return &provider.Result{}, nil
}

func (*noopProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

type noopOverwriteWriter struct{}

func (*noopOverwriteWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (*noopOverwriteWriter) WriteOverwrite(p []byte) (int, error) {
	return len(p), nil
}

var _ Router = (*noopRouter)(nil)
var _ Provider = (*noopProvider)(nil)
var _ OverwriteWriter = (*noopOverwriteWriter)(nil)
