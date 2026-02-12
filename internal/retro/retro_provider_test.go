package retro

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// TestNewRetro_AcceptsProvider verifies that NewRetro can accept a Provider for learnings filtering and analysis
func TestNewRetro_AcceptsProvider(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock provider
	mockProvider := &mockProvider{}

	// NewRetro should accept a provider parameter
	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	if r == nil {
		t.Fatal("expected non-nil Retro")
	}

	if r.provider != mockProvider {
		t.Error("expected Retro to store the provider")
	}
}

// mockProvider implements provider.Provider for testing
type mockProvider struct {
	runCalled bool
	runResult *provider.Result
	runErr    error
}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	m.runCalled = true
	if m.runErr != nil {
		return nil, m.runErr
	}
	if m.runResult != nil {
		return m.runResult, nil
	}
	return &provider.Result{Success: true, Output: `{"category":"logic","recoverable":true,"root_cause":"test","suggestion":"test"}`}, nil
}

func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output interface{}, handler interface{}, onToolCall interface{}) (*provider.Result, error) {
	return m.Run(ctx, prompt, tier)
}

func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return m.Run(ctx, "", tier)
}

func (m *mockProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
