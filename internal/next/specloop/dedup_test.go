package specloop

import (
	"strings"
	"testing"
)

// TestExtractRootCause_CannotRead tests parsing "cannot read" error format
func TestExtractRootCause_CannotRead(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `cannot read "/path/to/file"`,
			expected: "/path/to/file",
		},
		{
			input:    `cannot read "/home/user/config.json"`,
			expected: "/home/user/config.json",
		},
		{
			input:    `cannot read "file.txt"`,
			expected: "file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractRootCause(tt.input)
			if result != tt.expected {
				t.Errorf("extractRootCause(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractRootCause_FileDoesNotExist tests parsing "file does not exist" error format
func TestExtractRootCause_FileDoesNotExist(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `file "/path/to/file" does not exist`,
			expected: "/path/to/file",
		},
		{
			input:    `file "/etc/config.yaml" does not exist`,
			expected: "/etc/config.yaml",
		},
		{
			input:    `file "missing.txt" does not exist`,
			expected: "missing.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractRootCause(tt.input)
			if result != tt.expected {
				t.Errorf("extractRootCause(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractRootCause_PatternNotFound tests parsing "pattern not found in" error format
func TestExtractRootCause_PatternNotFound(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `pattern "search_term" not found in "/path/to/file"`,
			expected: "/path/to/file:search_term",
		},
		{
			input:    `pattern "TODO" not found in "main.go"`,
			expected: "main.go:TODO",
		},
		{
			input:    `pattern "func Test" not found in "/home/user/test_file.go"`,
			expected: "/home/user/test_file.go:func Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractRootCause(tt.input)
			if result != tt.expected {
				t.Errorf("extractRootCause(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractRootCause_UnknownFormat tests that unknown formats return empty string
func TestExtractRootCause_UnknownFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "some random error message",
			expected: "",
		},
		{
			input:    "error: something went wrong",
			expected: "",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "malformed string without quotes",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractRootCause(tt.input)
			if result != tt.expected {
				t.Errorf("extractRootCause(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractRootCause_SameFileGrouping tests that multiple errors on same file group together
func TestExtractRootCause_SameFileGrouping(t *testing.T) {
	// Test that errors from different formats but same file have consistent grouping key
	cannotReadError := `cannot read "/app/config.json"`
	fileNotExistError := `file "/app/config.json" does not exist`

	cannotReadKey := extractRootCause(cannotReadError)
	fileNotExistKey := extractRootCause(fileNotExistError)

	if cannotReadKey != fileNotExistKey {
		t.Errorf("same file should produce same group key: %q vs %q", cannotReadKey, fileNotExistKey)
	}

	// Both should extract to just the file path
	expectedKey := "/app/config.json"
	if cannotReadKey != expectedKey {
		t.Errorf("extractRootCause(%q) = %q, want %q", cannotReadError, cannotReadKey, expectedKey)
	}
}

// TestExtractRootCause_PatternVsFilePath tests that pattern:path is distinct from just path
func TestExtractRootCause_PatternVsFilePath(t *testing.T) {
	patternError := `pattern "func main" not found in "/app/main.go"`
	cannotReadError := `cannot read "/app/main.go"`

	patternKey := extractRootCause(patternError)
	fileKey := extractRootCause(cannotReadError)

	if patternKey == fileKey {
		t.Errorf("pattern error and file error should have different keys: %q vs %q", patternKey, fileKey)
	}

	if patternKey != "/app/main.go:func main" {
		t.Errorf("pattern error key = %q, want %q", patternKey, "/app/main.go:func main")
	}

	if fileKey != "/app/main.go" {
		t.Errorf("file error key = %q, want %q", fileKey, "/app/main.go")
	}
}

// TestExtractScenarioName tests extracting scenario names from contract failure strings
func TestExtractScenarioName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal case with em-dash delimiter",
			input:    "contract:Happy path — file_exists failed: something",
			expected: "Happy path",
		},
		{
			name:     "scenario name with multiple words",
			input:    "contract:User creates new account — validation failed: email required",
			expected: "User creates new account",
		},
		{
			name:     "missing em-dash delimiter returns full prefix after contract:",
			input:    "contract:No delimiter scenario",
			expected: "No delimiter scenario",
		},
		{
			name:     "empty string after contract: prefix",
			input:    "contract:",
			expected: "",
		},
		{
			name:     "contract prefix only",
			input:    "contract:",
			expected: "",
		},
		{
			name:     "scenario with special characters",
			input:    "contract:Edge case (with parens) — test failed: details",
			expected: "Edge case (with parens)",
		},
		{
			name:     "scenario with numbers",
			input:    "contract:Test scenario 123 — failed: error",
			expected: "Test scenario 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractScenarioName(tt.input)
			if result != tt.expected {
				t.Errorf("extractScenarioName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractDetailFromContract tests extracting the detail part from contract failure strings.
// The separator " — " (space + U+2014 EM DASH + space) is 5 bytes in UTF-8.
// If the byte offset is wrong (e.g., +4 instead of +5), the returned string will have a leading space.
func TestExtractDetailFromContract(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal case",
			input:    `contract:Happy path — file_exists failed: cannot read "config.json"`,
			expected: `cannot read "config.json"`,
		},
		{
			name:     "multi-word assertion",
			input:    `contract:Scenario — file_contains failed: pattern "foo" not found in "bar.go"`,
			expected: `pattern "foo" not found in "bar.go"`,
		},
		{
			name:     "no delimiter",
			input:    "contract:No delimiter",
			expected: "",
		},
		{
			name:     "no failed: suffix",
			input:    "contract:Scenario — assertion without failed marker",
			expected: "",
		},
		{
			name:     "detail with colon",
			input:    `contract:Scenario — file_exists failed: file "foo.go" does not exist`,
			expected: `file "foo.go" does not exist`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDetailFromContract(tt.input)
			if result != tt.expected {
				t.Errorf("extractDetailFromContract(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestDeduplicateFailures covers all deduplication scenarios
func TestDeduplicateFailures(t *testing.T) {
	t.Run("empty input returns empty slice", func(t *testing.T) {
		result := DeduplicateFailures([]string{})
		if len(result) != 0 {
			t.Errorf("DeduplicateFailures([]) = %v, want []", result)
		}
	})

	t.Run("single contract failure kept as-is", func(t *testing.T) {
		input := []string{
			"contract:Happy path — file_exists failed: cannot read \"config.json\"",
		}
		result := DeduplicateFailures(input)
		if len(result) != 1 {
			t.Errorf("single contract failure: got %d results, want 1", len(result))
		}
		if result[0] != input[0] {
			t.Errorf("single contract failure: got %q, want %q", result[0], input[0])
		}
	})

	t.Run("multiple contract failures from same missing file collapsed into summary with count and scenario list", func(t *testing.T) {
		input := []string{
			"contract:Setup database — file_exists failed: cannot read \"app/migrations/001.sql\"",
			"contract:Create user — file_exists failed: cannot read \"app/migrations/001.sql\"",
			"contract:Delete user — file_exists failed: cannot read \"app/migrations/001.sql\"",
		}
		result := DeduplicateFailures(input)
		if len(result) != 1 {
			t.Errorf("same missing file: got %d results, want 1", len(result))
		}
		// Should contain all scenario names and the error details (normalized format)
		if !containsSubstring(result[0], "Setup database") ||
			!containsSubstring(result[0], "Create user") ||
			!containsSubstring(result[0], "Delete user") ||
			!containsSubstring(result[0], "file") ||
			!containsSubstring(result[0], "does not exist") ||
			!containsSubstring(result[0], "app/migrations/001.sql") ||
			!containsSubstring(result[0], "3") { // Should show count
			t.Errorf("same missing file summary missing expected content: %q", result[0])
		}
	})

	t.Run("failures from different files remain separate", func(t *testing.T) {
		input := []string{
			"contract:Test 1 — file_exists failed: cannot read \"file1.txt\"",
			"contract:Test 1 — file_exists failed: cannot read \"file2.txt\"",
			"contract:Test 2 — file_exists failed: cannot read \"file1.txt\"",
		}
		result := DeduplicateFailures(input)
		if len(result) != 2 {
			t.Errorf("different files: got %d results, want 2", len(result))
		}
		// Should have summaries for both files
		hasFile1 := false
		hasFile2 := false
		for _, r := range result {
			if containsSubstring(r, "file1.txt") {
				hasFile1 = true
			}
			if containsSubstring(r, "file2.txt") {
				hasFile2 = true
			}
		}
		if !hasFile1 || !hasFile2 {
			t.Errorf("different files: missing file summaries, got %v", result)
		}
	})

	t.Run("mixed contract and non-contract failures with correct ordering", func(t *testing.T) {
		input := []string{
			"contract:Scenario 1 — file_exists failed: cannot read \"file.txt\"",
			"compile error: undefined variable",
			"contract:Scenario 2 — file_exists failed: cannot read \"file.txt\"",
			"test failure: assertion failed",
		}
		result := DeduplicateFailures(input)
		// Should have: compile error, test failure, deduped contracts for file.txt
		if len(result) != 3 {
			t.Errorf("mixed failures: got %d results, want 3", len(result))
		}
		// First two results should be non-contract failures
		if result[0] != "compile error: undefined variable" {
			t.Errorf("mixed failures: result[0] should be compile error, got %q", result[0])
		}
		if result[1] != "test failure: assertion failed" {
			t.Errorf("mixed failures: result[1] should be test failure, got %q", result[1])
		}
		// Third result should contain both contract scenarios
		if !containsSubstring(result[2], "Scenario 1") ||
			!containsSubstring(result[2], "Scenario 2") {
			t.Errorf("mixed failures: result[2] should have deduplicated contracts, got %q", result[2])
		}
	})

	t.Run("pattern-not-found failures deduplicated by path:pattern", func(t *testing.T) {
		input := []string{
			"contract:Scenario 1 — pattern_search failed: pattern \"func main\" not found in \"main.go\"",
			"contract:Scenario 2 — pattern_search failed: pattern \"func main\" not found in \"main.go\"",
		}
		result := DeduplicateFailures(input)
		if len(result) != 1 {
			t.Errorf("pattern-not-found dedup: got %d results, want 1", len(result))
		}
		if !containsSubstring(result[0], "Scenario 1") ||
			!containsSubstring(result[0], "Scenario 2") ||
			!containsSubstring(result[0], "main.go") ||
			!containsSubstring(result[0], "func main") ||
			!containsSubstring(result[0], "2") { // count
			t.Errorf("pattern-not-found summary missing expected content: %q", result[0])
		}
	})

	t.Run("persistent-failure hint lines pass through unchanged", func(t *testing.T) {
		input := []string{
			"contract:Test 1 — failed: cannot read \"file.txt\"",
			"persistent-failure: contract:Test 1 has failed 3 consecutive cycles — may indicate a bad test specification",
			"contract:Test 1 — failed: cannot read \"file.txt\"",
			"persistent-failure: contract:Test 1 has failed 3 consecutive cycles — may indicate a bad test specification",
		}
		result := DeduplicateFailures(input)
		// Persistent-failure hints should pass through unchanged
		var persistentFailureCount int
		for _, r := range result {
			if containsSubstring(r, "persistent-failure:") {
				persistentFailureCount++
			}
		}
		if persistentFailureCount < 2 {
			t.Errorf("persistent-failure hints: expected at least 2 hints to pass through, got %d in %v", persistentFailureCount, result)
		}
	})
}

// containsSubstring is a test helper to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestDeduplicateFailures_IntegrationWithFailureHistory simulates the full replan handler flow:
// extract keys → update failure history → annotate persistent hints → deduplicate.
// Verifies that failure history sees all 5 original granular failures (3 unique scenario keys),
// persistent hints are generated correctly, and the final deduplicated output collapses
// contract failures while preserving hints.
func TestDeduplicateFailures_IntegrationWithFailureHistory(t *testing.T) {
	// Cycle 1: 5 granular failures (3 contract failures on same missing file, 2 non-contract failures)
	cycle1Failures := []string{
		"contract:Setup database — file_exists failed: cannot read \"migrations/001.sql\"",
		"contract:Create user — file_exists failed: cannot read \"migrations/001.sql\"",
		"contract:Delete user — file_exists failed: cannot read \"migrations/001.sql\"",
		"compile error: undefined variable x",
		"test failure: assertion failed on line 42",
	}

	// Extract contract failure keys for failure history tracking
	contractKeys1 := ExtractContractFailureKeys(cycle1Failures)
	if len(contractKeys1) != 3 {
		t.Errorf("cycle 1: expected 3 contract keys, got %d", len(contractKeys1))
	}

	// Initialize failure history
	failureHistory := make(map[string]int)

	// Cycle 1: update failure history
	UpdateFailureHistory(failureHistory, contractKeys1)
	if len(failureHistory) != 3 {
		t.Errorf("after cycle 1: expected 3 keys in history, got %d", len(failureHistory))
	}

	// Cycle 2: same failures repeat
	cycle2Failures := []string{
		"contract:Setup database — file_exists failed: cannot read \"migrations/001.sql\"",
		"contract:Create user — file_exists failed: cannot read \"migrations/001.sql\"",
		"contract:Delete user — file_exists failed: cannot read \"migrations/001.sql\"",
	}
	contractKeys2 := ExtractContractFailureKeys(cycle2Failures)
	UpdateFailureHistory(failureHistory, contractKeys2)

	// Cycle 3: same failures repeat again
	contractKeys3 := ExtractContractFailureKeys(cycle2Failures) // Same failures
	UpdateFailureHistory(failureHistory, contractKeys3)

	// Verify failure history after 3 cycles
	if len(failureHistory) != 3 {
		t.Errorf("after 3 cycles: expected 3 keys in history, got %d", len(failureHistory))
	}

	// All 3 contract scenarios should have count 3 after 3 updates
	for key, count := range failureHistory {
		if count != 3 {
			t.Errorf("expected count 3 for key %q, got %d", key, count)
		}
	}

	// Now simulate cycle 3 failures with persistent hints added
	// Generate persistent-failure hints for failures with count >= 3
	cycle3FailuresWithHints := AnnotateWithPersistentHints(cycle2Failures, failureHistory, 3)

	// Add non-contract failures (they don't get hints, but still appear)
	cycle3FailuresWithHints = append(cycle3FailuresWithHints, "compile error: undefined variable x")
	cycle3FailuresWithHints = append(cycle3FailuresWithHints, "test failure: assertion failed on line 42")

	// Verify persistent hints were added
	var persistentHintCount int
	for _, f := range cycle3FailuresWithHints {
		if strings.HasPrefix(f, "persistent-failure:") {
			persistentHintCount++
		}
	}
	if persistentHintCount != 3 {
		t.Errorf("expected 3 persistent hints, got %d", persistentHintCount)
	}

	// Now deduplicate the final failures with hints
	dedupResult := DeduplicateFailures(cycle3FailuresWithHints)

	// Verify deduplication results
	// Expected:
	// - 1 deduplicated contract summary (3 failures on same file collapsed to 1)
	// - 3 persistent-failure hints
	// - 2 non-contract failures

	var contractCount int
	var finalPersistentHintCount int
	var nonContractCount int

	for _, r := range dedupResult {
		if strings.HasPrefix(r, "persistent-failure:") {
			finalPersistentHintCount++
		} else if strings.HasPrefix(r, "contract:") || strings.Contains(r, "contract assertions failed") {
			contractCount++
		} else if r != "" {
			nonContractCount++
		}
	}

	if contractCount != 1 {
		t.Errorf("expected 1 deduplicated contract result, got %d", contractCount)
	}

	if finalPersistentHintCount != 3 {
		t.Errorf("expected 3 persistent hints in final result, got %d", finalPersistentHintCount)
	}

	if nonContractCount != 2 {
		t.Errorf("expected 2 non-contract failures in final result, got %d. Results: %v", nonContractCount, dedupResult)
	}

	if len(dedupResult) != 6 {
		t.Errorf("expected 6 total results (1 contract summary + 3 hints + 2 non-contract), got %d: %v", len(dedupResult), dedupResult)
	}

	// Verify the deduplicated contract summary contains all scenario names
	var contractSummary string
	for _, r := range dedupResult {
		if strings.Contains(r, "contract assertions failed") {
			contractSummary = r
			break
		}
	}
	if contractSummary == "" {
		t.Errorf("deduplicated contract summary not found in results: %v", dedupResult)
	} else if !containsSubstring(contractSummary, "Setup database") ||
		!containsSubstring(contractSummary, "Create user") ||
		!containsSubstring(contractSummary, "Delete user") ||
		!containsSubstring(contractSummary, "3") { // count
		t.Errorf("deduplicated contract summary missing expected content: %q", contractSummary)
	}

	// Verify hints contain expected scenario keys
	hintContent := strings.Join(dedupResult, " ")
	if !strings.Contains(hintContent, "contract:Setup database") ||
		!strings.Contains(hintContent, "contract:Create user") ||
		!strings.Contains(hintContent, "contract:Delete user") {
		t.Errorf("hints missing expected scenario keys in: %v", dedupResult)
	}
}

// ============= Scenario-based tests (consolidated from separate files) =============

func TestScenario_MultipleContractFailuresFromSameMissingFile(t *testing.T) {
	// Seed: 5 contract failure entries all caused by the same missing file.
	// Two different error formats ("does not exist" and "cannot read ... no such file")
	// share the same root cause — the file path.
	failures := []string{
		`contract:Happy path — file_exists failed: file "internal/next/specloop/stages/write_scenario_tests.go" does not exist`,
		`contract:Happy path — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": open .../write_scenario_tests.go: no such file or directory`,
		`contract:Self-repair succeeds — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": open .../write_scenario_tests.go: no such file or directory`,
		`contract:Self-repair fails — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": open .../write_scenario_tests.go: no such file or directory`,
		`contract:Replan preserves — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": open .../write_scenario_tests.go: no such file or directory`,
	}

	// Invoke
	result := DeduplicateFailures(failures)

	// Assert: all 5 entries collapse into a single summary
	if len(result) != 1 {
		t.Fatalf("expected 1 deduplicated result, got %d: %v", len(result), result)
	}

	want := `5 contract assertions failed: file "internal/next/specloop/stages/write_scenario_tests.go" does not exist (scenarios: Happy path, Self-repair succeeds, Self-repair fails, Replan preserves)`
	if result[0] != want {
		t.Errorf("unexpected summary:\n got: %s\nwant: %s", result[0], want)
	}
}

func TestScenario_DeduplicateFailures_DifferentFilesRemainSeparate(t *testing.T) {
	// Seed: two contract failures referencing different files
	input := []string{
		`contract:Happy path — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": no such file`,
		`contract:Scenario test fails — file_contains failed: cannot read "internal/next/runstore/types.go": no such file`,
	}

	// Invoke
	result := DeduplicateFailures(input)

	// Assert: both entries remain separate
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(result), result)
	}

	hasFirst := false
	hasSecond := false
	for _, r := range result {
		if containsSubstring(r, "write_scenario_tests.go") {
			hasFirst = true
		}
		if containsSubstring(r, "types.go") {
			hasSecond = true
		}
	}

	if !hasFirst {
		t.Error("expected output to contain write_scenario_tests.go failure")
	}
	if !hasSecond {
		t.Error("expected output to contain types.go failure")
	}
}

func TestScenario_MixedContractAndTestFailures(t *testing.T) {
	// Mixed contract and test failures: Test consolidation of contract and non-contract failures
	// Seed: 3 contract failures (same root cause) + 2 test/check failures
	failures := []string{
		`contract:A — file_contains failed: cannot read "stages/write_scenario_tests.go": file not found`,
		`contract:B — file_contains failed: cannot read "stages/write_scenario_tests.go": file not found`,
		`contract:C — file_contains failed: cannot read "stages/write_scenario_tests.go": file not found`,
		`always-run check "unit-tests" failed: --- FAIL: TestAdd`,
		`always-run check "vet" failed: pattern ./...: directory prefix . does not contain main module`,
	}

	// Invoke
	result := DeduplicateFailures(failures)

	// Assert: exactly 3 entries
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(result), result)
	}

	// Assert: first two entries are the non-contract (test/check) failures, unchanged
	if result[0] != `always-run check "unit-tests" failed: --- FAIL: TestAdd` {
		t.Errorf("result[0]: expected unit-tests failure unchanged, got %q", result[0])
	}
	if result[1] != `always-run check "vet" failed: pattern ./...: directory prefix . does not contain main module` {
		t.Errorf("result[1]: expected vet failure unchanged, got %q", result[1])
	}

	// Assert: third entry is the collapsed contract summary
	summary := result[2]
	if !strings.Contains(summary, "3") {
		t.Errorf("contract summary should contain count 3, got %q", summary)
	}
	if !strings.Contains(summary, "A") {
		t.Errorf("contract summary should mention scenario A, got %q", summary)
	}
	if !strings.Contains(summary, "B") {
		t.Errorf("contract summary should mention scenario B, got %q", summary)
	}
	if !strings.Contains(summary, "C") {
		t.Errorf("contract summary should mention scenario C, got %q", summary)
	}
	if !strings.Contains(summary, "stages/write_scenario_tests.go") {
		t.Errorf("contract summary should mention the file path, got %q", summary)
	}

	// Assert: test failures are never grouped or modified
	for _, r := range result {
		if strings.HasPrefix(r, "always-run") {
			// Verify these are exact copies of the originals
			if r != `always-run check "unit-tests" failed: --- FAIL: TestAdd` &&
				r != `always-run check "vet" failed: pattern ./...: directory prefix . does not contain main module` {
				t.Errorf("non-contract failure was modified: %q", r)
			}
		}
	}
}

func TestScenario_SingleContractFailure(t *testing.T) {
	// Single contract failure: Verify single contract failure is kept unchanged
	// Seed
	input := []string{
		`contract:Happy path — file_exists failed: file "write_scenario_tests.go" does not exist`,
	}

	// Invoke
	result := DeduplicateFailures(input)

	// Assert
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(result), result)
	}
	if result[0] != input[0] {
		t.Errorf("expected original string unchanged:\n  got:  %q\n  want: %q", result[0], input[0])
	}
}

func TestScenario_FailureHistoryStillWorks(t *testing.T) {
	// Failure history still works: Deduplication is a display optimization, history extraction unchanged
	// Seed: 5 contract failures from 3 scenarios
	originalFailures := []string{
		`contract:Happy path — file_exists failed: cannot read "config.json"`,
		`contract:Happy path — pattern_search failed: pattern "func main" not found in "main.go"`,
		`contract:Happy path — file_exists failed: file "output.txt" does not exist`,
		`contract:Self-repair succeeds — file_exists failed: cannot read "config.json"`,
		`contract:Self-repair fails — file_exists failed: cannot read "config.json"`,
	}

	// Invoke: extract keys from original (pre-dedup) failures, then dedup separately
	contractKeys := ExtractContractFailureKeys(originalFailures)
	dedupedFailures := DeduplicateFailures(originalFailures)

	// Assert: ExtractContractFailureKeys returns 5 raw keys (one per failure line)
	if len(contractKeys) != 5 {
		t.Fatalf("expected 5 contract keys from original failures, got %d: %v", len(contractKeys), contractKeys)
	}

	// Assert: dedup reduces the output (fewer than 5 entries)
	if len(dedupedFailures) >= 5 {
		t.Errorf("expected dedup to reduce failures below 5, got %d: %v", len(dedupedFailures), dedupedFailures)
	}

	// Assert: failure history correctly tracks 3 unique scenario keys
	failureHistory := make(map[string]int)
	UpdateFailureHistory(failureHistory, contractKeys)

	expectedKeys := map[string]bool{
		"contract:Happy path":            true,
		"contract:Self-repair succeeds":  true,
		"contract:Self-repair fails":     true,
	}

	if len(failureHistory) != 3 {
		t.Fatalf("expected 3 keys in failure history, got %d: %v", len(failureHistory), failureHistory)
	}

	for key := range expectedKeys {
		count, ok := failureHistory[key]
		if !ok {
			t.Errorf("expected key %q in failure history, not found", key)
			continue
		}
		if count != 1 {
			t.Errorf("expected count 1 for key %q, got %d", key, count)
		}
	}

	// Assert: no unexpected keys in history
	for key := range failureHistory {
		if !expectedKeys[key] {
			t.Errorf("unexpected key %q in failure history", key)
		}
	}

	// Assert: dedup did not destroy original data — we can still extract the same keys
	// after dedup has already been called (original slice is unchanged)
	contractKeysAfter := ExtractContractFailureKeys(originalFailures)
	if len(contractKeysAfter) != 5 {
		t.Errorf("original failures mutated by dedup: expected 5 keys, got %d", len(contractKeysAfter))
	}
}

func TestScenario_FileContainsFailuresDeduplicatedByPathPattern(t *testing.T) {
	// Seed: 3 contract failures where file exists but pattern not found,
	// all referencing the same file and pattern from different scenarios.
	input := []string{
		`contract:Happy path — file_contains failed: pattern "func RunScenarioTests" not found in "internal/next/specloop/stages/write_scenario_tests.go"`,
		`contract:Self-repair succeeds — file_contains failed: pattern "func RunScenarioTests" not found in "internal/next/specloop/stages/write_scenario_tests.go"`,
		`contract:Self-repair fails — file_contains failed: pattern "func RunScenarioTests" not found in "internal/next/specloop/stages/write_scenario_tests.go"`,
	}

	// Invoke
	result := DeduplicateFailures(input)

	// Assert: 3 entries collapsed into 1
	if len(result) != 1 {
		t.Fatalf("expected 1 deduplicated result, got %d: %v", len(result), result)
	}

	expected := `3 contract assertions failed: pattern "func RunScenarioTests" not found in "internal/next/specloop/stages/write_scenario_tests.go" (scenarios: Happy path, Self-repair succeeds, Self-repair fails)`
	if result[0] != expected {
		t.Errorf("unexpected result:\n got: %s\nwant: %s", result[0], expected)
	}
}

func TestScenario_EmptyReplanContext(t *testing.T) {
	// Empty replan context: Deduplication handles empty failure lists correctly
	// Seed: an empty failures slice
	failures := []string{}

	// Invoke
	result := DeduplicateFailures(failures)

	// Assert
	if result == nil {
		t.Fatal("DeduplicateFailures([]) returned nil, want empty slice")
	}
	if len(result) != 0 {
		t.Errorf("DeduplicateFailures([]) = %v (len %d), want empty slice", result, len(result))
	}
}

// TestScenario_UnrecognizedAssertionTypesRemainUngrouped verifies that contract failures
// with unrecognized assertion types (e.g., file_not_contains, file_not_modified) do NOT
// get collapsed into a single summary but remain as individual ungrouped failures.
// This addresses the issue where unrecognized error formats all return empty string as
// their grouping key, causing them to be incorrectly grouped together.
func TestScenario_UnrecognizedAssertionTypesRemainUngrouped(t *testing.T) {
	// Seed: Two contract failures with unrecognized assertion types
	// (file_not_contains and file_not_modified are not recognized patterns)
	failures := []string{
		`contract:Scenario A — file_not_contains failed: expected pattern absent but found it`,
		`contract:Scenario B — file_not_modified failed: file was modified unexpectedly`,
	}

	// Invoke
	result := DeduplicateFailures(failures)

	// Assert: unrecognized patterns should remain separate (not collapsed into summary)
	// Expected: 2 results (one per failure, unchanged)
	// Current (broken): 1 result (both collapsed into a summary with count 2)
	if len(result) != 2 {
		t.Fatalf("expected 2 ungrouped results, got %d: %v", len(result), result)
	}

	// Assert: both failures should remain as-is (not summarized)
	if result[0] != failures[0] {
		t.Errorf("result[0] should be unchanged:\n  got: %q\n want: %q", result[0], failures[0])
	}
	if result[1] != failures[1] {
		t.Errorf("result[1] should be unchanged:\n  got: %q\n want: %q", result[1], failures[1])
	}
}

// TestScenario_MixedRecognizedAndUnrecognizedPatterns verifies that when failures mix
// recognized and unrecognized patterns, recognized ones are deduplicated while
// unrecognized ones appear ungrouped alongside them (not collapsed with each other).
func TestScenario_MixedRecognizedAndUnrecognizedPatterns(t *testing.T) {
	// Seed: Mix of recognized patterns (same file) + multiple unrecognized patterns
	// The recognized ones should deduplicate into one summary.
	// The unrecognized ones should NOT be grouped with each other.
	failures := []string{
		// Recognized: same file, same error - should collapse to 1 summary
		`contract:Scenario A — file_exists failed: cannot read "config.json"`,
		`contract:Scenario B — file_exists failed: cannot read "config.json"`,
		// Unrecognized: different assertion types - should each remain separate
		`contract:Scenario C — file_not_contains failed: expected pattern absent but found it`,
		`contract:Scenario D — file_not_modified failed: file was modified unexpectedly`,
	}

	// Invoke
	result := DeduplicateFailures(failures)

	// Assert: expected 3 results
	// - 1 deduplicated summary for the two "cannot read" failures (same file)
	// - 2 ungrouped results for the unrecognized failures (Scenario C and D remain separate)
	// Current (broken): 2 results (1 summary for recognized, 1 summary for both unrecognized)
	if len(result) != 3 {
		t.Fatalf("expected 3 results (1 deduplicated + 2 ungrouped), got %d: %v", len(result), result)
	}

	// Assert: one result should be the deduplicated summary for config.json
	foundDeduped := false
	foundScenarioC := false
	foundScenarioD := false

	for _, r := range result {
		// Check for deduplicated summary for recognized failures
		if strings.Contains(r, "2 contract assertions failed") &&
			strings.Contains(r, "config.json") &&
			strings.Contains(r, "Scenario A") &&
			strings.Contains(r, "Scenario B") {
			foundDeduped = true
		}
		// Check for ungrouped unrecognized failures (originals unchanged)
		if r == failures[2] {
			foundScenarioC = true
		}
		if r == failures[3] {
			foundScenarioD = true
		}
	}

	if !foundDeduped {
		t.Errorf("expected deduplicated summary for config.json in results: %v", result)
	}
	if !foundScenarioC {
		t.Errorf("expected ungrouped Scenario C failure in results: %v", result)
	}
	if !foundScenarioD {
		t.Errorf("expected ungrouped Scenario D failure in results: %v", result)
	}
}
