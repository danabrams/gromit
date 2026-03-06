package testutil

import (
    "context"
    "testing"

    "github.com/danabrams/gromit/internal/v2/adapter/llm"
)

func TestFakeLLM_InvokeReturnsConfiguredResponse(t *testing.T) {
    t.Parallel()

    fake := NewFakeLLM()
    fake.SetResponse("plan", &llm.LLMResponse{Success: true, Output: "calculated plan", Tokens: 10})

    req := llm.InvokeRequest{Prompt: "create plan", Model: "claude"}
    resp, err := fake.Invoke(context.Background(), req)
    if err != nil {
        t.Fatalf("Invoke() returned error: %v", err)
    }

    if resp == nil {
        t.Fatal("Invoke() returned nil response")
    }
    if resp.Output != "calculated plan" {
        t.Fatalf("unexpected output: %q", resp.Output)
    }

    if len(fake.Calls) != 1 {
        t.Fatalf("expected 1 recorded call, got %d", len(fake.Calls))
    }
    if fake.Calls[0].Prompt != req.Prompt {
        t.Fatalf("recorded call prompt mismatch: %q", fake.Calls[0].Prompt)
    }
}
