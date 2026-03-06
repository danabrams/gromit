package llm

import (
	"context"
	"testing"
)

func TestLLMProviderContract(t *testing.T) {
	var _ interface {
		Invoke(context.Context, LLMInvokeRequest) (*LLMInvokeResponse, error)
		StreamInvoke(context.Context, LLMStreamInvokeRequest) (*LLMStreamInvokeResponse, error)
	} = (LLMProvider)(nil)
}
