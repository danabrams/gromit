package provider

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// mockProvider is a test implementation of the Provider interface
type mockProvider struct {
	name string
}

var _ Provider = (*mockProvider)(nil)

func (m *mockProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockProvider) ModelForTier(tier string) string {
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}
	if model, ok := tierMap[tier]; ok {
		return model
	}
	return tier
}

func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	return &Result{Success: true}, nil
}

func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	return &Result{Success: true}, nil
}

func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	return &Result{Success: true}, nil
}

func (m *mockProvider) IsUsageLimitError(result *Result, err error) bool {
	return false
}

func (m *mockProvider) IsValidationPassed(result *Result) bool {
	return IsValidationPassed(result)
}

func (m *mockProvider) IsScopeTooLarge(result *Result) (bool, string) {
	return IsScopeTooLarge(result)
}

// TestProviderInterfaceDefinition verifies that the Provider interface exists
// and can be used in type assertions and interface satisfaction checks.
// Expected failure: Provider interface does not exist yet
func TestProviderInterfaceDefinition(t *testing.T) {
	// Verify we can declare a variable of type Provider
	var p Provider
	if p != nil {
		t.Error("nil Provider should be nil")
	}
}

// TestProviderNameMethod verifies that Provider interface has a Name() method
// that returns the provider's name as a string.
// Expected failure: Provider interface and Name() method do not exist yet
func TestProviderNameMethod(t *testing.T) {
	tests := []struct {
		name         string
		provider     Provider
		expectedName string
	}{
		{
			name:         "nil provider",
			provider:     nil,
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.provider == nil {
				return // Skip nil case
			}
			got := tt.provider.Name()
			if got != tt.expectedName {
				t.Errorf("Name() = %q, want %q", got, tt.expectedName)
			}
		})
	}
}

// TestProviderRunMethod verifies that Provider interface has a Run() method
// that accepts context, prompt string, and tier string, returning Result and error.
// Expected failure: Provider interface, Run() method, and Result struct do not exist yet
func TestProviderRunMethod(t *testing.T) {
	var p Provider
	if p == nil {
		t.Skip("Cannot test Run() on nil provider")
	}

	ctx := context.Background()
	result, err := p.Run(ctx, "test prompt", TierMedium)

	if err == nil && result == nil {
		t.Error("Run() returned nil result with nil error")
	}
}

// TestProviderStreamRunMethod verifies that Provider interface has a StreamRun() method
// that accepts context, prompt, tier, output writer, EventHandler, and ToolCallHandler.
// Expected failure: Provider interface, StreamRun() method, EventHandler, and ToolCallHandler types do not exist yet
func TestProviderStreamRunMethod(t *testing.T) {
	var p Provider
	if p == nil {
		t.Skip("Cannot test StreamRun() on nil provider")
	}

	ctx := context.Background()
	var output strings.Builder
	var handler EventHandler
	var toolCallHandler ToolCallHandler

	result, err := p.StreamRun(ctx, "test prompt", TierHigh, &output, handler, toolCallHandler)

	if err == nil && result == nil {
		t.Error("StreamRun() returned nil result with nil error")
	}
}

// TestProviderRunValidationMethod verifies that Provider interface has a RunValidation() method
// that accepts context, commands slice, tier, and working directory.
// Expected failure: Provider interface and RunValidation() method do not exist yet
func TestProviderRunValidationMethod(t *testing.T) {
	var p Provider
	if p == nil {
		t.Skip("Cannot test RunValidation() on nil provider")
	}

	ctx := context.Background()
	commands := []string{"go test ./..."}
	result, err := p.RunValidation(ctx, commands, TierLow, "/tmp")

	if err == nil && result == nil {
		t.Error("RunValidation() returned nil result with nil error")
	}
}

// TestProviderIsUsageLimitErrorMethod verifies that Provider interface has an
// IsUsageLimitError() method that detects provider-specific usage limit errors.
// Expected failure: Provider interface and IsUsageLimitError() method do not exist yet
func TestProviderIsUsageLimitErrorMethod(t *testing.T) {
	var p Provider
	if p == nil {
		t.Skip("Cannot test IsUsageLimitError() on nil provider")
	}

	result := &Result{
		Success:  false,
		ExitCode: 1,
		Output:   "usage limit exceeded",
	}

	isLimitError := p.IsUsageLimitError(result, nil)

	// The specific return value depends on provider implementation,
	// but the method should be callable
	_ = isLimitError
}

