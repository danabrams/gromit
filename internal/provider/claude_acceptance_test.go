//go:build acceptance

package provider

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/claude"
)

// TestClaudeProviderInterfaceSatisfaction verifies that ClaudeProvider implements
// the Provider interface at compile time.
// Expected failure: ClaudeProvider type does not exist yet
func TestClaudeProviderInterfaceSatisfaction(t *testing.T) {
	var _ Provider = (*ClaudeProvider)(nil)
}

// TestNewClaudeProvider verifies that NewClaudeProvider constructor exists and
// creates a ClaudeProvider with the provided claude.Client and tier→model map.
// Expected failure: NewClaudeProvider function and ClaudeProvider type do not exist yet
func TestNewClaudeProvider(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	provider := NewClaudeProvider(client, tierMap)

	if provider == nil {
		t.Fatal("NewClaudeProvider returned nil")
	}
}

// TestClaudeProviderName verifies that ClaudeProvider.Name() returns "claude".
// Expected failure: ClaudeProvider type and Name() method do not exist yet
func TestClaudeProviderName(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	provider := NewClaudeProvider(client, tierMap)
	name := provider.Name()

	if name != "claude" {
		t.Errorf("Name() = %q, want %q", name, "claude")
	}
}

// TestClaudeProviderRunMapsHighTier verifies that Run() maps the "high" tier
// to the configured high-tier model name and delegates to claude.Client.Run().
// Expected failure: ClaudeProvider type and Run() method do not exist yet
func TestClaudeProviderRunMapsHighTier(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	prompt := "test prompt for high tier"

	result, err := provider.Run(ctx, prompt, TierHigh)

	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Verify the mock was called with the correct model
	if !client.runCalled {
		t.Error("claude.Client.Run() was not called")
	}

	if client.lastModel != "opus" {
		t.Errorf("Run() called client with model %q, want %q", client.lastModel, "opus")
	}

	if client.lastPrompt != prompt {
		t.Errorf("Run() called client with prompt %q, want %q", client.lastPrompt, prompt)
	}
}

// TestClaudeProviderRunMapsMediumTier verifies that Run() maps the "medium" tier
// to the configured medium-tier model name.
// Expected failure: ClaudeProvider type and Run() method do not exist yet
func TestClaudeProviderRunMapsMediumTier(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	_, err := provider.Run(ctx, "test", TierMedium)

	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if client.lastModel != "sonnet" {
		t.Errorf("Run() called client with model %q, want %q", client.lastModel, "sonnet")
	}
}

// TestClaudeProviderRunMapsLowTier verifies that Run() maps the "low" tier
// to the configured low-tier model name.
// Expected failure: ClaudeProvider type and Run() method do not exist yet
func TestClaudeProviderRunMapsLowTier(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	_, err := provider.Run(ctx, "test", TierLow)

	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if client.lastModel != "haiku" {
		t.Errorf("Run() called client with model %q, want %q", client.lastModel, "haiku")
	}
}

