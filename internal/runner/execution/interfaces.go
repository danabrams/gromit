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
