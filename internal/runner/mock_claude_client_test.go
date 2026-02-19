package runner

import (
	"context"
	"io"
	"sync"

	"github.com/danabrams/gromit/internal/claude"
)

// mockClaudeClient is a test helper for adapters that still build routers from a
// claude.Client-compatible interface.
type mockClaudeClient struct {
	RunFn           func(ctx context.Context, prompt string, model string) (*claude.Result, error)
	StreamRunFn     func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error)
	RunValidationFn func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error)

	mu              sync.Mutex
	RunCalls        []mockClaudeCall
	StreamRunCalls  []mockClaudeCall
	ValidationCalls int
}

type mockClaudeCall struct {
	Prompt string
	Model  string
}

func (m *mockClaudeClient) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	m.mu.Lock()
	m.RunCalls = append(m.RunCalls, mockClaudeCall{Prompt: prompt, Model: model})
	m.mu.Unlock()
	if m.RunFn != nil {
		return m.RunFn(ctx, prompt, model)
	}
	return &claude.Result{Success: true, Output: "ok"}, nil
}

func (m *mockClaudeClient) StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
	m.mu.Lock()
	m.StreamRunCalls = append(m.StreamRunCalls, mockClaudeCall{Prompt: prompt, Model: model})
	m.mu.Unlock()
	if m.StreamRunFn != nil {
		return m.StreamRunFn(ctx, prompt, model, output, handler, onToolCall)
	}
	return &claude.Result{Success: true, Output: "ok"}, nil
}

func (m *mockClaudeClient) RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
	m.mu.Lock()
	m.ValidationCalls++
	m.mu.Unlock()
	if m.RunValidationFn != nil {
		return m.RunValidationFn(ctx, commands, model, workDir)
	}
	return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
}
