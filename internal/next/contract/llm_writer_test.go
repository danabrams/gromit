package contract

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// stubInvoker is a test double for llmadapter.Invoker.
type stubInvoker struct {
	output string
	err    error
}

func (s *stubInvoker) Invoke(_ context.Context, _ string) (*provider.Result, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &provider.Result{Output: s.output}, nil
}

func (s *stubInvoker) InvokeInDir(_ context.Context, _ string, _ string) (*provider.Result, error) {
	return s.Invoke(context.Background(), "")
}

// nilResultInvoker returns nil result without error.
type nilResultInvoker struct{}

func (n *nilResultInvoker) Invoke(_ context.Context, _ string) (*provider.Result, error) {
	return nil, nil
}

func (n *nilResultInvoker) InvokeInDir(_ context.Context, _ string, _ string) (*provider.Result, error) {
	return nil, nil
}

// captureInvoker records the prompt passed to Invoke for assertion in tests.
type captureInvoker struct {
	capturedPrompt string
	output         string
}

func (c *captureInvoker) Invoke(_ context.Context, prompt string) (*provider.Result, error) {
	c.capturedPrompt = prompt
	return &provider.Result{Output: c.output}, nil
}

func (c *captureInvoker) InvokeInDir(_ context.Context, prompt string, _ string) (*provider.Result, error) {
	return c.Invoke(context.Background(), prompt)
}

// TestLLMContractWriter_Success verifies that WriteContracts returns a non-nil
// *ScenarioContract pointer with correct data on success.
func TestLLMContractWriter_Success(t *testing.T) {
	yamlOutput := `scenarios:
  - name: add-works
    assertions:
      - file_exists: calc/calc.go`
	inv := &stubInvoker{output: yamlOutput}
	w := NewLLMContractWriter(inv)

	scenarios := []SpecScenario{
		{Name: "add-works", When: "add is called", Then: "result is 3"},
	}
	result, err := w.WriteContracts(context.Background(), scenarios, "spec packet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil *ScenarioContract")
	}
	if len(result.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(result.Scenarios))
	}
	if result.Scenarios[0].Name != "add-works" {
		t.Fatalf("expected scenario name 'add-works', got %q", result.Scenarios[0].Name)
	}
}

// TestLLMContractWriter_InvokerError verifies that WriteContracts returns nil pointer
// and an error when the invoker fails.
func TestLLMContractWriter_InvokerError(t *testing.T) {
	inv := &stubInvoker{err: errors.New("llm unavailable")}
	w := NewLLMContractWriter(inv)

	result, err := w.WriteContracts(context.Background(), []SpecScenario{}, "packet")
	if err == nil {
		t.Fatal("expected error when invoker fails")
	}
	if result != nil {
		t.Fatalf("expected nil pointer on error, got %+v", result)
	}
}

// TestLLMContractWriter_NilResult verifies that WriteContracts returns nil pointer
// and an error when the invoker returns a nil result without error.
func TestLLMContractWriter_NilResult(t *testing.T) {
	w := NewLLMContractWriter(&nilResultInvoker{})

	result, err := w.WriteContracts(context.Background(), []SpecScenario{}, "packet")
	if err == nil {
		t.Fatal("expected error for nil invoker result")
	}
	if result != nil {
		t.Fatalf("expected nil pointer on nil invoker result, got %+v", result)
	}
}

// TestLLMContractWriter_YAMLFence verifies that YAML fences in LLM output are stripped.
func TestLLMContractWriter_YAMLFence(t *testing.T) {
	yamlOutput := "Here you go:\n```yaml\nscenarios:\n  - name: test\n    assertions:\n      - file_exists: main.go\n```\n"
	inv := &stubInvoker{output: yamlOutput}
	w := NewLLMContractWriter(inv)

	scenarios := []SpecScenario{
		{Name: "test", Then: "main.go exists"},
	}
	result, err := w.WriteContracts(context.Background(), scenarios, "packet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil *ScenarioContract")
	}
	if len(result.Scenarios) != 1 || result.Scenarios[0].Name != "test" {
		t.Fatalf("expected 1 scenario named 'test', got %v", result.Scenarios)
	}
}

// TestLLMContractWriter_SpecPacketIncludedInPrompt verifies that specPacket
// content is included in the rendered prompt.
func TestLLMContractWriter_SpecPacketIncludedInPrompt(t *testing.T) {
	yamlOutput := `scenarios:
  - name: add-works
    assertions:
      - file_exists: calc/calc.go`
	inv := &captureInvoker{output: yamlOutput}
	w := NewLLMContractWriter(inv)

	specPacket := "# Prior Validation Errors\nfile_exists: calc/calc.go failed"
	scenarios := []SpecScenario{
		{Name: "add-works", When: "add is called", Then: "result is 3"},
	}
	_, err := w.WriteContracts(context.Background(), scenarios, specPacket)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(inv.capturedPrompt, specPacket) {
		t.Errorf("expected prompt to contain specPacket;\ngot: %q", inv.capturedPrompt)
	}
}

// TestLLMContractWriter_SameInputsSamePrompt verifies that the same inputs
// produce the same prompt.
func TestLLMContractWriter_SameInputsSamePrompt(t *testing.T) {
	yamlOutput := `scenarios:
  - name: add-works
    assertions:
      - file_exists: calc/calc.go`
	inv1 := &captureInvoker{output: yamlOutput}
	inv2 := &captureInvoker{output: yamlOutput}
	w1 := NewLLMContractWriter(inv1)
	w2 := NewLLMContractWriter(inv2)

	scenarios := []SpecScenario{{Name: "add-works", When: "add is called", Then: "result is 3"}}
	specPacket := "spec packet"
	_, _ = w1.WriteContracts(context.Background(), scenarios, specPacket)
	_, _ = w2.WriteContracts(context.Background(), scenarios, specPacket)

	if inv1.capturedPrompt != inv2.capturedPrompt {
		t.Error("same inputs should produce the same prompt")
	}
}
