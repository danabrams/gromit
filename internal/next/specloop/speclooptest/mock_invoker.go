package speclooptest

import (
	"context"

	"github.com/danabrams/gromit/internal/provider"
)

// MockInvoker captures the prompt and returns a configured result.
type MockInvoker struct {
	CapturedPrompt  string
	CapturedDir     string
	UsedInvokeInDir bool
	Result          *provider.Result
	Err             error
}

func (m *MockInvoker) Invoke(_ context.Context, prompt string) (*provider.Result, error) {
	m.CapturedPrompt = prompt
	return m.Result, m.Err
}

func (m *MockInvoker) InvokeInDir(_ context.Context, prompt string, dir string) (*provider.Result, error) {
	m.CapturedPrompt = prompt
	m.CapturedDir = dir
	m.UsedInvokeInDir = true
	return m.Result, m.Err
}
