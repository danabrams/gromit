package planner

import (
	"strings"
	"testing"
)

func TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure(t *testing.T) {
	// Seed: a replan context with only a persistent-failure hint and no
	// corresponding contract: failure entry (the original failure was
	// deduplicated into a summary that no longer carries the contract: prefix).
	req := FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures: []string{
			"persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
		},
		Cycle: 2,
	}

	// Invoke
	prompt := buildFixPlanPrompt(req)

	// Assert: persistent failures audit section is present with full instructions
	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
		t.Fatal("persistent failures audit section must be present")
	}
	if !strings.Contains(prompt, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
		t.Fatal("persistent-failure hint must appear in the audit section")
	}
	// Audit instructions must be present
	if !strings.Contains(prompt, "BEFORE creating any implementation fix task for these failures:") {
		t.Fatal("audit directive must be present in persistent failures section")
	}
	if !strings.Contains(prompt, "scenario-contracts.yaml") {
		t.Fatal("audit section must mention scenario-contracts.yaml as the audit target")
	}
	if !strings.Contains(prompt, "contract fix task") {
		t.Fatal("audit section must mention creating a contract fix task as the action")
	}
	if strings.Contains(prompt, "escalate to the spec author") {
		t.Fatal("audit instructions must not contain 'escalate to the spec author'")
	}

	// Assert: the persistent-failure hint also appears in Validation Failures
	// (it is not a review: entry, so it lands in otherFailures)
	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
	if validationIdx < 0 {
		t.Fatal("validation failures section must be present")
	}
	validationSection := prompt[validationIdx:]
	if !strings.Contains(validationSection, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
		t.Fatal("persistent-failure hint must also appear in the validation failures section")
	}

	// Assert: the hint appears at least twice (once per section)
	hint := "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles"
	count := strings.Count(prompt, hint)
	if count < 2 {
		t.Fatalf("persistent-failure hint must appear at least twice (audit + validation), got %d", count)
	}

	// Assert: persistent failures section appears before validation failures section
	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
	if persistentIdx > validationIdx {
		t.Fatal("persistent failures section must appear before validation failures section")
	}

	// Assert: no review findings section (there are no review: entries)
	if strings.Contains(prompt, "## Review Findings to Fix") {
		t.Fatal("review findings section must not appear when there are no review: entries")
	}
}