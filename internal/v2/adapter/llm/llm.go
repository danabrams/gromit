package llm

import "github.com/danabrams/gromit/internal/v2/llmtypes"

// Type aliases — these are identical to the llmtypes originals.
type LLMInvokeRequest = llmtypes.LLMInvokeRequest
type LLMStreamInvokeRequest = llmtypes.LLMStreamInvokeRequest
type LLMInvokeResponse = llmtypes.LLMInvokeResponse
type LLMProvider = llmtypes.LLMProvider

// Backward-compatibility aliases retained for existing callers.
type InvokeRequest = llmtypes.LLMInvokeRequest
type StreamInvokeRequest = llmtypes.LLMStreamInvokeRequest
type LLMResponse = llmtypes.LLMInvokeResponse
type LLMStreamInvokeResponse = llmtypes.LLMInvokeResponse
type StreamInvokeResponse = llmtypes.LLMInvokeResponse
