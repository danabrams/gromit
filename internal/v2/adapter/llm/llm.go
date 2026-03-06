package llm

import (
	"context"
	"io"
	"time"
)

// LLMInvokeRequest carries the prompt and model for a non-streaming invocation.
type LLMInvokeRequest struct {
	Prompt   string
	Model    string
	Metadata map[string]string
}

// InvokeRequest is retained for backwards compatibility with existing callers.
type InvokeRequest = LLMInvokeRequest

// LLMStreamInvokeRequest carries the prompt, model, and output writer for streaming invocations.
type LLMStreamInvokeRequest struct {
	Prompt   string
	Model    string
	Output   io.Writer
	Metadata map[string]string
}

// StreamInvokeRequest is retained for backwards compatibility with existing callers.
type StreamInvokeRequest = LLMStreamInvokeRequest

// LLMInvokeResponse summarizes the result of a Claude invocation.
type LLMInvokeResponse struct {
	Success  bool
	Output   string
	Tokens   int
	CostUSD  float64
	Duration time.Duration
	Metadata map[string]string
}

// LLMResponse is retained for backwards compatibility with existing callers.
type LLMResponse = LLMInvokeResponse

// LLMStreamInvokeResponse describes the streaming output and mirrors the invoke response structure.
type LLMStreamInvokeResponse = LLMInvokeResponse

// StreamInvokeResponse is retained for backwards compatibility with existing callers.
type StreamInvokeResponse = LLMStreamInvokeResponse

// LLMProvider executes Claude invocations.
type LLMProvider interface {
	Invoke(ctx context.Context, req LLMInvokeRequest) (*LLMInvokeResponse, error)
	StreamInvoke(ctx context.Context, req LLMStreamInvokeRequest) (*LLMStreamInvokeResponse, error)
}
