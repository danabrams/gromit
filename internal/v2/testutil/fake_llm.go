package testutil

import (
    "context"
    "fmt"
    "io"
    "strings"
    "sync"

    "github.com/danabrams/gromit/internal/v2/adapter/llm"
)

type responseEntry struct {
    key  string
    resp *llm.LLMResponse
}

// FakeLLM simulates an llm.LLMProvider by returning canned responses.
type FakeLLM struct {
    mu          sync.Mutex
    responses   []responseEntry
    Calls       []llm.InvokeRequest
    StreamCalls []llm.StreamInvokeRequest
}

// NewFakeLLM constructs an empty FakeLLM that can be configured with responses.
func NewFakeLLM() *FakeLLM {
    return &FakeLLM{}
}

// SetResponse associates the provided response with the key that must appear in the prompt.
// An empty key matches any prompt, so it can serve as a default response.
func (f *FakeLLM) SetResponse(key string, resp *llm.LLMResponse) {
    if resp == nil {
        return
    }
    f.mu.Lock()
    defer f.mu.Unlock()
    f.responses = append(f.responses, responseEntry{key: key, resp: resp})
}

// Invoke records the request and returns the first response whose key is contained in the prompt.
func (f *FakeLLM) Invoke(_ context.Context, req llm.InvokeRequest) (*llm.LLMResponse, error) {
    f.mu.Lock()
    defer f.mu.Unlock()

    resp, err := f.firstMatchingResponse(req.Prompt)
    if err != nil {
        return nil, err
    }
    f.Calls = append(f.Calls, req)
    return resp, nil
}

// StreamInvoke records the streaming invocation and writes the response output to the provided writer.
func (f *FakeLLM) StreamInvoke(_ context.Context, req llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
    f.mu.Lock()
    defer f.mu.Unlock()

    resp, err := f.firstMatchingResponse(req.Prompt)
    if err != nil {
        return nil, err
    }
    f.StreamCalls = append(f.StreamCalls, req)

    if req.Output != nil && resp.Output != "" {
        _, _ = io.WriteString(req.Output, resp.Output)
    }

    return resp, nil
}

func (f *FakeLLM) firstMatchingResponse(prompt string) (*llm.LLMResponse, error) {
    for _, entry := range f.responses {
        if entry.key == "" || strings.Contains(prompt, entry.key) {
            return entry.resp, nil
        }
    }
    return nil, fmt.Errorf("no fake response for prompt %q", prompt)
}
