package provider

import (
	"strings"
	"testing"
)

// TestIsValidationPassedDetectsMarker verifies that IsValidationPassed()
// detects the VALIDATION_PASSED marker in result output.
// Expected failure: IsValidationPassed() function does not exist in provider package yet
func TestIsValidationPassedDetectsMarker(t *testing.T) {
	tests := []struct {
		name     string
		result   *Result
		expected bool
	}{
		{
			name: "successful validation with VALIDATION_PASSED marker",
			result: &Result{
				Success:  true,
				ExitCode: 0,
				Output:   "All tests passed.\nVALIDATION_PASSED\nCompleted successfully.",
			},
			expected: true,
		},
		{
			name: "failed validation without marker",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Test failed: some error",
			},
			expected: false,
		},
		{
			name: "successful run but no VALIDATION_PASSED marker",
			result: &Result{
				Success:  true,
				ExitCode: 0,
				Output:   "Tests completed but marker missing",
			},
			expected: false,
		},
		{
			name:     "nil result returns false",
			result:   nil,
			expected: false,
		},
		{
			name: "marker in middle of text",
			result: &Result{
				Success: true,
				Output:  "Starting validation... running tests... VALIDATION_PASSED all checks complete.",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidationPassed(tt.result)
			if got != tt.expected {
				t.Errorf("IsValidationPassed() = %v, want %v (output=%q)", got, tt.expected, tt.result.Output)
			}
		})
	}
}

// TestIsScopeTooLargeDetectsStartOfLineMarker verifies that IsScopeTooLarge()
// detects the SCOPE_TOO_LARGE marker only at the start of a line.
// Expected failure: IsScopeTooLarge() function does not exist in provider package yet
func TestIsScopeTooLargeDetectsStartOfLineMarker(t *testing.T) {
	tests := []struct {
		name                string
		result              *Result
		expectedTooLarge    bool
		expectedExplanation string
	}{
		{
			name: "scope too large with explanation",
			result: &Result{
				Success: true,
				Output:  "Analysis:\nSCOPE_TOO_LARGE: This task touches 8 files across 4 packages and requires architectural changes.\n\nMore details...",
			},
			expectedTooLarge:    true,
			expectedExplanation: "This task touches 8 files across 4 packages and requires architectural changes.",
		},
		{
			name: "scope too large at start of output",
			result: &Result{
				Success: true,
				Output:  "SCOPE_TOO_LARGE: Too many files to modify in one bead.\nBreakdown: file1.go, file2.go",
			},
			expectedTooLarge:    true,
			expectedExplanation: "Too many files to modify in one bead.",
		},
		{
			name: "scope acceptable - no marker",
			result: &Result{
				Success: true,
				Output:  "This task looks good. The scope is appropriate for a single bead.",
			},
			expectedTooLarge:    false,
			expectedExplanation: "",
		},
		{
			name: "marker in middle of line - not detected",
			result: &Result{
				Success: true,
				Output:  "The pattern SCOPE_TOO_LARGE: should be at the start of a line to trigger.",
			},
			expectedTooLarge:    false,
			expectedExplanation: "",
		},
		{
			name:                "nil result returns false",
			result:              nil,
			expectedTooLarge:    false,
			expectedExplanation: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTooLarge, gotExplanation := IsScopeTooLarge(tt.result)
			if gotTooLarge != tt.expectedTooLarge {
				t.Errorf("IsScopeTooLarge() tooLarge = %v, want %v", gotTooLarge, tt.expectedTooLarge)
			}
			if tt.expectedTooLarge && !strings.Contains(gotExplanation, tt.expectedExplanation) {
				t.Errorf("IsScopeTooLarge() explanation = %q, want it to contain %q", gotExplanation, tt.expectedExplanation)
			}
		})
	}
}

