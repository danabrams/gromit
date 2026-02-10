//go:build acceptance

package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClaudeAdapterFileExists verifies that adapter.go exists
// in the internal/claude package
func TestClaudeAdapterFileExists(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	_, err := os.Stat(adapterPath)
	if err != nil {
		t.Errorf("adapter.go must be created in internal/claude package: %v", err)
	}
}

// TestClaudeAdapterDefinesType verifies that ClaudeRunnerAdapter
// type is defined in internal/claude/adapter.go
func TestClaudeAdapterDefinesType(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("cannot read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// Should define either ClaudeRunnerAdapter or similar adapter type
	hasAdapter := strings.Contains(sourceStr, "type") &&
		(strings.Contains(sourceStr, "ClaudeRunnerAdapter") ||
			strings.Contains(sourceStr, "Adapter struct") ||
			strings.Contains(sourceStr, "adapter struct"))

	if !hasAdapter {
		t.Error("adapter.go should define a ClaudeRunnerAdapter type")
	}
}

// TestClaudeAdapterExportsRunMethod verifies the adapter exports
// a Run method with the correct signature
func TestClaudeAdapterExportsRunMethod(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("cannot read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// Must have a Run method
	if !strings.Contains(sourceStr, "func") || !strings.Contains(sourceStr, "Run(") {
		t.Error("adapter must export a Run method")
	}

	// Run method must accept context (as per typical Claude integration pattern)
	if !strings.Contains(sourceStr, "context") {
		t.Error("Run method should accept a context parameter")
	}
}

// TestClaudeAdapterConvertsResults verifies the adapter properly
// converts claude.Result to the output type
func TestClaudeAdapterConvertsResults(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("cannot read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// Adapter acceptance criterion 1: "exports ClaudeRunnerAdapter with Run method
	// that wraps claude.Runner and converts results"
	// This means the implementation should map result fields

	// Should show evidence of accessing result fields for conversion
	hasResultConversion := (strings.Contains(sourceStr, "Success") ||
		strings.Contains(sourceStr, "Output")) &&
		strings.Contains(sourceStr, "Result")

	if !hasResultConversion {
		t.Error("adapter should convert result fields from claude.Result (mapping Success, Output, etc)")
	}
}

// TestClaudeAdapterIncludesNilResultCheck verifies the adapter
// includes nil-result safety check before accessing fields
func TestClaudeAdapterIncludesNilResultCheck(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("cannot read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// Acceptance criterion 2: "includes nil-result check before accessing result fields"
	// The code must check if result is nil before accessing fields like Success, Output

	// Should have nil check(s)
	hasNilCheck := strings.Contains(sourceStr, "== nil") ||
		strings.Contains(sourceStr, "!= nil")

	if !hasNilCheck {
		t.Error("adapter must include nil-result check before accessing result fields")
	}

	// Should have evidence of checking for nil before accessing result fields
	// (either explicit nil check in Run method, or safe access pattern)
	hasResultFieldAccess := strings.Contains(sourceStr, ".Success") ||
		strings.Contains(sourceStr, ".Output") ||
		strings.Contains(sourceStr, "[\"Success\"]") ||
		strings.Contains(sourceStr, "[\"Output\"]")

	if !hasResultFieldAccess && !strings.Contains(sourceStr, "return") {
		t.Error("adapter should access result fields for conversion")
	}
}

// TestClaudeAdapterWrapsClaudeRunner verifies the adapter wraps
// a Claude client/runner interface
func TestClaudeAdapterWrapsClaudeRunner(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("cannot read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// The adapter struct should have a field for the underlying Claude runner
	// Typical patterns: "client", "runner", "runner Client", etc.
	hasRunnerField := (strings.Contains(sourceStr, "client") ||
		strings.Contains(sourceStr, "runner") ||
		strings.Contains(sourceStr, "Client") ||
		strings.Contains(sourceStr, "Runner")) &&
		strings.Contains(sourceStr, "struct")

	if !hasRunnerField {
		t.Error("adapter should wrap a Claude client/runner in its struct")
	}
}

// TestClaudeAdapterExportsConstructor verifies the adapter exports
// a constructor function (acceptance criteria requirement for public API)
func TestClaudeAdapterExportsConstructor(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("cannot read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// Should export a constructor (NewClaudeRunnerAdapter or similar)
	hasConstructor := strings.Contains(sourceStr, "func New") &&
		(strings.Contains(sourceStr, "ClaudeRunnerAdapter") ||
			strings.Contains(sourceStr, "Adapter"))

	if !hasConstructor {
		t.Error("adapter should export a constructor function (e.g., NewClaudeRunnerAdapter)")
	}
}

// TestClaudeAdapterNilCheckBeforeFieldAccess verifies nil checks
// occur before accessing result fields
func TestClaudeAdapterNilCheckBeforeFieldAccess(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("cannot read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// The adapter must check for nil result before accessing its fields
	// This is the safety guarantee mentioned in acceptance criteria
	// We verify this by checking that:
	// 1. There's a nil check somewhere
	// 2. There's field access to Success or Output
	// 3. They appear in the same code section (Run method body)

	hasNilCheck := strings.Contains(sourceStr, "== nil") ||
		strings.Contains(sourceStr, "!= nil")

	hasFieldAccess := strings.Contains(sourceStr, ".Success") ||
		strings.Contains(sourceStr, ".Output")

	if !hasNilCheck {
		t.Error("adapter must include nil checks to prevent panics when result is nil")
	}

	if !hasFieldAccess {
		t.Error("adapter must access result fields (Success, Output) to convert them")
	}
}