// TestResultStruct verifies that the Result struct has the required fields:
// Success, Output, ExitCode, Duration, Model.
// Expected failure: Result struct does not exist yet
func TestResultStruct(t *testing.T) {
	result := &Result{
		Success:  true,
		Output:   "test output",
		ExitCode: 0,
		Duration: 5 * time.Second,
		Model:    "sonnet",
	}

	if !result.Success {
		t.Errorf("Success = %v, want true", result.Success)
	}
	if result.Output != "test output" {
		t.Errorf("Output = %q, want %q", result.Output, "test output")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want %v", result.Duration, 5*time.Second)
	}
	if result.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", result.Model, "sonnet")
	}
}

// TestResultStructZeroValue verifies that Result struct can be zero-initialized
// and has sensible zero values for all fields.
// Expected failure: Result struct does not exist yet
func TestResultStructZeroValue(t *testing.T) {
	var result Result

	if result.Success {
		t.Error("zero-value Success should be false")
	}
	if result.Output != "" {
		t.Errorf("zero-value Output = %q, want empty", result.Output)
	}
	if result.ExitCode != 0 {
		t.Errorf("zero-value ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Duration != 0 {
		t.Errorf("zero-value Duration = %v, want 0", result.Duration)
	}
	if result.Model != "" {
		t.Errorf("zero-value Model = %q, want empty", result.Model)
	}
}

// TestTierConstants verifies that the tier constants TierHigh, TierMedium, TierLow
// are defined as strings with the expected values.
// Expected failure: TierHigh, TierMedium, TierLow constants do not exist yet
func TestTierConstants(t *testing.T) {
	tests := []struct {
		name          string
		tier          string
		expectedValue string
	}{
		{
			name:          "high tier",
			tier:          TierHigh,
			expectedValue: "high",
		},
		{
			name:          "medium tier",
			tier:          TierMedium,
			expectedValue: "medium",
		},
		{
			name:          "low tier",
			tier:          TierLow,
			expectedValue: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tier != tt.expectedValue {
				t.Errorf("tier constant = %q, want %q", tt.tier, tt.expectedValue)
			}
		})
	}
}

// TestTierConstantsAreDistinct verifies that all tier constants have unique values.
// Expected failure: TierHigh, TierMedium, TierLow constants do not exist yet
func TestTierConstantsAreDistinct(t *testing.T) {
	tiers := []string{TierHigh, TierMedium, TierLow}

	seen := make(map[string]bool)
	for _, tier := range tiers {
		if seen[tier] {
			t.Errorf("duplicate tier constant value: %q", tier)
		}
		seen[tier] = true
	}

	if len(seen) != 3 {
		t.Errorf("expected 3 distinct tier constants, got %d", len(seen))
	}
}

// TestEventHandlerType verifies that EventHandler is a function type that accepts
// a byte slice representing a line of JSON output.
// Expected failure: EventHandler type does not exist yet
func TestEventHandlerType(t *testing.T) {
	var handler EventHandler

	if handler != nil {
		t.Error("nil EventHandler should be nil")
	}

	// Verify we can assign a function to EventHandler
	handler = func(line []byte) {
		// Process line
		_ = line
	}

	if handler == nil {
		t.Error("assigned EventHandler should not be nil")
	}

	// Verify we can call the handler
	handler([]byte(`{"type":"test"}`))
}

// TestEventHandlerSignature verifies that EventHandler matches the expected function signature.
// Expected failure: EventHandler type does not exist yet
func TestEventHandlerSignature(t *testing.T) {
	lineProcessed := false
	var capturedLine []byte

	var handler EventHandler = func(line []byte) {
		lineProcessed = true
		capturedLine = make([]byte, len(line))
		copy(capturedLine, line)
	}

	testData := []byte(`{"type":"assistant","text":"Hello"}`)
	handler(testData)

	if !lineProcessed {
		t.Error("EventHandler was not invoked")
	}

	if string(capturedLine) != string(testData) {
		t.Errorf("EventHandler captured line = %q, want %q", capturedLine, testData)
	}
}

