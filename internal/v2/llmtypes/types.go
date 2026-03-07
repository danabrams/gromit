package llmtypes

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

// LLMStreamInvokeRequest carries the prompt, model, and output writer for streaming invocations.
type LLMStreamInvokeRequest struct {
	Prompt   string
	Model    string
	Output   io.Writer
	Metadata map[string]string
}

// LLMInvokeResponse summarizes the result of a Claude invocation.
type LLMInvokeResponse struct {
	Success  bool
	Output   string
	Tokens   int
	CostUSD  float64
	Duration time.Duration
	Metadata map[string]string
}

// LLMProvider executes Claude invocations.
type LLMProvider interface {
	Invoke(ctx context.Context, req LLMInvokeRequest) (*LLMInvokeResponse, error)
	StreamInvoke(ctx context.Context, req LLMStreamInvokeRequest) (*LLMInvokeResponse, error)
}
