package contract

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// ContractWriter translates spec scenarios into declarative contract assertions.
// The LLM-backed implementation receives spec scenarios and produces a ScenarioContract.
type ContractWriter interface {
	WriteContracts(ctx context.Context, scenarios []SpecScenario, specPacket string) (*ScenarioContract, error)
}

// LLMContractWriter implements ContractWriter using an LLM invoker.
// It constructs a prompt from spec scenarios and packet, invokes the LLM,
// and parses the YAML response into a ScenarioContract. Uses Sonnet (P1) model tier.
type LLMContractWriter struct {
	invoker llmadapter.Invoker
}

// NewLLMContractWriter creates an LLMContractWriter backed by the given invoker.
func NewLLMContractWriter(invoker llmadapter.Invoker) *LLMContractWriter {
	return &LLMContractWriter{invoker: invoker}
}

// WriteContracts implements ContractWriter, translating spec scenarios into
// declarative contract assertions via LLM.
func (w *LLMContractWriter) WriteContracts(ctx context.Context, scenarios []SpecScenario, specPacket string) (*ScenarioContract, error) {
	prompt, err := RenderContractPrompt(ContractPromptInput{
		SpecPacket: specPacket,
		Scenarios:  scenarios,
	})
	if err != nil {
		return nil, fmt.Errorf("render contract prompt: %w", err)
	}

	result, err := w.invoker.Invoke(ctx, prompt)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("contract writer: provider returned nil result")
	}

	c, err := ParseContractYAML(result.Output)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