// TestClaudeProviderRunReturnsClaudeResult verifies that Run() converts
// claude.Result to provider.Result and returns it.
// Expected failure: ClaudeProvider type and Run() method do not exist yet
func TestClaudeProviderRunReturnsClaudeResult(t *testing.T) {
	client := &mockClaudeClient{
		resultToReturn: &claude.Result{
			Success:  true,
			Output:   "test output from claude",
			ExitCode: 0,
			Duration: 5 * time.Second,
			Model:    "sonnet",
		},
	}

	tierMap := map[string]string{
		TierMedium: "sonnet",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	result, err := provider.Run(ctx, "test", TierMedium)

	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if !result.Success {
		t.Errorf("Result.Success = %v, want true", result.Success)
	}

	if result.Output != "test output from claude" {
		t.Errorf("Result.Output = %q, want %q", result.Output, "test output from claude")
	}

	if result.ExitCode != 0 {
		t.Errorf("Result.ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Duration != 5*time.Second {
		t.Errorf("Result.Duration = %v, want %v", result.Duration, 5*time.Second)
	}

	if result.Model != "sonnet" {
		t.Errorf("Result.Model = %q, want %q", result.Model, "sonnet")
	}
}

// TestClaudeProviderRunPropagatesError verifies that Run() propagates errors
// from claude.Client.Run() unchanged.
// Expected failure: ClaudeProvider type and Run() method do not exist yet
func TestClaudeProviderRunPropagatesError(t *testing.T) {
	expectedErr := context.DeadlineExceeded
	client := &mockClaudeClient{
		errorToReturn: expectedErr,
	}

	tierMap := map[string]string{
		TierHigh: "opus",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	result, err := provider.Run(ctx, "test", TierHigh)

	if err != expectedErr {
		t.Errorf("Run() error = %v, want %v", err, expectedErr)
	}

	if result != nil {
		t.Errorf("Run() returned result %+v when error occurred, want nil", result)
	}
}

// TestClaudeProviderStreamRunMapsHighTier verifies that StreamRun() maps the
// "high" tier to the configured high-tier model and delegates to claude.Client.StreamRun().
// Expected failure: ClaudeProvider type and StreamRun() method do not exist yet
func TestClaudeProviderStreamRunMapsHighTier(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	prompt := "test streaming prompt"
	var output strings.Builder
	var handler EventHandler
	var toolHandler ToolCallHandler

	result, err := provider.StreamRun(ctx, prompt, TierHigh, &output, handler, toolHandler)

	if err != nil {
		t.Fatalf("StreamRun() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	// Verify the mock was called with the correct model
	if !client.streamRunCalled {
		t.Error("claude.Client.StreamRun() was not called")
	}

	if client.lastModel != "opus" {
		t.Errorf("StreamRun() called client with model %q, want %q", client.lastModel, "opus")
	}

	if client.lastPrompt != prompt {
		t.Errorf("StreamRun() called client with prompt %q, want %q", client.lastPrompt, prompt)
	}
}

// TestClaudeProviderStreamRunPassesEventHandler verifies that StreamRun()
// passes the EventHandler through to claude.Client.StreamRun() unchanged.
// Expected failure: ClaudeProvider type and StreamRun() method do not exist yet
func TestClaudeProviderStreamRunPassesEventHandler(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierMedium: "sonnet",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	var output strings.Builder

	handlerCalled := false
	handler := func(line []byte) {
		handlerCalled = true
	}

	_, err := provider.StreamRun(ctx, "test", TierMedium, &output, handler, nil)

	if err != nil {
		t.Fatalf("StreamRun() returned error: %v", err)
	}

	// The mock client should have received and called the handler
	if client.lastEventHandler == nil {
		t.Error("StreamRun() did not pass EventHandler to claude.Client")
	}

	// Invoke the handler that was passed to verify it's the same one
	if client.lastEventHandler != nil {
		client.lastEventHandler([]byte("test event"))
		if !handlerCalled {
			t.Error("EventHandler passed to claude.Client is not the same as provided")
		}
	}
}

// TestClaudeProviderStreamRunPassesToolCallHandler verifies that StreamRun()
// passes the ToolCallHandler through to claude.Client.StreamRun() unchanged.
// Expected failure: ClaudeProvider type and StreamRun() method do not exist yet
func TestClaudeProviderStreamRunPassesToolCallHandler(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierLow: "haiku",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	var output strings.Builder

	toolHandlerCalled := false
	toolHandler := func(event ToolEvent) {
		toolHandlerCalled = true
	}

	_, err := provider.StreamRun(ctx, "test", TierLow, &output, nil, toolHandler)

	if err != nil {
		t.Fatalf("StreamRun() returned error: %v", err)
	}

	// The mock client should have received and called the tool handler
	if client.lastToolCallHandler == nil {
		t.Error("StreamRun() did not pass ToolCallHandler to claude.Client")
	}

	// Invoke the handler that was passed to verify it's the same one
	if client.lastToolCallHandler != nil {
		client.lastToolCallHandler(claude.ToolEvent{})
		if !toolHandlerCalled {
			t.Error("ToolCallHandler passed to claude.Client is not the same as provided")
		}
	}
}

// TestClaudeProviderStreamRunPassesOutputWriter verifies that StreamRun()
// passes the output writer through to claude.Client.StreamRun().
// Expected failure: ClaudeProvider type and StreamRun() method do not exist yet
func TestClaudeProviderStreamRunPassesOutputWriter(t *testing.T) {
	client := &mockClaudeClient{
		resultToReturn: &claude.Result{
			Success: true,
			Output:  "stream output",
		},
	}

	tierMap := map[string]string{
		TierHigh: "opus",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	var output strings.Builder

	_, err := provider.StreamRun(ctx, "test", TierHigh, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() returned error: %v", err)
	}

	// Verify output writer was passed to client
	if client.lastOutput == nil {
		t.Error("StreamRun() did not pass output writer to claude.Client")
	}

	if client.lastOutput != &output {
		t.Error("StreamRun() passed a different output writer than provided")
	}
}

// TestClaudeProviderStreamRunReturnsClaudeResult verifies that StreamRun()
// converts claude.Result to provider.Result and returns it.
// Expected failure: ClaudeProvider type and StreamRun() method do not exist yet
func TestClaudeProviderStreamRunReturnsClaudeResult(t *testing.T) {
	client := &mockClaudeClient{
		resultToReturn: &claude.Result{
			Success:  true,
			Output:   "streamed output",
			ExitCode: 0,
			Duration: 3 * time.Second,
			Model:    "haiku",
		},
	}

	tierMap := map[string]string{
		TierLow: "haiku",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	var output strings.Builder

	result, err := provider.StreamRun(ctx, "test", TierLow, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() returned error: %v", err)
	}

	if !result.Success {
		t.Errorf("Result.Success = %v, want true", result.Success)
	}

	if result.Output != "streamed output" {
		t.Errorf("Result.Output = %q, want %q", result.Output, "streamed output")
	}

	if result.Duration != 3*time.Second {
		t.Errorf("Result.Duration = %v, want %v", result.Duration, 3*time.Second)
	}
}

// TestClaudeProviderStreamRunPropagatesError verifies that StreamRun() propagates
// errors from claude.Client.StreamRun() unchanged.
// Expected failure: ClaudeProvider type and StreamRun() method do not exist yet
func TestClaudeProviderStreamRunPropagatesError(t *testing.T) {
	expectedErr := context.Canceled
	client := &mockClaudeClient{
		errorToReturn: expectedErr,
	}

	tierMap := map[string]string{
		TierMedium: "sonnet",
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	var output strings.Builder

	result, err := provider.StreamRun(ctx, "test", TierMedium, &output, nil, nil)

	if err != expectedErr {
		t.Errorf("StreamRun() error = %v, want %v", err, expectedErr)
	}

	if result != nil {
		t.Errorf("StreamRun() returned result %+v when error occurred, want nil", result)
	}
}

// TestClaudeProviderRunWithUnmappedTier verifies that Run() handles tiers
// not present in the tier map (passes through tier as model name).
// Expected failure: ClaudeProvider type and Run() method do not exist yet
func TestClaudeProviderRunWithUnmappedTier(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierHigh: "opus",
		// TierMedium and TierLow deliberately omitted
	}

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	_, err := provider.Run(ctx, "test", TierMedium)

	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Should pass through the tier name as the model name when not mapped
	if client.lastModel != TierMedium {
		t.Errorf("Run() called client with model %q, want %q (tier passthrough)",
			client.lastModel, TierMedium)
	}
}

// TestClaudeProviderWithCustomModelNames verifies that tier→model mapping works
// with non-standard model names (e.g., "claude-opus-4", "claude-sonnet-4").
// Expected failure: ClaudeProvider type and methods do not exist yet
func TestClaudeProviderWithCustomModelNames(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{
		TierHigh:   "claude-opus-4",
		TierMedium: "claude-sonnet-4",
		TierLow:    "claude-haiku-4",
	}

	provider := NewClaudeProvider(client, tierMap)

	tests := []struct {
		tier          string
		expectedModel string
	}{
		{TierHigh, "claude-opus-4"},
		{TierMedium, "claude-sonnet-4"},
		{TierLow, "claude-haiku-4"},
	}

	for _, tt := range tests {
		t.Run("tier_"+tt.tier, func(t *testing.T) {
			ctx := context.Background()
			_, err := provider.Run(ctx, "test", tt.tier)

			if err != nil {
				t.Fatalf("Run() returned error: %v", err)
			}

			if client.lastModel != tt.expectedModel {
				t.Errorf("Run() called client with model %q, want %q",
					client.lastModel, tt.expectedModel)
			}
		})
	}
}

// TestClaudeProviderNilClientHandling verifies that ClaudeProvider handles
// nil claude.Client gracefully (returns error rather than panicking).
// Expected failure: ClaudeProvider type and methods do not exist yet
func TestClaudeProviderNilClientHandling(t *testing.T) {
	tierMap := map[string]string{
		TierHigh: "opus",
	}

	provider := NewClaudeProvider(nil, tierMap)

	ctx := context.Background()

	// Run() should return an error, not panic
	_, err := provider.Run(ctx, "test", TierHigh)
	if err == nil {
		t.Error("Run() with nil client returned nil error, expected error")
	}

	// StreamRun() should return an error, not panic
	var output strings.Builder
	_, err = provider.StreamRun(ctx, "test", TierHigh, &output, nil, nil)
	if err == nil {
		t.Error("StreamRun() with nil client returned nil error, expected error")
	}
}

// TestClaudeProviderEmptyTierMap verifies that ClaudeProvider works with
// an empty tier map (all tiers pass through as model names).
// Expected failure: ClaudeProvider type and methods do not exist yet
func TestClaudeProviderEmptyTierMap(t *testing.T) {
	client := &mockClaudeClient{}
	tierMap := map[string]string{} // empty

	provider := NewClaudeProvider(client, tierMap)

	ctx := context.Background()
	_, err := provider.Run(ctx, "test", "custom-model-v2")

	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Should pass through the tier/model name directly
	if client.lastModel != "custom-model-v2" {
		t.Errorf("Run() called client with model %q, want %q",
			client.lastModel, "custom-model-v2")
	}
}

// mockClaudeClient is a test double for claude.Client
type mockClaudeClient struct {
	runCalled           bool
	streamRunCalled     bool
	lastPrompt          string
	lastModel           string
	lastOutput          io.Writer
	lastEventHandler    claude.EventHandler
	lastToolCallHandler claude.ToolCallHandler
	resultToReturn      *claude.Result
	errorToReturn       error
}

func (m *mockClaudeClient) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	m.runCalled = true
	m.lastPrompt = prompt
	m.lastModel = model

	if m.errorToReturn != nil {
		return nil, m.errorToReturn
	}

	if m.resultToReturn != nil {
		return m.resultToReturn, nil
	}

	return &claude.Result{
		Success:  true,
		Output:   "mock output",
		ExitCode: 0,
		Duration: 1 * time.Second,
		Model:    model,
	}, nil
}

func (m *mockClaudeClient) StreamRun(ctx context.Context, prompt string, model string,
	output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
	m.streamRunCalled = true
	m.lastPrompt = prompt
	m.lastModel = model
	m.lastOutput = output
	m.lastEventHandler = handler
	m.lastToolCallHandler = onToolCall

	if m.errorToReturn != nil {
		return nil, m.errorToReturn
	}

	if m.resultToReturn != nil {
		return m.resultToReturn, nil
	}

	return &claude.Result{
		Success:  true,
		Output:   "mock stream output",
		ExitCode: 0,
		Duration: 1 * time.Second,
		Model:    model,
	}, nil
}

func (m *mockClaudeClient) RunValidation(ctx context.Context, commands []string,
	model string, workDir string) (*claude.Result, error) {
	return &claude.Result{Success: true}, nil
}
