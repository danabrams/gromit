package review

import (
	"context"

	"github.com/danabrams/gromit/internal/provider"
)

// mockInvoker satisfies llmadapter.Invoker for testing.
type mockInvoker struct {
	result           *provider.Result
	err              error
	calledWithPrompt string
	calledWithDir    string
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	m.calledWithPrompt = prompt
	return m.result, m.err
}

func (m *mockInvoker) InvokeInDir(ctx context.Context, prompt string, dir string) (*provider.Result, error) {
	m.calledWithDir = dir
	return m.Invoke(ctx, prompt)
}