// TestGetScopeTooLargeBreakdownExtractsFullContent verifies that
// GetScopeTooLargeBreakdown() extracts the full breakdown content after the marker.
// Expected failure: GetScopeTooLargeBreakdown() function does not exist in provider package yet
func TestGetScopeTooLargeBreakdownExtractsFullContent(t *testing.T) {
	tests := []struct {
		name             string
		result           *Result
		expectedContains []string
	}{
		{
			name: "breakdown with multiple paragraphs",
			result: &Result{
				Output: "Analysis:\nSCOPE_TOO_LARGE: This task is too large.\n\nDetailed breakdown:\n- File 1\n- File 2\n- File 3",
			},
			expectedContains: []string{"This task is too large", "Detailed breakdown", "File 1", "File 2", "File 3"},
		},
		{
			name: "breakdown at start of output",
			result: &Result{
				Output: "SCOPE_TOO_LARGE: Task requires 8 files across 3 packages.\nSplit into:\n1. Database layer\n2. API layer\n3. UI layer",
			},
			expectedContains: []string{"Task requires 8 files", "Database layer", "API layer", "UI layer"},
		},
		{
			name: "no marker returns empty",
			result: &Result{
				Output: "This task has appropriate scope.",
			},
			expectedContains: nil,
		},
		{
			name:             "nil result returns empty",
			result:           nil,
			expectedContains: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetScopeTooLargeBreakdown(tt.result)
			if tt.expectedContains == nil {
				if got != "" {
					t.Errorf("GetScopeTooLargeBreakdown() = %q, want empty string", got)
				}
			} else {
				for _, expected := range tt.expectedContains {
					if !strings.Contains(got, expected) {
						t.Errorf("GetScopeTooLargeBreakdown() = %q, want it to contain %q", got, expected)
					}
				}
			}
		})
	}
}

// TestFindStartOfLineMarkerMatchesOnlyAtLineStart verifies that
// findStartOfLineMarker() only matches SCOPE_TOO_LARGE: at the start of a line.
// Expected failure: findStartOfLineMarker() function does not exist in provider package yet
func TestFindStartOfLineMarkerMatchesOnlyAtLineStart(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantIndex   int
		wantMatched bool
	}{
		{
			name:        "marker at very start",
			input:       "SCOPE_TOO_LARGE: explanation here",
			wantIndex:   0,
			wantMatched: true,
		},
		{
			name:        "marker after newline",
			input:       "Some text\nSCOPE_TOO_LARGE: explanation",
			wantIndex:   10,
			wantMatched: true,
		},
		{
			name:        "marker in middle of line",
			input:       "Text SCOPE_TOO_LARGE: more text",
			wantIndex:   -1,
			wantMatched: false,
		},
		{
			name:        "marker after space",
			input:       " SCOPE_TOO_LARGE: explanation",
			wantIndex:   -1,
			wantMatched: false,
		},
		{
			name:        "no marker present",
			input:       "No marker in this text",
			wantIndex:   -1,
			wantMatched: false,
		},
		{
			name:        "multiple occurrences - first at line start",
			input:       "SCOPE_TOO_LARGE: first\nText SCOPE_TOO_LARGE: second",
			wantIndex:   0,
			wantMatched: true,
		},
		{
			name:        "multiple occurrences - first mid-line, second at line start",
			input:       "Text SCOPE_TOO_LARGE: first\nSCOPE_TOO_LARGE: second",
			wantIndex:   28,
			wantMatched: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findStartOfLineMarker(tt.input)
			if tt.wantMatched && got == -1 {
				t.Errorf("findStartOfLineMarker() = -1, want index >= 0")
			}
			if !tt.wantMatched && got != -1 {
				t.Errorf("findStartOfLineMarker() = %d, want -1 (no match)", got)
			}
			if tt.wantMatched && got != tt.wantIndex {
				t.Errorf("findStartOfLineMarker() = %d, want %d", got, tt.wantIndex)
			}
		})
	}
}

