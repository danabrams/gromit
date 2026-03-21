package stages

import (
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
)

func TestScenario_SpecificityWarningString_DeduplicatesFormatting(t *testing.T) {
	// Seed: a set of specificity warnings that both the retry-prompt block
	// and event-emission block would receive.
	warnings := []contract.SpecificityWarning{
		{
			ScenarioName: "add-works",
			AssertionIdx: 0,
			Pattern:      "ModelTier",
			Path:         "calc/calc.go",
			Reason:       "single exported identifier — ambiguous if file contains multiple types",
		},
		{
			ScenarioName: "subtract-works",
			AssertionIdx: 2,
			Pattern:      "Budget",
			Path:         "internal/budget.go",
			Reason:       "single exported identifier — ambiguous if file contains multiple types",
		},
	}

	// Invoke: call the helper twice (looping over warnings), simulating the two call sites.
	retryMessages := make([]string, len(warnings))
	for i, w := range warnings {
		retryMessages[i] = specificityWarningString(w)
	}
	eventMessages := make([]string, len(warnings))
	for i, w := range warnings {
		eventMessages[i] = specificityWarningString(w)
	}

	// Assert: both calls produce identical output.
	if len(retryMessages) != len(eventMessages) {
		t.Fatalf("length mismatch: retry=%d, event=%d", len(retryMessages), len(eventMessages))
	}
	for i := range retryMessages {
		if retryMessages[i] != eventMessages[i] {
			t.Errorf("message %d differs:\n  retry: %s\n  event: %s", i, retryMessages[i], eventMessages[i])
		}
	}

	// Assert: each message contains the expected fields from the warning.
	for i, msg := range retryMessages {
		w := warnings[i]
		for _, substr := range []string{w.ScenarioName, w.Pattern, w.Path, w.Reason} {
			if !contains(msg, substr) {
				t.Errorf("message %d missing %q: %s", i, substr, msg)
			}
		}
	}

	// Assert: the format string produces the expected layout.
	expected0 := "add-works (assertion 0, calc/calc.go, pattern: ModelTier): single exported identifier — ambiguous if file contains multiple types"
	if retryMessages[0] != expected0 {
		t.Errorf("unexpected format:\n  got:  %s\n  want: %s", retryMessages[0], expected0)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexString(s, substr) >= 0)
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
