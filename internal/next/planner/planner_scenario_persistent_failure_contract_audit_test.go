package planner

import (
	"strings"
	"testing"
)

func TestScenario_PersistentFailureTriggersContractAuditSection(t *testing.T) {
	// Seed: a replan context with one contract failure and its corresponding persistent-failure hint
	req := FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures: []string{
			`contract:first-failure-no-escalation — file_contains failed: pattern "ChainIDs.*\[\]string" not found in "internal/next/runstore/types.go"`,
			`persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug`,
		},
		Cycle: 2,
	}

	// Invoke
	prompt := buildFixPlanPrompt(req)

	// Assert: persistent failures audit section is present
	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
		t.Fatal("expected persistent failures audit section")
	}

	// Assert: the persistent-failure hint appears in the audit section
	if !strings.Contains(prompt, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
		t.Fatal("persistent-failure hint must appear in the audit section")
	}

	// Assert: audit instructions reference scenario-contracts.yaml
	if !strings.Contains(prompt, "scenario-contracts.yaml") {
		t.Fatal("audit section must instruct to check scenario-contracts.yaml")
	}

	// Assert: the contract failure appears in the validation failures section
	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
	if validationIdx < 0 {
		t.Fatal("expected validation failures section")
	}
	validationSection := prompt[validationIdx:]
	if !strings.Contains(validationSection, `contract:first-failure-no-escalation — file_contains failed`) {
		t.Fatal("contract failure must appear in the validation failures section")
	}

	// Assert: persistent-failure hint also appears in validation section (duplicated from audit)
	if !strings.Contains(validationSection, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
		t.Fatal("persistent-failure hint must also appear in validation failures section")
	}

	// Assert: hint appears at least twice (once in audit section, once in validation section)
	hint := "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles"
	if strings.Count(prompt, hint) < 2 {
		t.Fatalf("persistent-failure hint must appear at least twice (audit section + validation section), got %d occurrence(s)", strings.Count(prompt, hint))
	}

	// Assert: audit section appears before validation section
	auditIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
	if auditIdx > validationIdx {
		t.Fatal("persistent failures audit section must appear before validation failures section")
	}
}