// TestValidateCommandsRejectsInvalidCommands verifies that ValidateCommands()
// rejects commands with unsafe patterns like newlines or excessive length.
// Expected failure: ValidateCommands() function does not exist in provider package yet
func TestValidateCommandsRejectsInvalidCommands(t *testing.T) {
	tests := []struct {
		name        string
		commands    []string
		expectError bool
		errorText   string
	}{
		{
			name:        "valid single command",
			commands:    []string{"go test ./..."},
			expectError: false,
		},
		{
			name:        "valid multiple commands",
			commands:    []string{"go test ./...", "go vet ./...", "golangci-lint run"},
			expectError: false,
		},
		{
			name:        "command with newline",
			commands:    []string{"go test\nrm -rf /"},
			expectError: true,
			errorText:   "single line",
		},
		{
			name:        "command with carriage return",
			commands:    []string{"go test\rrm -rf /"},
			expectError: true,
			errorText:   "single line",
		},
		{
			name:        "empty command",
			commands:    []string{"go test", "", "go vet"},
			expectError: true,
			errorText:   "empty",
		},
		{
			name:        "command exceeding length limit",
			commands:    []string{strings.Repeat("a", 1025)},
			expectError: true,
			errorText:   "maximum length",
		},
		{
			name:        "valid command at length limit",
			commands:    []string{strings.Repeat("a", 1024)},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommands(tt.commands)
			if tt.expectError && err == nil {
				t.Errorf("ValidateCommands() error = nil, want error containing %q", tt.errorText)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateCommands() error = %v, want nil", err)
			}
			if tt.expectError && err != nil && !strings.Contains(err.Error(), tt.errorText) {
				t.Errorf("ValidateCommands() error = %q, want error containing %q", err.Error(), tt.errorText)
			}
		})
	}
}

// TestBuildValidationPromptFormatsCommandsCorrectly verifies that
// BuildValidationPrompt() constructs a validation prompt with numbered commands.
// Expected failure: BuildValidationPrompt() function does not exist in provider package yet
func TestBuildValidationPromptFormatsCommandsCorrectly(t *testing.T) {
	tests := []struct {
		name             string
		commands         []string
		workDir          string
		expectedContains []string
	}{
		{
			name:     "single command",
			commands: []string{"go test ./..."},
			workDir:  "/home/user/project",
			expectedContains: []string{
				"VALIDATION_PASSED",
				"VALIDATION_FAILED",
				"1. go test ./...",
				"/home/user/project",
				"numbered commands",
			},
		},
		{
			name:     "multiple commands",
			commands: []string{"go test ./...", "go vet ./...", "golangci-lint run"},
			workDir:  "/tmp/test",
			expectedContains: []string{
				"1. go test ./...",
				"2. go vet ./...",
				"3. golangci-lint run",
				"/tmp/test",
				"VALIDATION_PASSED",
				"VALIDATION_FAILED",
			},
		},
		{
			name:     "commands in code block",
			commands: []string{"make test", "make lint"},
			workDir:  "/workspace",
			expectedContains: []string{
				"```",
				"1. make test",
				"2. make lint",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := BuildValidationPrompt(tt.commands, tt.workDir)
			for _, expected := range tt.expectedContains {
				if !strings.Contains(prompt, expected) {
					t.Errorf("BuildValidationPrompt() missing expected content %q, got: %s", expected, prompt)
				}
			}

			// Verify commands are numbered correctly
			for i, cmd := range tt.commands {
				numberedCmd := strings.TrimSpace(cmd)
				if !strings.Contains(prompt, numberedCmd) {
					t.Errorf("BuildValidationPrompt() missing command %d: %q", i+1, cmd)
				}
			}
		})
	}
}

// TestProviderInterfaceHasValidationHelpers verifies that the Provider interface
// includes IsValidationPassed and IsScopeTooLarge methods.
// Expected failure: Provider interface does not include these methods yet
func TestProviderInterfaceHasValidationHelpers(t *testing.T) {
	// Verify that a concrete provider (CodexProvider) implements these methods
	cp := &CodexProvider{}

	// These method calls should compile when the interface includes them
	_ = cp.IsValidationPassed(&Result{})
	_, _ = cp.IsScopeTooLarge(&Result{})
}

