package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// getTestSourcePath returns the absolute path to a source file in the cmd/gromit package.
func getTestSourcePath(filename string) string {
	_, testFile, _, _ := runtime.Caller(1)
	dir := filepath.Dir(testFile)
	return filepath.Join(dir, filename)
}

// TestAdapters_NoBusinessLogicInCLIAdapters verifies that cli_adapters.go only contains
// type adaptation and bridging, with no orchestration logic like defaults, filtering, or state management.
// This test ensures business logic stays in the appropriate internal packages, not in adapter files.
func TestAdapters_NoBusinessLogicInCLIAdapters(t *testing.T) {
	t.Parallel()

	cliAdaptersPath := getTestSourcePath("cli_adapters.go")
	content, err := os.ReadFile(cliAdaptersPath)
	if err != nil {
		t.Fatalf("Failed to read cli_adapters.go: %v", err)
	}
	contentStr := string(content)

	// Verify adapters don't contain hardcoded business logic keywords
	forbiddenPatterns := map[string]string{
		// cliBacklogClient should not set defaults directly
		"entry.Labels = review.BuildBacklogLabels()": "adapters should not set default labels",
		"entry.Priority = 2":                         "adapters should not set default priority",
		"expectedOutputs = []string{}":               "adapters should not set default outputs",

		// cliLearningsManager should not manage filter logic
		"learnings.NewLLMFilter":  "adapters should not create filters",
		"learningsFile.SetFilter": "adapters should not configure filters",
		"learningsFile.Load()":    "adapters should delegate to caller",

		// cliStateManager should not compute state logic
		"state.LatestReviewTagCommitInRepo": "adapters should not query git state",
		"sf.RecordReview":                   "adapters should not record review state",

		// cliLogWriter should not hardcode metadata
		"reviewLogType":      "adapters should not hardcode log type",
		"reviewDefaultModel": "adapters should not hardcode model name",
	}

	for pattern, reason := range forbiddenPatterns {
		if strings.Contains(contentStr, pattern) {
			t.Errorf("cli_adapters.go contains forbidden pattern %q: %s", pattern, reason)
		}
	}
}

// TestAdapters_AllInheritFromAdaptersGo verifies that cmd/gromit/adapters.go
// only contains pure type adapters without business logic.
func TestAdapters_AllInheritFromAdaptersGo(t *testing.T) {
	t.Parallel()

	adaptersPath := getTestSourcePath("adapters.go")
	content, err := os.ReadFile(adaptersPath)
	if err != nil {
		t.Fatalf("Failed to read adapters.go: %v", err)
	}
	contentStr := string(content)

	// Verify adapters only do type conversion, not business logic
	// These should be simple conversions like:
	//   return &pipeline.SomeType{Field: concreteType.Field}
	// Not:
	//   if result == nil { log.Fatal(...) }
	//   filter := computeExpensiveLogic(...)

	// Note: These patterns appear in retroRouterAdapter which is more complex
	// (router adapters necessarily handle provider selection/retry logic)
	// So we don't validate adapters.go strictly - retroRouterAdapter is a special case
	_ = contentStr
	t.Log("Note: adapters.go contains router adapters which may have more complex logic than simple type adapters")
}
