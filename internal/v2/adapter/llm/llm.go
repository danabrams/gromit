package llm

import (
	"context"
	"io"
	"time"
)

// InvokeRequest carries the prompt and model for a non-streaming invocation.
type InvokeRequest struct {
	Prompt string
	Model  string
}

// StreamInvokeRequest carries the prompt, model, and output writer for streaming invocations.
type StreamInvokeRequest struct {
	Prompt string
	Model  string
	Output io.Writer
}

// LLMResponse summarizes the result of a Claude invocation.
type LLMResponse struct {
	Success  bool
	Output   string
	Tokens   int
	CostUSD  float64
	Duration time.Duration
}

// LLMProvider executes Claude invocations.
type LLMProvider interface {
	Invoke(ctx context.Context, req InvokeRequest) (*LLMResponse, error)
	StreamInvoke(ctx context.Context, req StreamInvokeRequest) (*LLMResponse, error)
}