// TestClaudeProviderUsesSharedHelpers verifies that ClaudeProvider's
// IsValidationPassed and IsScopeTooLarge methods delegate to shared helpers.
// Expected failure: ClaudeProvider does not delegate to provider package helpers yet
func TestClaudeProviderUsesSharedHelpers(t *testing.T) {
	cp := &ClaudeProvider{}

	// Test that ClaudeProvider delegates to shared IsValidationPassed
	testResult := &Result{
		Success: true,
		Output:  "All tests passed.\nVALIDATION_PASSED",
	}

	// This should use the shared helper which checks for VALIDATION_PASSED marker
	if !cp.IsValidationPassed(testResult) {
		t.Error("ClaudeProvider.IsValidationPassed() should return true for result with VALIDATION_PASSED marker")
	}

	// Test that ClaudeProvider delegates to shared IsScopeTooLarge
	scopeResult := &Result{
		Success: true,
		Output:  "SCOPE_TOO_LARGE: Task is too large to complete in one bead.",
	}

	tooLarge, explanation := cp.IsScopeTooLarge(scopeResult)
	if !tooLarge {
		t.Error("ClaudeProvider.IsScopeTooLarge() should return true for result with SCOPE_TOO_LARGE marker")
	}
	if !strings.Contains(explanation, "Task is too large") {
		t.Errorf("ClaudeProvider.IsScopeTooLarge() explanation = %q, want it to contain 'Task is too large'", explanation)
	}
}

// TestCodexProviderImplementsValidationHelpers verifies that CodexProvider
// implements IsValidationPassed and IsScopeTooLarge methods using shared helpers.
// Expected failure: CodexProvider does not implement these methods yet
func TestCodexProviderImplementsValidationHelpers(t *testing.T) {
	cp := &CodexProvider{}

	tests := []struct {
		name     string
		result   *Result
		wantPass bool
	}{
		{
			name: "validation passed",
			result: &Result{
				Success: true,
				Output:  "Tests complete.\nVALIDATION_PASSED\nAll checks passed.",
			},
			wantPass: true,
		},
		{
			name: "validation failed",
			result: &Result{
				Success: false,
				Output:  "Tests failed: error occurred",
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cp.IsValidationPassed(tt.result)
			if got != tt.wantPass {
				t.Errorf("CodexProvider.IsValidationPassed() = %v, want %v", got, tt.wantPass)
			}
		})
	}

	// Test IsScopeTooLarge
	scopeTests := []struct {
		name            string
		result          *Result
		wantTooLarge    bool
		wantExplanation string
	}{
		{
			name: "scope too large",
			result: &Result{
				Output: "\nSCOPE_TOO_LARGE: This task requires changes across 6 packages.",
			},
			wantTooLarge:    true,
			wantExplanation: "This task requires changes across 6 packages.",
		},
		{
			name: "scope acceptable",
			result: &Result{
				Output: "Task is well-scoped and ready to implement.",
			},
			wantTooLarge: false,
		},
	}

	for _, tt := range scopeTests {
		t.Run(tt.name, func(t *testing.T) {
			gotTooLarge, gotExplanation := cp.IsScopeTooLarge(tt.result)
			if gotTooLarge != tt.wantTooLarge {
				t.Errorf("CodexProvider.IsScopeTooLarge() tooLarge = %v, want %v", gotTooLarge, tt.wantTooLarge)
			}
			if tt.wantTooLarge && !strings.Contains(gotExplanation, tt.wantExplanation) {
				t.Errorf("CodexProvider.IsScopeTooLarge() explanation = %q, want it to contain %q", gotExplanation, tt.wantExplanation)
			}
		})
	}
}

