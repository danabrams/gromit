package provider

import (
	"context"
	"io"
	"path/filepath"
	"reflect"
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

// TestResultStructDoesNotExposeStdout verifies that Result no longer includes
// a Stdout field in the public struct definition.
func TestResultStructDoesNotExposeStdout(t *testing.T) {
	t.Parallel()
	resultType := reflect.TypeOf(Result{})
	if _, ok := resultType.FieldByName("Stdout"); ok {
		t.Fatalf("Result should not include Stdout field")
	}
}

func TestResultAndToolEventJSONTags(t *testing.T) {
	t.Parallel()

	type fieldTag struct {
		name string
		tag  string
	}

	resultWant := []fieldTag{
		{name: "Success", tag: "success"},
		{name: "Output", tag: "output"},
		{name: "Stderr", tag: "stderr"},
		{name: "Diagnostics", tag: "diagnostics"},
		{name: "FailureCategory", tag: "failure_category"},
		{name: "ExitCode", tag: "exit_code"},
		{name: "Duration", tag: "duration"},
		{name: "Model", tag: "model"},
		{name: "CostUSD", tag: "cost_usd"},
		{name: "InputTokens", tag: "input_tokens"},
		{name: "CachedInputTokens", tag: "cached_input_tokens"},
		{name: "OutputTokens", tag: "output_tokens"},
	}

	resultType := reflect.TypeOf(Result{})
	for _, field := range resultWant {
		sf, ok := resultType.FieldByName(field.name)
		if !ok {
			t.Fatalf("missing field %q on Result", field.name)
		}

		if got := sf.Tag.Get("json"); got != field.tag {
			t.Fatalf("Result.%s json tag = %q, want %q", field.name, got, field.tag)
		}
	}

	eventWant := []fieldTag{
		{name: "ToolName", tag: "tool_name"},
		{name: "FilePath", tag: "file_path"},
		{name: "Timestamp", tag: "timestamp"},
	}

	eventType := reflect.TypeOf(ToolEvent{})
	for _, field := range eventWant {
		sf, ok := eventType.FieldByName(field.name)
		if !ok {
			t.Fatalf("missing field %q on ToolEvent", field.name)
		}

		if got := sf.Tag.Get("json"); got != field.tag {
			t.Fatalf("ToolEvent.%s json tag = %q, want %q", field.name, got, field.tag)
		}
	}
}

// TestTierConstants verifies that the tier constants TierHigh, TierMedium, TierLow
// are defined as strings with the expected values.
func TestTierConstants(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			if tt.tier != tt.expectedValue {
				t.Errorf("tier constant = %q, want %q", tt.tier, tt.expectedValue)
			}
		})
	}
}

// TestTierConstantsAreDistinct verifies that all tier constants have unique values.
func TestTierConstantsAreDistinct(t *testing.T) {
	t.Parallel()
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

// TestEventHandlerSignature verifies that EventHandler matches the expected function signature.
func TestEventHandlerSignature(t *testing.T) {
	t.Parallel()
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

// TestToolCallHandlerSignature verifies that ToolCallHandler matches the expected function signature.
func TestToolCallHandlerSignature(t *testing.T) {
	t.Parallel()
	eventReceived := false
	var capturedEvent ToolEvent
	scriptPath := filepath.Join(t.TempDir(), "script.sh")

	var handler ToolCallHandler = func(event ToolEvent) {
		eventReceived = true
		capturedEvent = event
	}

	testEvent := ToolEvent{
		ToolName:  "bash",
		FilePath:  scriptPath,
		Timestamp: time.Now(),
	}

	handler(testEvent)

	if !eventReceived {
		t.Error("ToolCallHandler was not invoked")
	}

	if capturedEvent.ToolName != "bash" {
		t.Errorf("ToolCallHandler captured ToolName = %q, want %q", capturedEvent.ToolName, "bash")
	}
	if capturedEvent.FilePath != scriptPath {
		t.Errorf("ToolCallHandler captured FilePath = %q, want %q", capturedEvent.FilePath, scriptPath)
	}
}

// TestProviderMethodSignatures verifies that all Provider interface methods
// have the correct signatures and can be called.
func TestProviderMethodSignatures(t *testing.T) {
	t.Parallel(
	// Use the package-level mock implementation
	)

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
	_, _ = impl.RunValidation(ctx, []string{"test"}, TierLow, t.TempDir())

	// Verify IsUsageLimitError(result, err) bool
	_ = impl.IsUsageLimitError(&Result{}, nil)
}

// TestTierFromLegacyModelClaudeModels verifies that TierFromLegacyModel() maps
// Claude model names (opus, sonnet, haiku) to the correct tier constants.
func TestTierFromLegacyModelClaudeModels(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

// TestTierFromLegacyModelOpenAIModels verifies that TierFromLegacyModel() maps
// OpenAI model names (o3, gpt-4o, gpt-4o-mini) to the correct tier constants.
func TestTierFromLegacyModelOpenAIModels(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

// TestTierFromLegacyModelGeminiModels verifies that Gemini model names map
// to the expected tier constants for backward compatibility.
func TestTierFromLegacyModelGeminiModels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		modelName    string
		expectedTier string
	}{
		{
			name:         "gemini-3.1-pro maps to high tier",
			modelName:    "gemini-3.1-pro",
			expectedTier: TierHigh,
		},
		{
			name:         "gemini-3-pro maps to high tier",
			modelName:    "gemini-3-pro",
			expectedTier: TierHigh,
		},
		{
			name:         "gemini-3-flash maps to medium tier",
			modelName:    "gemini-3-flash",
			expectedTier: TierMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

// TestTierFromLegacyModelUnrecognizedPassthrough verifies that TierFromLegacyModel()
// passes through unrecognized model names unchanged for forward compatibility.
func TestTierFromLegacyModelUnrecognizedPassthrough(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
func TestTierFromLegacyModelCaseInsensitive(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
func TestTierFromLegacyModelAllKnownModels(t *testing.T) {
	t.Parallel(
	// Complete mapping from the spec: opus→high, sonnet→medium, haiku→low,
	// o3→high, gpt-4o→medium, gpt-4o-mini→low, gpt-5.3-codex→medium,
	// gpt-5.1-codex-mini→low
	)

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
		{"gpt-5.1-codex-mini", TierLow},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			t.Parallel()
			got := TierFromLegacyModel(tt.modelName)
			if got != tt.expectedTier {
				t.Errorf("TierFromLegacyModel(%q) = %q, want %q", tt.modelName, got, tt.expectedTier)
			}
		})
	}
}

func TestTierFromLegacyModel_XHigh(t *testing.T) {
	// No model maps to xhigh yet, but xhigh should be a valid tier constant
	if TierXHigh != "xhigh" {
		t.Fatalf("want xhigh, got %s", TierXHigh)
	}
}

func TestTierToLegacyModel_XHigh(t *testing.T) {
	model := TierToLegacyModel(TierXHigh)
	if model != "opus" {
		t.Fatalf("xhigh should map to opus, got %s", model)
	}
}

// TestTierFromLegacyModelIdempotent verifies that TierFromLegacyModel()
// is idempotent - calling it multiple times with the same input produces
// the same output, and tier constants remain unchanged when passed through.
func TestTierFromLegacyModelIdempotent(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
