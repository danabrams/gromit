package planner

import (
	"strings"
	"testing"
)

func TestScenario_MultiplePersistentFailuresAcrossDifferentContracts(t *testing.T) {
	// Seed: three contract failures, two with persistent-failure hints
	req := FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures: []string{
			"persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles — may indicate a bad test specification",
			"contract:validation-contracts.yaml input_sanitization failed: expected sanitized output",
			"persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles — may indicate a bad test specification",
		},
		Cycle: 3,
	}

	// Invoke
	prompt := buildFixPlanPrompt(req)

	// Assert: persistent failures section exists
	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
		t.Fatal("expected persistent failures section to be present")
	}

	// Assert: both persistent failures appear in the dedicated section
	persistentSection := prompt[strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts"):]
	validationIdx := strings.Index(persistentSection, "## Validation Failures to Fix")
	if validationIdx < 0 {
		t.Fatal("expected validation failures section after persistent failures section")
	}
	persistentOnly := persistentSection[:validationIdx]

	if !strings.Contains(persistentOnly, "persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles") {
		t.Fatal("first persistent failure must appear in persistent failures section")
	}
	if !strings.Contains(persistentOnly, "persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles") {
		t.Fatal("second persistent failure must appear in persistent failures section")
	}

	// Assert: non-persistent failure does NOT appear in the persistent section
	if strings.Contains(persistentOnly, "contract:validation-contracts.yaml input_sanitization") {
		t.Fatal("non-persistent failure must not appear in persistent failures section")
	}

	// Assert: all three failures appear in the validation failures section
	validationSection := persistentSection[validationIdx:]
	if !strings.Contains(validationSection, "persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles") {
		t.Fatal("first persistent failure must also appear in validation failures section")
	}
	if !strings.Contains(validationSection, "contract:validation-contracts.yaml input_sanitization failed: expected sanitized output") {
		t.Fatal("non-persistent failure must appear in validation failures section")
	}
	if !strings.Contains(validationSection, "persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles") {
		t.Fatal("second persistent failure must also appear in validation failures section")
	}

	// Assert: persistent section appears before validation section in the full prompt
	fullPersistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
	fullValidationIdx := strings.Index(prompt, "## Validation Failures to Fix")
	if fullPersistentIdx > fullValidationIdx {
		t.Fatal("persistent failures section must appear before validation failures section")
	}
}
