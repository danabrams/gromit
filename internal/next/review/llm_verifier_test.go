package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// contains is a helper to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestLLMFindingVerifier_ReturnsConfirmedWithFileUnreadableOnReadError tests that
// Verify returns DispositionConfirmed with 'file unreadable — retaining finding' reason when file cannot be read
func TestLLMFindingVerifier_ReturnsConfirmedWithFileUnreadableOnReadError(t *testing.T) {
	invoker := &mockInvoker{}
	verifier := NewLLMFindingVerifier(invoker)

	finding := Finding{
		File: "nonexistent.go",
		Line: 10,
	}

	result, err := verifier.Verify(context.Background(), finding, "/tmp")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != DispositionConfirmed {
		t.Errorf("expected Disposition %q, got %q", DispositionConfirmed, result.Disposition)
	}
	if result.Reason != "file unreadable — retaining finding" {
		t.Errorf("expected reason %q, got %q", "file unreadable — retaining finding", result.Reason)
	}
}

// TestLLMFindingVerifier_ReadsFileContextCorrectly tests that Verify reads
// the correct line range (max(0,line-5) to min(EOF,line+10))
func TestLLMFindingVerifier_ReadsFileContextCorrectly(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")

	// Create a test file with numbered lines
	content := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\nline 11\nline 12\nline 13\nline 14\nline 15\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	invoker := &mockInvoker{
		result: &provider.Result{
			Output: "confirmed",
		},
	}
	verifier := NewLLMFindingVerifier(invoker)

	finding := Finding{
		File: "test.go",
		Line: 10,
	}

	result, err := verifier.Verify(context.Background(), finding, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invoker.calledWithPrompt == "" {
		t.Error("expected invoker to be called with prompt")
	}

	// Verify the excerpt contains the correct line range (lines 5-15 for line 10)
	if !contains(result.FileExcerpt, "line 5") {
		t.Errorf("expected excerpt to contain 'line 5', got: %s", result.FileExcerpt)
	}
	if !contains(result.FileExcerpt, "line 15") {
		t.Errorf("expected excerpt to contain 'line 15', got: %s", result.FileExcerpt)
	}
	if contains(result.FileExcerpt, "line 4") {
		t.Error("expected excerpt to NOT contain 'line 4'")
	}
	if contains(result.FileExcerpt, "line 16") {
		t.Error("expected excerpt to NOT contain 'line 16'")
	}
}

// TestLLMFindingVerifier_LineRangeNearFileStart tests line range calculation
// for lines near the start of the file (min should clamp to 0)
func TestLLMFindingVerifier_LineRangeNearFileStart(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")

	// Create a test file
	content := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\nline 11\nline 12\nline 13\nline 14\nline 15\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	invoker := &mockInvoker{
		result: &provider.Result{
			Output: "confirmed",
		},
	}
	verifier := NewLLMFindingVerifier(invoker)

	finding := Finding{
		File: "test.go",
		Line: 2,
	}

	result, err := verifier.Verify(context.Background(), finding, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// For line 2, should read max(0, 2-5) = 0 to min(15, 2+10) = 12
	// So should include line 1 but not line 13+
	if !contains(result.FileExcerpt, "line 1") {
		t.Error("expected excerpt to contain 'line 1' (start should be clamped to 0)")
	}
	if !contains(result.FileExcerpt, "line 12") {
		t.Error("expected excerpt to contain 'line 12'")
	}
	if contains(result.FileExcerpt, "line 13") {
		t.Error("expected excerpt to NOT contain 'line 13'")
	}
}

// TestLLMFindingVerifier_ParsesDispositionFromFirstWord tests that
// Verify parses the disposition from the first word of the LLM output
func TestLLMFindingVerifier_ParsesDispositionFromFirstWord(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("test code"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		output      string
		expected    VerifierDisposition
		expectParse bool
	}{
		{
			name:        "confirmed with explanation",
			output:      "confirmed this is a real issue",
			expected:    DispositionConfirmed,
			expectParse: false,
		},
		{
			name:        "downgraded",
			output:      "downgraded not as bad as stated",
			expected:    DispositionDowngraded,
			expectParse: false,
		},
		{
			name:        "fixed",
			output:      "fixed in main branch",
			expected:    DispositionFixed,
			expectParse: false,
		},
		{
			name:        "unknown disposition",
			output:      "unknown not a valid word",
			expected:    DispositionConfirmed,
			expectParse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &mockInvoker{
				result: &provider.Result{
					Output: tt.output,
				},
			}
			verifier := NewLLMFindingVerifier(invoker)

			finding := Finding{
				File: "test.go",
				Line: 1,
			}

			result, err := verifier.Verify(context.Background(), finding, tmpDir)

			// Per spec: parse errors return nil error with disposition encoded in result
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectParse {
				if result.Disposition != DispositionConfirmed {
					t.Errorf("expected Disposition %q on parse error, got %q", DispositionConfirmed, result.Disposition)
				}
				if result.Reason != "parse error — retaining finding" {
					t.Errorf("expected reason %q, got %q", "parse error — retaining finding", result.Reason)
				}
			} else {
				if result.Disposition != tt.expected {
					t.Errorf("expected Disposition %q, got %q", tt.expected, result.Disposition)
				}
			}
		})
	}
}

// TestLLMFindingVerifier_PromptIncludesDescription tests that the verifier prompt
// includes the finding's description, severity, and the structured disposition guidance
func TestLLMFindingVerifier_PromptIncludesDescription(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("test code"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	invoker := &mockInvoker{
		result: &provider.Result{
			Output: "confirmed this is a real issue",
		},
	}
	verifier := NewLLMFindingVerifier(invoker)

	finding := Finding{
		File:        "test.go",
		Line:        1,
		Description: "bare Acceptance Criteria marker found",
		Severity:    SeverityError,
	}

	result, err := verifier.Verify(context.Background(), finding, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if invoker.calledWithPrompt == "" {
		t.Fatal("expected invoker to be called with prompt")
	}

	prompt := invoker.calledWithPrompt

	// Verify the prompt includes the description
	if !contains(prompt, "bare Acceptance Criteria marker found") {
		t.Errorf("prompt should include description, got: %s", prompt)
	}

	// Verify the prompt includes the severity string
	severityStr := finding.Severity.String()
	if !contains(prompt, severityStr) {
		t.Errorf("prompt should include severity %q, got: %s", severityStr, prompt)
	}

	// Verify the prompt includes 'downgraded' in the structured guidance
	if !contains(prompt, "downgraded") {
		t.Errorf("prompt should include 'downgraded' in structured guidance, got: %s", prompt)
	}

	// Verify the disposition was parsed correctly
	if result.Disposition != DispositionConfirmed {
		t.Errorf("expected Disposition %q, got %q", DispositionConfirmed, result.Disposition)
	}
}

// TestLLMFindingVerifier_InvokeError tests that Verify returns DispositionConfirmed
// with 'invoke error — retaining finding' reason when invoker.Invoke returns a non-nil error.
// Per spec, invoke errors return nil error with the fail-safe disposition encoded in the result.
func TestLLMFindingVerifier_InvokeError(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("test code"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	invoker := &mockInvoker{
		err: fmt.Errorf("invoker failure"),
	}
	verifier := NewLLMFindingVerifier(invoker)

	finding := Finding{
		File: "test.go",
		Line: 1,
	}

	result, err := verifier.Verify(context.Background(), finding, tmpDir)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Disposition != DispositionConfirmed {
		t.Errorf("expected Disposition %q, got %q", DispositionConfirmed, result.Disposition)
	}
	if result.Reason != "invoke error — retaining finding" {
		t.Errorf("expected reason %q, got %q", "invoke error — retaining finding", result.Reason)
	}
}

// TestLLMFindingVerifier_NilResult tests that Verify returns DispositionConfirmed
// with 'invoke error — retaining finding' reason when invoker.Invoke returns nil result
// with nil error. Per spec, this is treated as an invoke error (fail-safe).
func TestLLMFindingVerifier_NilResult(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("test code"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	invoker := &mockInvoker{
		result: nil,
		err:    nil,
	}
	verifier := NewLLMFindingVerifier(invoker)

	finding := Finding{
		File: "test.go",
		Line: 1,
	}

	result, err := verifier.Verify(context.Background(), finding, tmpDir)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Disposition != DispositionConfirmed {
		t.Errorf("expected Disposition %q, got %q", DispositionConfirmed, result.Disposition)
	}
	if result.Reason != "invoke error — retaining finding" {
		t.Errorf("expected reason %q, got %q", "invoke error — retaining finding", result.Reason)
	}
}

// TestLLMFindingVerifier_EmptyResponse tests that Verify returns DispositionConfirmed
// with 'parse error — retaining finding' reason when the invoker returns a Result with
// empty or whitespace-only Output.
func TestLLMFindingVerifier_EmptyResponse(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("test code"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	invoker := &mockInvoker{
		result: &provider.Result{
			Output: "   ",
		},
	}
	verifier := NewLLMFindingVerifier(invoker)

	finding := Finding{
		File: "test.go",
		Line: 1,
	}

	result, err := verifier.Verify(context.Background(), finding, tmpDir)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Disposition != DispositionConfirmed {
		t.Errorf("expected Disposition %q, got %q", DispositionConfirmed, result.Disposition)
	}
	if result.Reason != "parse error — retaining finding" {
		t.Errorf("expected reason %q, got %q", "parse error — retaining finding", result.Reason)
	}
}
