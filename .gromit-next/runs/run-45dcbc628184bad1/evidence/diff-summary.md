diff --git a/internal/next/planner/planner.go b/internal/next/planner/planner.go
index 879fdf247..867845c79 100644
--- a/internal/next/planner/planner.go
+++ b/internal/next/planner/planner.go
@@ -156,17 +156,44 @@ func buildFixPlanPrompt(req FixPlanRequest) string {
 		b.WriteString("\n")
 	}
 
-	// Separate review findings from other failures for clarity.
+	// Separate persistent failures, review findings, and other failures for clarity.
+	var persistentFailures []string
 	var reviewFindings []string
 	var otherFailures []string
 	for _, f := range req.Failures {
-		if strings.HasPrefix(f, "review:") {
+		if strings.HasPrefix(f, "persistent-failure:") {
+			persistentFailures = append(persistentFailures, f)
+			// Also add to otherFailures so it appears in the validation section
+			otherFailures = append(otherFailures, f)
+		} else if strings.HasPrefix(f, "review:") {
 			reviewFindings = append(reviewFindings, f)
 		} else {
 			otherFailures = append(otherFailures, f)
 		}
 	}
 
+	if len(persistentFailures) > 0 {
+		b.WriteString("## Persistent Failures — Possible Bad Contracts\n")
+		b.WriteString("The following failures have repeated across multiple consecutive cycles.\n")
+		b.WriteString("This strongly suggests the contract assertion itself is wrong, not the implementation.\n")
+		b.WriteString("\n")
+		b.WriteString("BEFORE creating any implementation fix task for these failures:\n")
+		b.WriteString("1. Find the assertion in scenario-contracts.yaml that corresponds to this failure\n")
+		b.WriteString("2. Verify the pattern actually appears in the target file (run grep manually in your head)\n")
+		b.WriteString("3. If the pattern looks like a regex (contains .*  \\w+  \\[  etc.) but the file uses\n")
+		b.WriteString("   literal Go syntax, the pattern may need to be a literal substring instead\n")
+		b.WriteString("4. Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you\n")
+		b.WriteString("   have high confidence the implementation is wrong\n")
+		b.WriteString("\n")
+		b.WriteString("Persistent failures:\n")
+		for _, f := range persistentFailures {
+			b.WriteString("- ")
+			b.WriteString(f)
+			b.WriteString("\n")
+		}
+		b.WriteString("\n")
+	}
+
 	if len(reviewFindings) > 0 {
 		b.WriteString("## Review Findings to Fix\n")
 		b.WriteString("The following review warnings were raised against the current code. Each fix task you create MUST directly address one or more of these findings.\n")
diff --git a/internal/next/planner/planner_scenario_multiple_persistent_failures_test.go b/internal/next/planner/planner_scenario_multiple_persistent_failures_test.go
new file mode 100644
index 000000000..8015ea1d0
--- /dev/null
+++ b/internal/next/planner/planner_scenario_multiple_persistent_failures_test.go
@@ -0,0 +1,66 @@
+package planner
+
+import (
+	"strings"
+	"testing"
+)
+
+func TestScenario_MultiplePersistentFailuresAcrossDifferentContracts(t *testing.T) {
+	// Seed: three contract failures, two with persistent-failure hints
+	req := FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures: []string{
+			"persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles — may indicate a bad test specification",
+			"contract:validation-contracts.yaml input_sanitization failed: expected sanitized output",
+			"persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles — may indicate a bad test specification",
+		},
+		Cycle: 3,
+	}
+
+	// Invoke
+	prompt := buildFixPlanPrompt(req)
+
+	// Assert: persistent failures section exists
+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
+		t.Fatal("expected persistent failures section to be present")
+	}
+
+	// Assert: both persistent failures appear in the dedicated section
+	persistentSection := prompt[strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts"):]
+	validationIdx := strings.Index(persistentSection, "## Validation Failures to Fix")
+	if validationIdx < 0 {
+		t.Fatal("expected validation failures section after persistent failures section")
+	}
+	persistentOnly := persistentSection[:validationIdx]
+
+	if !strings.Contains(persistentOnly, "persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles") {
+		t.Fatal("first persistent failure must appear in persistent failures section")
+	}
+	if !strings.Contains(persistentOnly, "persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles") {
+		t.Fatal("second persistent failure must appear in persistent failures section")
+	}
+
+	// Assert: non-persistent failure does NOT appear in the persistent section
+	if strings.Contains(persistentOnly, "contract:validation-contracts.yaml input_sanitization") {
+		t.Fatal("non-persistent failure must not appear in persistent failures section")
+	}
+
+	// Assert: all three failures appear in the validation failures section
+	validationSection := persistentSection[validationIdx:]
+	if !strings.Contains(validationSection, "persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles") {
+		t.Fatal("first persistent failure must also appear in validation failures section")
+	}
+	if !strings.Contains(validationSection, "contract:validation-contracts.yaml input_sanitization failed: expected sanitized output") {
+		t.Fatal("non-persistent failure must appear in validation failures section")
+	}
+	if !strings.Contains(validationSection, "persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles") {
+		t.Fatal("second persistent failure must also appear in validation failures section")
+	}
+
+	// Assert: persistent section appears before validation section in the full prompt
+	fullPersistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
+	fullValidationIdx := strings.Index(prompt, "## Validation Failures to Fix")
+	if fullPersistentIdx > fullValidationIdx {
+		t.Fatal("persistent failures section must appear before validation failures section")
+	}
+}
\ No newline at end of file
diff --git a/internal/next/planner/planner_scenario_no_persistent_failures_test.go b/internal/next/planner/planner_scenario_no_persistent_failures_test.go
new file mode 100644
index 000000000..64373eaf9
--- /dev/null
+++ b/internal/next/planner/planner_scenario_no_persistent_failures_test.go
@@ -0,0 +1,61 @@
+package planner
+
+import (
+	"strings"
+	"testing"
+)
+
+func TestScenario_NoPersistentFailures_PromptUnchanged(t *testing.T) {
+	// Seed: a fix plan request with only ordinary failures, no persistent-failure: entries
+	req := FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		CompletedTasks: []CompletedTask{{
+			TaskID:            "t-001",
+			Attempts:          1,
+			FilesChanged:      []string{"pkg/foo.go"},
+			ValidationOutcome: "failed",
+		}},
+		Failures: []string{
+			"contract:scenario-contracts.yaml TestAdd expected 4 got 5",
+			"go test ./pkg/... FAIL",
+			"lint error in pkg/foo.go: unused variable",
+		},
+		CurrentDiff: "diff --git a/pkg/foo.go b/pkg/foo.go\n-old\n+new",
+		Cycle:       2,
+	}
+
+	// Invoke
+	prompt := buildFixPlanPrompt(req)
+
+	// Assert: no Persistent Failures section appears
+	if strings.Contains(prompt, "## Persistent Failures") {
+		t.Fatal("prompt must not contain '## Persistent Failures' section when no persistent-failure: entries exist")
+	}
+	if strings.Contains(prompt, "Possible Bad Contracts") {
+		t.Fatal("prompt must not contain 'Possible Bad Contracts' when no persistent-failure: entries exist")
+	}
+	if strings.Contains(prompt, "Audit Instructions") {
+		t.Fatal("prompt must not contain 'Audit Instructions' when no persistent-failure: entries exist")
+	}
+
+	// Assert: standard sections are present
+	if !strings.Contains(prompt, "## Completed Tasks") {
+		t.Fatal("prompt must contain Completed Tasks section")
+	}
+	if !strings.Contains(prompt, "## Validation Failures to Fix") {
+		t.Fatal("prompt must contain Validation Failures section")
+	}
+	if !strings.Contains(prompt, "## Current Diff") {
+		t.Fatal("prompt must contain Current Diff section")
+	}
+	if !strings.Contains(prompt, "## Instructions") {
+		t.Fatal("prompt must contain Instructions section")
+	}
+
+	// Assert: all ordinary failures appear in the validation section
+	for _, f := range req.Failures {
+		if !strings.Contains(prompt, f) {
+			t.Fatalf("prompt must contain failure %q", f)
+		}
+	}
+}
\ No newline at end of file
diff --git a/internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go b/internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go
new file mode 100644
index 000000000..3eb2d624f
--- /dev/null
+++ b/internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go
@@ -0,0 +1,57 @@
+package planner
+
+import (
+	"strings"
+	"testing"
+)
+
+func TestScenario_PersistentFailureTriggersContractAuditSection(t *testing.T) {
+	// Seed: a replan context with one contract failure and its corresponding persistent-failure hint
+	req := FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures: []string{
+			`contract:first-failure-no-escalation — file_contains failed: pattern "ChainIDs.*\[\]string" not found in "internal/next/runstore/types.go"`,
+			`persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug`,
+		},
+		Cycle: 2,
+	}
+
+	// Invoke
+	prompt := buildFixPlanPrompt(req)
+
+	// Assert: persistent failures audit section is present
+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
+		t.Fatal("expected persistent failures audit section")
+	}
+
+	// Assert: the persistent-failure hint appears in the audit section
+	if !strings.Contains(prompt, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
+		t.Fatal("persistent-failure hint must appear in the audit section")
+	}
+
+	// Assert: audit instructions reference scenario-contracts.yaml
+	if !strings.Contains(prompt, "scenario-contracts.yaml") {
+		t.Fatal("audit section must instruct to check scenario-contracts.yaml")
+	}
+
+	// Assert: the contract failure appears in the validation failures section
+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
+	if validationIdx < 0 {
+		t.Fatal("expected validation failures section")
+	}
+	validationSection := prompt[validationIdx:]
+	if !strings.Contains(validationSection, `contract:first-failure-no-escalation — file_contains failed`) {
+		t.Fatal("contract failure must appear in the validation failures section")
+	}
+
+	// Assert: persistent-failure hint also appears in validation section (duplicated from audit)
+	if !strings.Contains(validationSection, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
+		t.Fatal("persistent-failure hint must also appear in validation failures section")
+	}
+
+	// Assert: audit section appears before validation section
+	auditIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
+	if auditIdx > validationIdx {
+		t.Fatal("persistent failures audit section must appear before validation failures section")
+	}
+}
\ No newline at end of file
diff --git a/internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go b/internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go
new file mode 100644
index 000000000..c7ff543af
--- /dev/null
+++ b/internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go
@@ -0,0 +1,72 @@
+package planner
+
+import (
+	"strings"
+	"testing"
+)
+
+func TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure(t *testing.T) {
+	// Seed: a replan context with only a persistent-failure hint and no
+	// corresponding contract: failure entry (the original failure was
+	// deduplicated into a summary that no longer carries the contract: prefix).
+	req := FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures: []string{
+			"persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
+		},
+		Cycle: 2,
+	}
+
+	// Invoke
+	prompt := buildFixPlanPrompt(req)
+
+	// Assert: persistent failures audit section is present with full instructions
+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
+		t.Fatal("persistent failures audit section must be present")
+	}
+	if !strings.Contains(prompt, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
+		t.Fatal("persistent-failure hint must appear in the audit section")
+	}
+	// Audit instructions must be present
+	if !strings.Contains(prompt, "BEFORE creating any implementation fix task for these failures:") {
+		t.Fatal("audit directive must be present in persistent failures section")
+	}
+	if !strings.Contains(prompt, "Find the assertion in scenario-contracts.yaml that corresponds to this failure") {
+		t.Fatal("audit instruction step 1 must be present")
+	}
+	if !strings.Contains(prompt, "Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you") {
+		t.Fatal("audit instruction step 4 guidance must be present")
+	}
+	if strings.Contains(prompt, "escalate to the spec author") {
+		t.Fatal("audit instructions must not contain 'escalate to the spec author'")
+	}
+
+	// Assert: the persistent-failure hint also appears in Validation Failures
+	// (it is not a review: entry, so it lands in otherFailures)
+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
+	if validationIdx < 0 {
+		t.Fatal("validation failures section must be present")
+	}
+	validationSection := prompt[validationIdx:]
+	if !strings.Contains(validationSection, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
+		t.Fatal("persistent-failure hint must also appear in the validation failures section")
+	}
+
+	// Assert: the hint appears at least twice (once per section)
+	hint := "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles"
+	count := strings.Count(prompt, hint)
+	if count < 2 {
+		t.Fatalf("persistent-failure hint must appear at least twice (audit + validation), got %d", count)
+	}
+
+	// Assert: persistent failures section appears before validation failures section
+	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
+	if persistentIdx > validationIdx {
+		t.Fatal("persistent failures section must appear before validation failures section")
+	}
+
+	// Assert: no review findings section (there are no review: entries)
+	if strings.Contains(prompt, "## Review Findings to Fix") {
+		t.Fatal("review findings section must not appear when there are no review: entries")
+	}
+}
\ No newline at end of file
diff --git a/internal/next/planner/planner_test.go b/internal/next/planner/planner_test.go
index 9dc2e7ffe..abde1df3c 100644
--- a/internal/next/planner/planner_test.go
+++ b/internal/next/planner/planner_test.go
@@ -338,3 +338,179 @@ func TestBuildFixPlanPrompt_InstructsAboutFixesField(t *testing.T) {
 		t.Fatal("buildFixPlanPrompt must reference 'failed task' when describing the fixes field")
 	}
 }
