package provider

import (
	"testing"

	"github.com/danabrams/gromit/internal/claude"
)

// TestConvertResultPropagatesCostFields verifies that convertResult maps
// CostUSD, InputTokens, and OutputTokens from claude.Result to provider.Result.
// Previously these fields were silently dropped, causing 0 cost for all Claude models.
func TestConvertResultPropagatesCostFields(t *testing.T) {
	claudeResult := &claude.Result{
		Success:      true,
		Output:       "test output",
		ExitCode:     0,
		Model:        "sonnet",
		CostUSD:      1.23,
		InputTokens:  5000,
		OutputTokens: 2000,
	}

	result := convertResult(claudeResult)

	if result.CostUSD != 1.23 {
		t.Errorf("CostUSD = %f, want 1.23", result.CostUSD)
	}
	if result.InputTokens != 5000 {
		t.Errorf("InputTokens = %d, want 5000", result.InputTokens)
	}
	if result.OutputTokens != 2000 {
		t.Errorf("OutputTokens = %d, want 2000", result.OutputTokens)
	}
}

// TestConvertResultPreservesExistingFields verifies that cost field propagation
// doesn't break existing field mapping.
func TestConvertResultPreservesExistingFields(t *testing.T) {
	claudeResult := &claude.Result{
		Success:      true,
		Output:       "hello",
		ExitCode:     0,
		Model:        "opus",
		CostUSD:      0.50,
		InputTokens:  100,
		OutputTokens: 50,
	}

	result := convertResult(claudeResult)

	if result.Success != true {
		t.Error("Success should be true")
	}
	if result.Output != "hello" {
		t.Errorf("Output = %q, want %q", result.Output, "hello")
	}
	if result.Model != "opus" {
		t.Errorf("Model = %q, want %q", result.Model, "opus")
	}
}
