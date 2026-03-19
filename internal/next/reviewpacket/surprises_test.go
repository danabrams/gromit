package reviewpacket

import (
	"strings"
	"testing"
)

func TestDetectSurprises_NoSurprises_CleanRun(t *testing.T) {
	inputs := Inputs{
		RunID:           "run-1",
		SpecTitle:       "Test Spec",
		SpecContent:     "# Test\n### Scenario: test\nGiven x\nWhen y\nThen z",
		TerminalState:   "ready_for_review",
		DegradedFlags:   []string{},
		RepairCycles:    0,
		RepeatedFailure: false,
		AcceptanceResult: AcceptanceData{
			Passed:  3,
			Failed:  0,
			Unclear: 0,
		},
	}

	surprises := DetectSurprises(inputs)

	if len(surprises) != 0 {
		t.Errorf("expected no surprises in clean run, got %d: %v", len(surprises), surprises)
	}
}

func TestDetectSurprises_AcceptancePassedDespiteDegradedEvidence(t *testing.T) {
	inputs := Inputs{
		RunID:           "run-1",
		SpecTitle:       "Test Spec",
		SpecContent:     "# Test\n### Scenario: test\nGiven x\nWhen y\nThen z",
		TerminalState:   "ready_for_review",
		DegradedFlags:   []string{"diff_unavailable"},
		RepairCycles:    0,
		RepeatedFailure: false,
		AcceptanceResult: AcceptanceData{
			Passed:  3,
			Failed:  0,
			Unclear: 0,
		},
	}

	surprises := DetectSurprises(inputs)

	if len(surprises) == 0 {
		t.Fatal("expected to detect surprise: acceptance passed despite degraded evidence")
	}

	found := false
	for _, s := range surprises {
		if strings.Contains(s, "acceptance") && strings.Contains(s, "degraded") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected surprise mentioning acceptance and degraded evidence, got: %v", surprises)
	}
}

func TestDetectSurprises_MultipleAcceptancePassedWithMultipleDegradedFlags(t *testing.T) {
	inputs := Inputs{
		RunID:           "run-1",
		SpecTitle:       "Test Spec",
		SpecContent:     "# Test\n### Scenario: s1\nGiven x\nWhen y\nThen z\n### Scenario: s2\nGiven a\nWhen b\nThen c",
		TerminalState:   "ready_for_review",
		DegradedFlags:   []string{"diff_unavailable", "review_timeout"},
		RepairCycles:    0,
		RepeatedFailure: false,
		AcceptanceResult: AcceptanceData{
			Passed:  5,
			Failed:  0,
			Unclear: 0,
		},
	}

	surprises := DetectSurprises(inputs)

	if len(surprises) == 0 {
		t.Fatal("expected to detect surprise: acceptance passed despite degraded evidence")
	}
}

func TestDetectSurprises_ScenarioCountMismatch(t *testing.T) {
	// More scenarios than passing acceptance criteria
	inputs := Inputs{
		RunID:           "run-1",
		SpecTitle:       "Test Spec",
		SpecContent:     "# Test\n### Scenario: s1\nGiven x\nWhen y\nThen z\n### Scenario: s2\nGiven a\nWhen b\nThen c\n### Scenario: s3\nGiven p\nWhen q\nThen r",
		TerminalState:   "ready_for_review",
		DegradedFlags:   []string{},
		RepairCycles:    0,
		RepeatedFailure: false,
		AcceptanceResult: AcceptanceData{
			Passed:  2,
			Failed:  0,
			Unclear: 0,
		},
	}

	surprises := DetectSurprises(inputs)

	if len(surprises) == 0 {
		t.Fatal("expected to detect surprise: scenario count mismatch")
	}

	found := false
	for _, s := range surprises {
		if strings.Contains(s, "scenario") && strings.Contains(s, "mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected surprise mentioning scenario mismatch, got: %v", surprises)
	}
}

func TestDetectSurprises_AcceptancePartiallyPassedWithDegradedFlags(t *testing.T) {
	// If acceptance is not fully passed, degraded flags aren't surprising
	inputs := Inputs{
		RunID:           "run-1",
		SpecTitle:       "Test Spec",
		SpecContent:     "# Test\n### Scenario: test\nGiven x\nWhen y\nThen z",
		TerminalState:   "ready_for_review",
		DegradedFlags:   []string{"diff_unavailable"},
		RepairCycles:    0,
		RepeatedFailure: false,
		AcceptanceResult: AcceptanceData{
			Passed:  2,
			Failed:  1,
			Unclear: 0,
		},
	}

	surprises := DetectSurprises(inputs)

	// Should not have the "acceptance passed despite degraded" surprise
	for _, s := range surprises {
		if strings.Contains(s, "acceptance") && strings.Contains(s, "passed") && strings.Contains(s, "degraded") {
			t.Errorf("should not flag partial acceptance with degraded flags as surprise: %v", surprises)
		}
	}
}

func TestDetectSurprises_BlockedRunWithAcceptancePassed(t *testing.T) {
	// Blocked run with passed acceptance is surprising
	inputs := Inputs{
		RunID:           "run-1",
		SpecTitle:       "Test Spec",
		SpecContent:     "# Test\n### Scenario: test\nGiven x\nWhen y\nThen z",
		TerminalState:   "blocked",
		DegradedFlags:   []string{},
		RepairCycles:    0,
		RepeatedFailure: false,
		AcceptanceResult: AcceptanceData{
			Passed:  3,
			Failed:  0,
			Unclear: 0,
		},
	}

	surprises := DetectSurprises(inputs)

	if len(surprises) == 0 {
		t.Fatal("expected to detect surprise: blocked run with passed acceptance")
	}

	found := false
	for _, s := range surprises {
		if strings.Contains(s, "blocked") && strings.Contains(s, "acceptance") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected surprise mentioning blocked and acceptance, got: %v", surprises)
	}
}