+
+func TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection(t *testing.T) {
+	prompt := buildFixPlanPrompt(FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures: []string{
+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
+		},
+		Cycle: 2,
+	})
+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
+		t.Fatal("buildFixPlanPrompt must include persistent failures audit section when present")
+	}
+	if !strings.Contains(prompt, "persistent-failure: TestFoo has failed 2 consecutive cycles") {
+		t.Fatal("persistent failure hint must appear in the audit section")
+	}
+}
+
+func TestBuildFixPlanPrompt_PersistentFailures_AppearsBeforeValidationSection(t *testing.T) {
+	prompt := buildFixPlanPrompt(FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures: []string{
+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
+			"compilation error in main.go",
+		},
+		Cycle: 2,
+	})
+	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
+	if persistentIdx < 0 {
+		t.Fatal("persistent failures section must be present")
+	}
+	if validationIdx < 0 {
+		t.Fatal("validation failures section must be present")
+	}
+	if persistentIdx > validationIdx {
+		t.Fatal("persistent failures section must appear before validation failures section")
+	}
+}
+
+func TestBuildFixPlanPrompt_PersistentFailures_AlsoAppearsInValidationSection(t *testing.T) {
+	prompt := buildFixPlanPrompt(FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures: []string{
+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
+		},
+		Cycle: 2,
+	})
+	// Count occurrences of the persistent failure hint
+	persistentHint := "persistent-failure: TestFoo has failed 2 consecutive cycles"
+	count := strings.Count(prompt, persistentHint)
+	if count < 2 {
+		t.Fatalf("persistent failure hint must appear at least twice (audit section and validation section), got %d", count)
+	}
+}
+
+func TestBuildFixPlanPrompt_NoPersistentFailures_NoSection(t *testing.T) {
+	prompt := buildFixPlanPrompt(FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures:     []string{"compilation error in main.go"},
+		Cycle:        2,
+	})
+	if strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
+		t.Fatal("persistent failures section must not appear when there are no persistent failures")
+	}
+}
+
+func TestBuildFixPlanPrompt_MultiplePersistentFailures_MixedWithNonPersistent(t *testing.T) {
+	prompt := buildFixPlanPrompt(FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures: []string{
+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
+			"lint error in package.go",
+			"persistent-failure: TestBar has failed 3 consecutive cycles — may indicate a bad test specification",
+			"compilation error in main.go",
+		},
+		Cycle: 2,
+	})
+	// Check persistent failures section exists
+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
+		t.Fatal("persistent failures section must be present")
+	}
+	// Check both persistent failures appear in their section
+	if !strings.Contains(prompt, "persistent-failure: TestFoo has failed 2 consecutive cycles") {
+		t.Fatal("first persistent failure must appear in audit section")
+	}
+	if !strings.Contains(prompt, "persistent-failure: TestBar has failed 3 consecutive cycles") {
+		t.Fatal("second persistent failure must appear in audit section")
+	}
+	// Check validation section still exists
+	if !strings.Contains(prompt, "## Validation Failures to Fix") {
+		t.Fatal("validation failures section must be present")
+	}
+	// Check non-persistent failures appear in validation section
+	if !strings.Contains(prompt, "lint error in package.go") {
+		t.Fatal("non-persistent failure must appear in validation section")
+	}
+	if !strings.Contains(prompt, "compilation error in main.go") {
+		t.Fatal("non-persistent failure must appear in validation section")
+	}
+}
+
+func TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure(t *testing.T) {
+	prompt := buildFixPlanPrompt(FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures: []string{
+			"persistent-failure: contract:scenario-contracts.yaml scenario-name has failed 2 consecutive cycles — may indicate a bad test specification",
+		},
+		Cycle: 2,
+	})
+	// Persistent failures section must exist even without corresponding contract failure
+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
+		t.Fatal("persistent failures section must be present even without corresponding contract failure")
+	}
+	// The persistent failure hint must appear in the section
+	if !strings.Contains(prompt, "persistent-failure: contract:scenario-contracts.yaml") {
+		t.Fatal("persistent-failure hint must appear in audit section")
+	}
+}
+
+func TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts(t *testing.T) {
+	// Test multiple persistent failures across different contracts
+	prompt := buildFixPlanPrompt(FixPlanRequest{
+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
+		Failures: []string{
+			`contract:contract-alpha — file_contains failed: pattern "ExpectedType" not found in "types.go"`,
+			`persistent-failure: contract:contract-alpha has failed 2 consecutive cycles — may indicate a bad test specification`,
+			`contract:contract-beta — assertion failed: expected 5 assertions but got 3`,
+			`persistent-failure: contract:contract-beta has failed 3 consecutive cycles — may indicate a bad test specification`,
+			`contract:contract-gamma — timeout after 30s`,
+		},
+		Cycle: 2,
+	})
+
+	// Assert: persistent failures section exists
+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
+		t.Fatal("persistent failures section must be present")
+	}
+
+	// Assert: both persistent failures appear in the persistent section
+	if !strings.Contains(prompt, "persistent-failure: contract:contract-alpha has failed 2 consecutive cycles") {
+		t.Fatal("first persistent failure must appear in persistent section")
+	}
+	if !strings.Contains(prompt, "persistent-failure: contract:contract-beta has failed 3 consecutive cycles") {
+		t.Fatal("second persistent failure must appear in persistent section")
+	}
+
+	// Assert: validation failures section exists
+	if !strings.Contains(prompt, "## Validation Failures to Fix") {
+		t.Fatal("validation failures section must be present")
+	}
+
+	// Assert: all three contract failures appear in validation section
+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
+	validationSection := prompt[validationIdx:]
+	if !strings.Contains(validationSection, "contract:contract-alpha") {
+		t.Fatal("first contract failure must appear in validation section")
+	}
+	if !strings.Contains(validationSection, "contract:contract-beta") {
+		t.Fatal("second contract failure must appear in validation section")
+	}
+	if !strings.Contains(validationSection, "contract:contract-gamma") {
+		t.Fatal("third contract failure must appear in validation section")
+	}
+
+	// Assert: non-persistent failure (contract-gamma timeout) does not appear in persistent section
+	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
+	persistentSection := prompt[persistentIdx:validationIdx]
+	if strings.Contains(persistentSection, "contract:contract-gamma") {
+		t.Fatal("non-persistent failure must not appear in persistent section")
+	}
+
+	// Assert: persistent section appears before validation section
+	if persistentIdx > validationIdx {
+		t.Fatal("persistent failures section must appear before validation failures section")
+	}
+}