// TestToolCallHandlerType verifies that ToolCallHandler is a function type that accepts
// a ToolEvent struct with tool call metadata.
// Expected failure: ToolCallHandler type does not exist yet
func TestToolCallHandlerType(t *testing.T) {
	var handler ToolCallHandler

	if handler != nil {
		t.Error("nil ToolCallHandler should be nil")
	}

	// Verify we can assign a function to ToolCallHandler
	handler = func(event ToolEvent) {
		// Process tool event
		_ = event
	}

	if handler == nil {
		t.Error("assigned ToolCallHandler should not be nil")
	}
}

// TestToolCallHandlerSignature verifies that ToolCallHandler matches the claude.ToolCallHandler signature.
// Expected failure: ToolCallHandler type does not exist yet
func TestToolCallHandlerSignature(t *testing.T) {
	eventReceived := false
	var capturedEvent ToolEvent

	var handler ToolCallHandler = func(event ToolEvent) {
		eventReceived = true
		capturedEvent = event
	}

	testEvent := ToolEvent{
		ToolName:  "bash",
		FilePath:  "/tmp/script.sh",
		Timestamp: time.Now(),
	}

	handler(testEvent)

	if !eventReceived {
		t.Error("ToolCallHandler was not invoked")
	}

	if capturedEvent.ToolName != "bash" {
		t.Errorf("ToolCallHandler captured ToolName = %q, want %q", capturedEvent.ToolName, "bash")
	}
	if capturedEvent.FilePath != "/tmp/script.sh" {
		t.Errorf("ToolCallHandler captured FilePath = %q, want %q", capturedEvent.FilePath, "/tmp/script.sh")
	}
}

// TestToolEventStruct verifies that ToolEvent struct exists with the expected fields.
// This is referenced by the ToolCallHandler type definition.
// Expected failure: ToolEvent struct does not exist yet in provider package
func TestToolEventStruct(t *testing.T) {
	now := time.Now()
	event := ToolEvent{
		ToolName:  "python",
		FilePath:  "/tmp/script.py",
		Timestamp: now,
	}

	if event.ToolName != "python" {
		t.Errorf("ToolName = %q, want %q", event.ToolName, "python")
	}
	if event.FilePath != "/tmp/script.py" {
		t.Errorf("FilePath = %q, want %q", event.FilePath, "/tmp/script.py")
	}
	if !event.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", event.Timestamp, now)
	}
}

// TestToolEventZeroValue verifies that ToolEvent can be zero-initialized.
// Expected failure: ToolEvent struct does not exist yet in provider package
func TestToolEventZeroValue(t *testing.T) {
	var event ToolEvent

	if event.ToolName != "" {
		t.Errorf("zero-value ToolName = %q, want empty", event.ToolName)
	}
	if event.FilePath != "" {
		t.Errorf("zero-value FilePath = %q, want empty", event.FilePath)
	}
	if !event.Timestamp.IsZero() {
		t.Errorf("zero-value Timestamp should be zero, got %v", event.Timestamp)
	}
}

// TestProviderInterfaceCompatibility verifies that the Provider interface can be
// used in real code patterns like dependency injection and interface satisfaction checks.
// Expected failure: Provider interface does not exist yet
func TestProviderInterfaceCompatibility(t *testing.T) {
	// Test that we can pass Provider to a function expecting the interface
	consumeProvider := func(p Provider) string {
		if p == nil {
			return ""
		}
		return p.Name()
	}

	result := consumeProvider(nil)
	if result != "" {
		t.Errorf("consumeProvider(nil) = %q, want empty", result)
	}
}

// TestResultFieldAccess verifies that all Result struct fields are accessible
// and can be read and written.
// Expected failure: Result struct does not exist yet
func TestResultFieldAccess(t *testing.T) {
	// Test write access
	result := &Result{}
	result.Success = true
	result.Output = "output text"
	result.ExitCode = 42
	result.Duration = 10 * time.Millisecond
	result.Model = "opus"

	// Test read access
	if !result.Success {
		t.Error("failed to write/read Success field")
	}
	if result.Output != "output text" {
		t.Error("failed to write/read Output field")
	}
	if result.ExitCode != 42 {
		t.Error("failed to write/read ExitCode field")
	}
	if result.Duration != 10*time.Millisecond {
		t.Error("failed to write/read Duration field")
	}
	if result.Model != "opus" {
		t.Error("failed to write/read Model field")
	}
}

