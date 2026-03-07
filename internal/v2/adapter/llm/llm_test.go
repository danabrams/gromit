package llm

import (
	"context"
	"io"
	"testing"
)

type llmTestStub struct{}

func (llmTestStub) Invoke(ctx context.Context, req InvokeRequest) (*LLMResponse, error) {
	return &LLMResponse{Success: true, Output: req.Prompt}, nil
}

func (llmTestStub) StreamInvoke(ctx context.Context, req StreamInvokeRequest) (*LLMResponse, error) {
	if req.Output != nil {
		io.WriteString(req.Output, "stream")
	}
	return &LLMResponse{Success: true}, nil
}

func TestLLMProviderInterface(t *testing.T) {
	var _ LLMProvider = llmTestStub{}
}
