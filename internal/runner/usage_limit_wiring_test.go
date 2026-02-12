package runner

import (
	"fmt"
	"testing"
)

// TestIterationResult_UsageLimitedField verifies that IterationResult struct
// has a UsageLimited bool field for tracking usage limit errors.
func TestIterationResult_UsageLimitedField(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationResult struct yet
	result := &IterationResult{
		BeadID:       "test-1",
		Model:        "sonnet",
		UsageLimited: true,
	}

	if !result.UsageLimited {
		t.Errorf("expected UsageLimited=true, got %v", result.UsageLimited)
	}
}

// TestIterationResult_UsageLimitedDefaultValue verifies that UsageLimited
// defaults to false when not explicitly set.
func TestIterationResult_UsageLimitedDefaultValue(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationResult struct yet
	result := &IterationResult{
		BeadID: "test-1",
		Model:  "sonnet",
	}

	// Default value should be false
	if result.UsageLimited {
		t.Error("expected UsageLimited to default to false")
	}
}

// TestIterationResult_UsageLimitedWithError verifies that when UsageLimited is true,
// the Error field is typically also set.
func TestIterationResult_UsageLimitedWithError(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationResult struct yet
	// This test documents the expected usage pattern
	result := &IterationResult{
		BeadID:       "test-1",
		Model:        "sonnet",
		Success:      false,
		UsageLimited: true,
		Error:        fmt.Errorf("usage limit detected"),
	}

	if !result.UsageLimited {
		t.Error("expected UsageLimited=true")
	}
	if result.Error == nil {
		t.Error("expected Error to be set when usage limit is detected")
	}
	if result.Success {
		t.Error("expected Success=false when usage limit is detected")
	}
}