// TestProviderMethodSignatures verifies that all Provider interface methods
// have the correct signatures and can be called.
// Expected failure: Provider interface and its methods do not exist yet
func TestProviderMethodSignatures(t *testing.T) {
	// Use the package-level mock implementation
	impl := &mockProvider{}

	// Verify Name() string
	_ = impl.Name()

	// Verify Run(ctx, prompt, tier) (*Result, error)
	ctx := context.Background()
	_, _ = impl.Run(ctx, "prompt", TierMedium)

	// Verify StreamRun(ctx, prompt, tier, output, handler, toolHandler) (*Result, error)
	var output io.Writer
	var handler EventHandler
	var toolHandler ToolCallHandler
	_, _ = impl.StreamRun(ctx, "prompt", TierHigh, output, handler, toolHandler)

	// Verify RunValidation(ctx, commands, tier, workDir) (*Result, error)
	_, _ = impl.RunValidation(ctx, []string{"test"}, TierLow, "/tmp")

	// Verify IsUsageLimitError(result, err) bool
	_ = impl.IsUsageLimitError(&Result{}, nil)
}

// TestEventHandlerNilSafety verifies that EventHandler can handle nil values safely.
// Expected failure: EventHandler type does not exist yet
func TestEventHandlerNilSafety(t *testing.T) {
	var handler EventHandler = nil

	// Calling a nil handler should be safe (no-op)
	if handler != nil {
		handler([]byte("test"))
	}
}

// TestToolCallHandlerNilSafety verifies that ToolCallHandler can handle nil values safely.
// Expected failure: ToolCallHandler type does not exist yet
func TestToolCallHandlerNilSafety(t *testing.T) {
	var handler ToolCallHandler = nil

	// Calling a nil handler should be safe (no-op)
	if handler != nil {
		handler(ToolEvent{})
	}
}

// TestProviderRunWithTiers verifies that Provider.Run() accepts all valid tier constants.
// Expected failure: Provider interface, Run() method, and tier constants do not exist yet
func TestProviderRunWithTiers(t *testing.T) {
	tiers := []string{TierHigh, TierMedium, TierLow}

	for _, tier := range tiers {
		t.Run("tier_"+tier, func(t *testing.T) {
			var p Provider
			if p == nil {
				t.Skip("Cannot test Run() on nil provider")
			}

			ctx := context.Background()
			_, err := p.Run(ctx, "test", tier)

			// We expect an error (nil provider), but the method signature should be correct
			_ = err
		})
	}
}

// TestProviderRunValidationWithTiers verifies that Provider.RunValidation()
// accepts all valid tier constants.
// Expected failure: Provider interface, RunValidation() method, and tier constants do not exist yet
func TestProviderRunValidationWithTiers(t *testing.T) {
	tiers := []string{TierHigh, TierMedium, TierLow}

	for _, tier := range tiers {
		t.Run("tier_"+tier, func(t *testing.T) {
			var p Provider
			if p == nil {
				t.Skip("Cannot test RunValidation() on nil provider")
			}

			ctx := context.Background()
			_, err := p.RunValidation(ctx, []string{"test"}, tier, "/tmp")

			// We expect an error (nil provider), but the method signature should be correct
			_ = err
		})
	}
}

// TestResultSuccessIndicatesExitCode verifies that Result.Success correlates
// with ExitCode == 0 for successful executions.
// Expected failure: Result struct does not exist yet
func TestResultSuccessIndicatesExitCode(t *testing.T) {
	tests := []struct {
		name     string
		success  bool
		exitCode int
	}{
		{
			name:     "successful execution",
			success:  true,
			exitCode: 0,
		},
		{
			name:     "failed execution",
			success:  false,
			exitCode: 1,
		},
		{
			name:     "failed with specific code",
			success:  false,
			exitCode: 127,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{
				Success:  tt.success,
				ExitCode: tt.exitCode,
			}

			if result.Success != tt.success {
				t.Errorf("Success = %v, want %v", result.Success, tt.success)
			}
			if result.ExitCode != tt.exitCode {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.exitCode)
			}
		})
	}
}