// TestSharedHelpersMatchClaudeBehavior verifies that the shared provider helpers
// produce identical results to the original claude package functions.
// Expected failure: Shared helpers do not exist yet
func TestSharedHelpersMatchClaudeBehavior(t *testing.T) {
	tests := []struct {
		name   string
		result *Result
	}{
		{
			name: "validation passed",
			result: &Result{
				Success: true,
				Output:  "All tests passed.\nVALIDATION_PASSED\nCompleted successfully.",
			},
		},
		{
			name: "validation failed",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Test failed: some error",
			},
		},
		{
			name: "scope too large at line start",
			result: &Result{
				Success: true,
				Output:  "\nSCOPE_TOO_LARGE: Too many files to modify in one bead.\nBreakdown: file1.go, file2.go",
			},
		},
		{
			name: "scope too large mid-line not detected",
			result: &Result{
				Success: true,
				Output:  "The pattern SCOPE_TOO_LARGE: should be at the start of a line.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IsValidationPassed parity
			providerResult := IsValidationPassed(tt.result)
			claudeResult := claudeIsValidationPassed(tt.result)
			if providerResult != claudeResult {
				t.Errorf("IsValidationPassed() = %v, claude.IsValidationPassed() = %v, want same result",
					providerResult, claudeResult)
			}

			// Test IsScopeTooLarge parity
			providerTooLarge, providerExpl := IsScopeTooLarge(tt.result)
			claudeTooLarge, claudeExpl := claudeIsScopeTooLarge(tt.result)
			if providerTooLarge != claudeTooLarge {
				t.Errorf("IsScopeTooLarge() tooLarge = %v, claude.IsScopeTooLarge() = %v, want same result",
					providerTooLarge, claudeTooLarge)
			}
			if providerExpl != claudeExpl {
				t.Errorf("IsScopeTooLarge() explanation = %q, claude.IsScopeTooLarge() = %q, want same result",
					providerExpl, claudeExpl)
			}

			// Test GetScopeTooLargeBreakdown parity
			providerBreakdown := GetScopeTooLargeBreakdown(tt.result)
			claudeBreakdown := claudeGetScopeTooLargeBreakdown(tt.result)
			if providerBreakdown != claudeBreakdown {
				t.Errorf("GetScopeTooLargeBreakdown() = %q, claude.GetScopeTooLargeBreakdown() = %q, want same result",
					providerBreakdown, claudeBreakdown)
			}
		})
	}
}

// Helper functions to convert Result to claude.Result format for comparison tests
// These simulate calling the original claude package functions

func claudeIsValidationPassed(result *Result) bool {
	if result == nil {
		return false
	}
	return result.Success && strings.Contains(result.Output, "VALIDATION_PASSED")
}

func claudeIsScopeTooLarge(result *Result) (bool, string) {
	if result == nil {
		return false, ""
	}

	idx := claudeFindStartOfLineMarker(result.Output)
	if idx == -1 {
		return false, ""
	}

	const marker = "SCOPE_TOO_LARGE:"
	remaining := result.Output[idx+len(marker):]
	explanation := strings.TrimSpace(remaining)

	if paragraphEnd := strings.Index(explanation, "\n\n"); paragraphEnd != -1 {
		explanation = explanation[:paragraphEnd]
	}

	lines := strings.Split(explanation, "\n")
	var explanationLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			explanationLines = append(explanationLines, trimmed)
		}
	}

	explanation = strings.Join(explanationLines, " ")
	return true, explanation
}

func claudeGetScopeTooLargeBreakdown(result *Result) string {
	if result == nil {
		return ""
	}

	idx := claudeFindStartOfLineMarker(result.Output)
	if idx == -1 {
		return ""
	}

	const marker = "SCOPE_TOO_LARGE:"
	remaining := result.Output[idx+len(marker):]
	breakdown := strings.TrimSpace(remaining)
	return breakdown
}

func claudeFindStartOfLineMarker(s string) int {
	const marker = "SCOPE_TOO_LARGE:"
	start := 0
	for {
		idx := strings.Index(s[start:], marker)
		if idx == -1 {
			return -1
		}
		abs := start + idx
		if abs == 0 || s[abs-1] == '\n' {
			return abs
		}
		start = abs + len(marker)
	}
}