// TestProviderIsUsageLimitErrorWithVariousResults verifies that
// IsUsageLimitError() can be called with different result states.
// Expected failure: Provider interface and IsUsageLimitError() method do not exist yet
func TestProviderIsUsageLimitErrorWithVariousResults(t *testing.T) {
	tests := []struct {
		name   string
		result *Result
		err    error
	}{
		{
			name:   "nil result and error",
			result: nil,
			err:    nil,
		},
		{
			name: "failed result with error output",
			result: &Result{
				Success:  false,
				Output:   "usage limit exceeded",
				ExitCode: 1,
			},
			err: nil,
		},
		{
			name: "successful result",
			result: &Result{
				Success:  true,
				Output:   "completed",
				ExitCode: 0,
			},
			err: nil,
		},
	}

	var p Provider
	if p == nil {
		t.Skip("Cannot test IsUsageLimitError() on nil provider")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isLimit := p.IsUsageLimitError(tt.result, tt.err)
			// The actual return value depends on implementation,
			// but the method should be callable with these arguments
			_ = isLimit
		})
	}
}

// TestTierFromLegacyModelClaudeModels verifies that TierFromLegacyModel() maps
// Claude model names (opus, sonnet, haiku) to the correct tier constants.
// Expected failure: TierFromLegacyModel() function does not exist yet
func TestTierFromLegacyModelClaudeModels(t *testing.T) {
	tests := []struct {
		name         string
		modelName    string
		expectedTier string
	}{
		{
			name:         "opus maps to high tier",
			modelName:    "opus",
			expectedTier: TierHigh,
		},
		{
			name:         "sonnet maps to medium tier",
			modelName:    "sonnet",
			expectedTier: TierMedium,
		},
		{
			name:         "haiku maps to low tier",
			modelName:    "haiku",
			expectedTier: TierLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

// TestTierFromLegacyModelOpenAIModels verifies that TierFromLegacyModel() maps
// OpenAI model names (o3, gpt-4o, gpt-4o-mini) to the correct tier constants.
// Expected failure: TierFromLegacyModel() function does not exist yet
func TestTierFromLegacyModelOpenAIModels(t *testing.T) {
	tests := []struct {
		name         string
		modelName    string
		expectedTier string
	}{
		{
			name:         "o3 maps to high tier",
			modelName:    "o3",
			expectedTier: TierHigh,
		},
		{
			name:         "gpt-4o maps to medium tier",
			modelName:    "gpt-4o",
			expectedTier: TierMedium,
		},
		{
			name:         "gpt-4o-mini maps to low tier",
			modelName:    "gpt-4o-mini",
			expectedTier: TierLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

// TestTierFromLegacyModelUnrecognizedPassthrough verifies that TierFromLegacyModel()
// passes through unrecognized model names unchanged for forward compatibility.
// Expected failure: TierFromLegacyModel() function does not exist yet
func TestTierFromLegacyModelUnrecognizedPassthrough(t *testing.T) {
	tests := []struct {
		name         string
		modelName    string
		expectedTier string
	}{
		{
			name:         "unknown model name passed through",
			modelName:    "gpt-5-turbo",
			expectedTier: "gpt-5-turbo",
		},
		{
			name:         "custom model name passed through",
			modelName:    "custom-llm-v2",
			expectedTier: "custom-llm-v2",
		},
		{
			name:         "empty string passed through",
			modelName:    "",
			expectedTier: "",
		},
		{
			name:         "tier constant passed through unchanged",
			modelName:    "high",
			expectedTier: "high",
		},
		{
			name:         "another tier constant passed through",
			modelName:    "medium",
			expectedTier: "medium",
		},
		{
			name:         "low tier constant passed through",
			modelName:    "low",
			expectedTier: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

// TestTierFromLegacyModelCaseInsensitive verifies that TierFromLegacyModel()
// handles model names case-insensitively for known models but preserves case
// for unrecognized models (passthrough).
// Expected failure: TierFromLegacyModel() function does not exist yet
func TestTierFromLegacyModelCaseInsensitive(t *testing.T) {
	tests := []struct {
		name         string
		modelName    string
		expectedTier string
	}{
		{
			name:         "OPUS uppercase maps to high tier",
			modelName:    "OPUS",
			expectedTier: TierHigh,
		},
		{
			name:         "Sonnet mixed case maps to medium tier",
			modelName:    "Sonnet",
			expectedTier: TierMedium,
		},
		{
			name:         "HAIKU uppercase maps to low tier",
			modelName:    "HAIKU",
			expectedTier: TierLow,
		},
		{
			name:         "GPT-4O uppercase maps to medium tier",
			modelName:    "GPT-4O",
			expectedTier: TierMedium,
		},
		{
			name:         "O3 uppercase maps to high tier",
			modelName:    "O3",
			expectedTier: TierHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

// TestTierFromLegacyModelAllKnownModels verifies that all known model names
// from the spec are properly mapped to their corresponding tiers.
// This test captures the complete mapping requirement in one place.
// Expected failure: TierFromLegacyModel() function does not exist yet
func TestTierFromLegacyModelAllKnownModels(t *testing.T) {
	// Complete mapping from the spec: opus→high, sonnet→medium, haiku→low,
	// o3→high, gpt-4o→medium, gpt-4o-mini→low, gpt-5.3-codex→medium
	tests := []struct {
		modelName    string
		expectedTier string
	}{
		// Claude models
		{"opus", TierHigh},
		{"sonnet", TierMedium},
		{"haiku", TierLow},
		// OpenAI models
		{"o3", TierHigh},
		{"gpt-4o", TierMedium},
		{"gpt-4o-mini", TierLow},
		// Codex models
		{"gpt-5.3-codex", TierMedium},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

// TestTierFromLegacyModelIdempotent verifies that TierFromLegacyModel()
// is idempotent - calling it multiple times with the same input produces
// the same output, and tier constants remain unchanged when passed through.
// Expected failure: TierFromLegacyModel() function does not exist yet
func TestTierFromLegacyModelIdempotent(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
	}{
		{
			name:      "opus remains consistent",
			modelName: "opus",
		},
		{
			name:      "high tier remains high",
			modelName: TierHigh,
		},
		{
			name:      "unknown model remains consistent",
			modelName: "future-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := TierFromLegacyModel(tt.modelName)
			second := TierFromLegacyModel(tt.modelName)
			third := TierFromLegacyModel(first)

			if first != second {
				t.Errorf("TierFromLegacyModel(%q) not idempotent: first=%q, second=%q",
					tt.modelName, first, second)
			}

			// Applying the function to its own output should be idempotent
			if first != third {
				t.Errorf("TierFromLegacyModel not idempotent when applied to result: "+
					"TierFromLegacyModel(%q)=%q, but TierFromLegacyModel(%q)=%q",
					tt.modelName, first, first, third)
			}
		})
	}
}

// TestTierFromLegacyModelCodexModels verifies that TierFromLegacyModel() maps
// Codex model names (gpt-5.3-codex) to the correct tier constants.
// Expected failure: gpt-5.3-codex is not yet in the TierFromLegacyModel mapping
func TestTierFromLegacyModelCodexModels(t *testing.T) {
	tests := []struct {
		name         string
		modelName    string
		expectedTier string
	}{
		{
			name:         "gpt-5.3-codex maps to medium tier",
			modelName:    "gpt-5.3-codex",
			expectedTier: TierMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

// TestTierFromLegacyModelCodexCaseInsensitive verifies that TierFromLegacyModel()
// handles gpt-5.3-codex model name case-insensitively.
// Expected failure: gpt-5.3-codex is not yet in the TierFromLegacyModel mapping
func TestTierFromLegacyModelCodexCaseInsensitive(t *testing.T) {
	tests := []struct {
		name         string
		modelName    string
		expectedTier string
	}{
		{
			name:         "GPT-5.3-CODEX uppercase maps to medium tier",
			modelName:    "GPT-5.3-CODEX",
			expectedTier: TierMedium,
		},
		{
			name:         "Gpt-5.3-Codex mixed case maps to medium tier",
			modelName:    "Gpt-5.3-Codex",
			expectedTier: TierMedium,
		},
		{
			name:         "gpt-5.3-codex lowercase maps to medium tier",
			modelName:    "gpt-5.3-codex",
			expectedTier: TierMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}
