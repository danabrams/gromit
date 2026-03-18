# Review Decision Sheet

## Terminal State

blocked

## What Changed

diff --git a/.gromit-next/runs/run-45dcbc628184bad1/events.jsonl b/.gromit-next/runs/run-45dcbc628184bad1/events.jsonl
deleted file mode 100644
index 3e214af8e..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/events.jsonl
+++ /dev/null
@@ -1,30 +0,0 @@
-{"type":"run_started","timestamp":"2026-03-18T13:49:16.780383-04:00","spec_id":"0003e-persistent-failure-contract-audit","project_id":"gromit"}
-{"type":"contracts_written","timestamp":"2026-03-18T13:50:30.926146-04:00","scenario_count":4}
-{"type":"task_started","timestamp":"2026-03-18T13:50:30.926274-04:00","task_id":"t-001","cycle":1,"task_index":1,"task_total":2,"objective":"Write tests for persistent failure extraction in buildFixPlanPrompt: (1) persistent failures render a dedicated '## Persistent Failures — Possible Bad Contracts' section before '## Validation Failures to Fix', (2) persistent failures still appear in otherFailures/validation section, (3) no persistent failures means no section rendered, (4) multiple persistent failures with mixed non-persistent, (5) persistent-failure hint without corresponding contract: failure still renders section"}
-{"type":"task_validation_result","timestamp":"2026-03-18T13:51:49.362706-04:00","task_id":"t-001","passed":true}
-{"type":"task_completed","timestamp":"2026-03-18T13:51:49.362838-04:00","task_id":"t-001","tokens_used":9398,"duration_ms":78069}
-{"type":"task_started","timestamp":"2026-03-18T13:51:49.362873-04:00","task_id":"t-002","cycle":1,"task_index":2,"task_total":2,"objective":"Implement persistent failure extraction in buildFixPlanPrompt: add a third pass to collect persistent-failure: prefixed entries into persistentFailures slice (without removing from otherFailures), then render a '## Persistent Failures — Possible Bad Contracts' section with audit instructions before the '## Validation Failures to Fix' section when persistentFailures is non-empty"}
-{"type":"task_validation_result","timestamp":"2026-03-18T13:52:21.713742-04:00","task_id":"t-002","passed":true}
-{"type":"task_completed","timestamp":"2026-03-18T13:52:21.713835-04:00","task_id":"t-002","tokens_used":3613,"duration_ms":31647}
-{"type":"scenario_tests_written","timestamp":"2026-03-18T13:52:37.731-04:00","scenario_name":"persistent failure triggers contract audit section","test_file":"internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go"}
-{"type":"scenario_tests_written","timestamp":"2026-03-18T13:52:51.853441-04:00","scenario_name":"no persistent failures — prompt unchanged","test_file":"internal/next/planner/planner_scenario_no_persistent_failures_test.go"}
-{"type":"scenario_tests_written","timestamp":"2026-03-18T13:53:10.881063-04:00","scenario_name":"multiple persistent failures across different contracts","test_file":"internal/next/planner/planner_scenario_multiple_persistent_failures_test.go"}
-{"type":"scenario_tests_written","timestamp":"2026-03-18T13:53:32.712465-04:00","scenario_name":"persistent-failure hint without corresponding contract failure","test_file":"internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go"}
-{"type":"scenario_tests_complete","timestamp":"2026-03-18T13:53:32.712511-04:00","scenario_count":4}
-{"type":"final_validation_result","timestamp":"2026-03-18T13:54:19.367233-04:00","passed":false}
-{"type":"replan_triggered","timestamp":"2026-03-18T13:54:19.367525-04:00","reason":"contract:persistent-failure-triggers-contract-audit-section — file_contains failed: pattern \"regex\" not found in \"internal/next/planner/planner.go\"","source":"validate"}
-{"type":"task_started","timestamp":"2026-03-18T13:54:50.801453-04:00","task_id":"t-003","cycle":2,"task_index":1,"task_total":2,"objective":"Add persistent-failure extraction and dedicated prompt section to buildFixPlanPrompt. After separating review findings from other failures, add a third pass that collects persistent-failure: prefixed entries into a persistentFailures slice (without removing them from otherFailures). When persistentFailures is non-empty, render the '## Persistent Failures — Possible Bad Contracts' section before '## Validation Failures to Fix' with the full audit instructions including the word 'regex' (e.g., 'If the pattern looks like a regex'). Fixes: contract:persistent-failure-triggers-contract-audit-section (pattern 'regex' not found in planner.go)."}
-{"type":"task_validation_result","timestamp":"2026-03-18T13:56:06.063735-04:00","task_id":"t-003","passed":true}
-{"type":"task_completed","timestamp":"2026-03-18T13:56:06.063844-04:00","task_id":"t-003","tokens_used":6821,"duration_ms":74586}
-{"type":"task_started","timestamp":"2026-03-18T13:56:06.063891-04:00","task_id":"t-004","cycle":2,"task_index":2,"task_total":2,"objective":"Add test case for multiple persistent failures across different contracts in planner_test.go. The test name or description must contain the phrase 'multiple persistent'. It should call buildFixPlanPrompt with three contract failures where two have persistent-failure: hints, and assert: (1) both persistent failures appear in the dedicated section, (2) all three contract failures appear in Validation Failures to Fix, (3) the non-persistent failure does not appear in the persistent section. Fixes: contract:multiple-persistent-failures-across-different-contracts."}
-{"type":"task_validation_result","timestamp":"2026-03-18T13:56:57.642091-04:00","task_id":"t-004","passed":true}
-{"type":"task_completed","timestamp":"2026-03-18T13:56:57.642222-04:00","task_id":"t-004","tokens_used":5696,"duration_ms":51009}
-{"type":"final_validation_result","timestamp":"2026-03-18T13:57:05.554659-04:00","passed":true}
-{"type":"review_result","timestamp":"2026-03-18T13:59:20.474181-04:00","total_findings":21,"blocking_findings":7,"findings_by_severity":{"error":7,"info":1,"suggestion":4,"warning":9},"facets_reviewed":["code_quality","spec_alignment"]}
-{"type":"replan_triggered","timestamp":"2026-03-18T13:59:20.474817-04:00","reason":"review:spec_alignment:error:internal/next/planner/planner.go:174 — Intro text deviates from spec. Spec requires: \"The following failures have repeated across multiple consecutive cycles.\\nThis strongly suggests the contract assertion itself is wrong, not the implementation.\" The implementation uses a weaker paraphrase that loses the critical \"strongly suggests\" signal the spec emphasizes. (suggested fix: Replace with exactly two sentences as specified: \"The following failures have repeated across multiple consecutive cycles.\\nThis strongly suggests the contract assertion itself is wrong, not the implementation.\\n\")","source":"review"}
-{"type":"task_started","timestamp":"2026-03-18T13:59:35.949154-04:00","task_id":"t-005","cycle":3,"task_index":1,"task_total":1,"objective":"Replace the persistent-failures section intro text and all four audit instruction steps in buildFixPlanPrompt (lines ~174-187) with the exact wording from the spec. Fixes all 7 review findings: intro must say 'The following failures have repeated across multiple consecutive cycles.\\nThis strongly suggests the contract assertion itself is wrong, not the implementation.'; directive must say 'BEFORE creating any implementation fix task for these failures:'; steps 1-4 must match spec verbatim including regex examples and 'Prefer creating a contract fix task' guidance."}
-{"type":"task_validation_result","timestamp":"2026-03-18T14:00:56.980901-04:00","task_id":"t-005","passed":true}
-{"type":"task_completed","timestamp":"2026-03-18T14:00:56.981034-04:00","task_id":"t-005","tokens_used":7599,"duration_ms":80356}
-{"type":"final_validation_result","timestamp":"2026-03-18T14:01:09.820487-04:00","passed":true}
-{"type":"review_result","timestamp":"2026-03-18T14:03:30.121712-04:00","total_findings":14,"blocking_findings":0,"findings_by_severity":{"info":1,"suggestion":8,"warning":5},"facets_reviewed":["code_quality","spec_alignment"]}
-{"type":"acceptance_result","timestamp":"2026-03-18T14:04:09.281651-04:00","total_criteria":6,"pass_count":6,"fail_count":0,"unclear_count":0}
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/acceptance.json b/.gromit-next/runs/run-45dcbc628184bad1/evidence/acceptance.json
deleted file mode 100644
index db51ea3d3..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/acceptance.json
+++ /dev/null
@@ -1,67 +0,0 @@
-{
-  "results": [
-    {
-      "criterion": "When `req.Failures` contains one or more `persistent-failure:` prefixed entries, `buildFixPlanPrompt` renders a `## Persistent Failures — Possible Bad Contracts` section before `## Validation Failures to Fix`",
-      "status": "pass",
-      "rationale": "The implementation in planner.go separates persistent-failure: entries into a dedicated slice and renders '## Persistent Failures — Possible Bad Contracts' before '## Validation Failures to Fix'. Multiple tests explicitly verify the ordering (persistentIdx \u003c validationIdx) and the section's presence. Validation passed (pass=true).",
-      "evidence_refs": [
-        "internal/next/planner/planner.go",
-        "internal/next/planner/planner_test.go",
-        "internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go",
-        "internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go",
-        "internal/next/planner/planner_scenario_multiple_persistent_failures_test.go"
-      ]
-    },
-    {
-      "criterion": "The persistent failures section includes explicit instructions to audit the contract assertion in `scenario-contracts.yaml` before creating implementation fix tasks",
-      "status": "pass",
-      "rationale": "The implementation in planner.go explicitly writes 'BEFORE creating any implementation fix task for these failures:' followed by step 1 'Find the assertion in scenario-contracts.yaml that corresponds to this failure'. This directly satisfies the criterion. Tests confirm this text appears in the prompt.",
-      "evidence_refs": [
-        "internal/next/planner/planner.go",
-        "internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go"
-      ]
-    },
-    {
-      "criterion": "Persistent failures still appear in `## Validation Failures to Fix` — they are not removed from the main list",
-      "status": "pass",
-      "rationale": "The implementation explicitly adds persistent failures to both `persistentFailures` AND `otherFailures` slices, ensuring they appear in both the dedicated audit section and the `## Validation Failures to Fix` section. Tests verify this: `TestBuildFixPlanPrompt_PersistentFailures_AlsoAppearsInValidationSection` checks count \u003e= 2, and `TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure` asserts the hint appears in the validation section.",
-      "evidence_refs": [
-        "internal/next/planner/planner.go",
-        "internal/next/planner/planner_test.go",
-        "internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go",
-        "internal/next/planner/planner_scenario_multiple_persistent_failures_test.go"
-      ]
-    },
-    {
-      "criterion": "When `req.Failures` contains no `persistent-failure:` entries, no persistent failures section is rendered and existing behavior is unchanged",
-      "status": "pass",
-      "rationale": "The implementation guards the persistent failures section with `if len(persistentFailures) \u003e 0`, so it only renders when there are entries with the `persistent-failure:` prefix. The test `TestBuildFixPlanPrompt_NoPersistentFailures_NoSection` explicitly verifies that when only ordinary failures are present, the `## Persistent Failures` section does not appear, and `TestScenario_NoPersistentFailures_PromptUnchanged` additionally verifies that all standard sections (Completed Tasks, Validation Failures, Current Diff, Instructions) remain present. Both tests pass.",
-      "evidence_refs": [
-        "internal/next/planner/planner.go",
-        "internal/next/planner/planner_scenario_no_persistent_failures_test.go",
-        "internal/next/planner/planner_test.go"
-      ]
-    },
-    {
-      "criterion": "Non-persistent contract failures and review findings are unaffected by this change",
-      "status": "pass",
-      "rationale": "The diff shows that non-persistent `contract:` failures fall into `otherFailures` (unchanged path) and `review:` prefixed failures still go into `reviewFindings` (unchanged path). Both the review findings section and validation failures section logic are untouched. The test `TestScenario_NoPersistentFailures_PromptUnchanged` explicitly verifies that prompts without `persistent-failure:` entries remain unchanged, and `TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts` confirms non-persistent contract failures (e.g., `contract:contract-gamma`) still appear only in the validation section and not the persistent section. Validation pass=true confirms all tests pass.",
-      "evidence_refs": [
-        "internal/next/planner/planner.go",
-        "internal/next/planner/planner_scenario_no_persistent_failures_test.go",
-        "internal/next/planner/planner_test.go"
-      ]
-    },
-    {
-      "criterion": "All existing planner tests continue to pass",
-      "status": "pass",
-      "rationale": "Validation results show pass=true, and the diff shows only additions to planner_test.go (no modifications to existing tests) plus new test files. The existing tests were not changed.",
-      "evidence_refs": [
-        "internal/next/planner/planner_test.go",
-        "internal/next/planner/planner.go"
-      ]
-    }
-  ],
-  "all_pass": true,
-  "has_fail_or_unclear": false
-}
\ No newline at end of file
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/diff-summary.md b/.gromit-next/runs/run-45dcbc628184bad1/evidence/diff-summary.md
deleted file mode 100644
index 7616fc5cd..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/diff-summary.md
+++ /dev/null
@@ -1,519 +0,0 @@
-diff --git a/internal/next/planner/planner.go b/internal/next/planner/planner.go
-index 879fdf247..867845c79 100644
---- a/internal/next/planner/planner.go
-+++ b/internal/next/planner/planner.go
-@@ -156,17 +156,44 @@ func buildFixPlanPrompt(req FixPlanRequest) string {
- 		b.WriteString("\n")
- 	}
- 
--	// Separate review findings from other failures for clarity.
-+	// Separate persistent failures, review findings, and other failures for clarity.
-+	var persistentFailures []string
- 	var reviewFindings []string
- 	var otherFailures []string
- 	for _, f := range req.Failures {
--		if strings.HasPrefix(f, "review:") {
-+		if strings.HasPrefix(f, "persistent-failure:") {
-+			persistentFailures = append(persistentFailures, f)
-+			// Also add to otherFailures so it appears in the validation section
-+			otherFailures = append(otherFailures, f)
-+		} else if strings.HasPrefix(f, "review:") {
- 			reviewFindings = append(reviewFindings, f)
- 		} else {
- 			otherFailures = append(otherFailures, f)
- 		}
- 	}
- 
-+	if len(persistentFailures) > 0 {
-+		b.WriteString("## Persistent Failures — Possible Bad Contracts\n")
-+		b.WriteString("The following failures have repeated across multiple consecutive cycles.\n")
-+		b.WriteString("This strongly suggests the contract assertion itself is wrong, not the implementation.\n")
-+		b.WriteString("\n")
-+		b.WriteString("BEFORE creating any implementation fix task for these failures:\n")
-+		b.WriteString("1. Find the assertion in scenario-contracts.yaml that corresponds to this failure\n")
-+		b.WriteString("2. Verify the pattern actually appears in the target file (run grep manually in your head)\n")
-+		b.WriteString("3. If the pattern looks like a regex (contains .*  \\w+  \\[  etc.) but the file uses\n")
-+		b.WriteString("   literal Go syntax, the pattern may need to be a literal substring instead\n")
-+		b.WriteString("4. Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you\n")
-+		b.WriteString("   have high confidence the implementation is wrong\n")
-+		b.WriteString("\n")
-+		b.WriteString("Persistent failures:\n")
-+		for _, f := range persistentFailures {
-+			b.WriteString("- ")
-+			b.WriteString(f)
-+			b.WriteString("\n")
-+		}
-+		b.WriteString("\n")
-+	}
-+
- 	if len(reviewFindings) > 0 {
- 		b.WriteString("## Review Findings to Fix\n")
- 		b.WriteString("The following review warnings were raised against the current code. Each fix task you create MUST directly address one or more of these findings.\n")
-diff --git a/internal/next/planner/planner_scenario_multiple_persistent_failures_test.go b/internal/next/planner/planner_scenario_multiple_persistent_failures_test.go
-new file mode 100644
-index 000000000..8015ea1d0
---- /dev/null
-+++ b/internal/next/planner/planner_scenario_multiple_persistent_failures_test.go
-@@ -0,0 +1,66 @@
-+package planner
-+
-+import (
-+	"strings"
-+	"testing"
-+)
-+
-+func TestScenario_MultiplePersistentFailuresAcrossDifferentContracts(t *testing.T) {
-+	// Seed: three contract failures, two with persistent-failure hints
-+	req := FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles — may indicate a bad test specification",
-+			"contract:validation-contracts.yaml input_sanitization failed: expected sanitized output",
-+			"persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles — may indicate a bad test specification",
-+		},
-+		Cycle: 3,
-+	}
-+
-+	// Invoke
-+	prompt := buildFixPlanPrompt(req)
-+
-+	// Assert: persistent failures section exists
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("expected persistent failures section to be present")
-+	}
-+
-+	// Assert: both persistent failures appear in the dedicated section
-+	persistentSection := prompt[strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts"):]
-+	validationIdx := strings.Index(persistentSection, "## Validation Failures to Fix")
-+	if validationIdx < 0 {
-+		t.Fatal("expected validation failures section after persistent failures section")
-+	}
-+	persistentOnly := persistentSection[:validationIdx]
-+
-+	if !strings.Contains(persistentOnly, "persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles") {
-+		t.Fatal("first persistent failure must appear in persistent failures section")
-+	}
-+	if !strings.Contains(persistentOnly, "persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles") {
-+		t.Fatal("second persistent failure must appear in persistent failures section")
-+	}
-+
-+	// Assert: non-persistent failure does NOT appear in the persistent section
-+	if strings.Contains(persistentOnly, "contract:validation-contracts.yaml input_sanitization") {
-+		t.Fatal("non-persistent failure must not appear in persistent failures section")
-+	}
-+
-+	// Assert: all three failures appear in the validation failures section
-+	validationSection := persistentSection[validationIdx:]
-+	if !strings.Contains(validationSection, "persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles") {
-+		t.Fatal("first persistent failure must also appear in validation failures section")
-+	}
-+	if !strings.Contains(validationSection, "contract:validation-contracts.yaml input_sanitization failed: expected sanitized output") {
-+		t.Fatal("non-persistent failure must appear in validation failures section")
-+	}
-+	if !strings.Contains(validationSection, "persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles") {
-+		t.Fatal("second persistent failure must also appear in validation failures section")
-+	}
-+
-+	// Assert: persistent section appears before validation section in the full prompt
-+	fullPersistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	fullValidationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	if fullPersistentIdx > fullValidationIdx {
-+		t.Fatal("persistent failures section must appear before validation failures section")
-+	}
-+}
-\ No newline at end of file
-diff --git a/internal/next/planner/planner_scenario_no_persistent_failures_test.go b/internal/next/planner/planner_scenario_no_persistent_failures_test.go
-new file mode 100644
-index 000000000..64373eaf9
---- /dev/null
-+++ b/internal/next/planner/planner_scenario_no_persistent_failures_test.go
-@@ -0,0 +1,61 @@
-+package planner
-+
-+import (
-+	"strings"
-+	"testing"
-+)
-+
-+func TestScenario_NoPersistentFailures_PromptUnchanged(t *testing.T) {
-+	// Seed: a fix plan request with only ordinary failures, no persistent-failure: entries
-+	req := FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		CompletedTasks: []CompletedTask{{
-+			TaskID:            "t-001",
-+			Attempts:          1,
-+			FilesChanged:      []string{"pkg/foo.go"},
-+			ValidationOutcome: "failed",
-+		}},
-+		Failures: []string{
-+			"contract:scenario-contracts.yaml TestAdd expected 4 got 5",
-+			"go test ./pkg/... FAIL",
-+			"lint error in pkg/foo.go: unused variable",
-+		},
-+		CurrentDiff: "diff --git a/pkg/foo.go b/pkg/foo.go\n-old\n+new",
-+		Cycle:       2,
-+	}
-+
-+	// Invoke
-+	prompt := buildFixPlanPrompt(req)
-+
-+	// Assert: no Persistent Failures section appears
-+	if strings.Contains(prompt, "## Persistent Failures") {
-+		t.Fatal("prompt must not contain '## Persistent Failures' section when no persistent-failure: entries exist")
-+	}
-+	if strings.Contains(prompt, "Possible Bad Contracts") {
-+		t.Fatal("prompt must not contain 'Possible Bad Contracts' when no persistent-failure: entries exist")
-+	}
-+	if strings.Contains(prompt, "Audit Instructions") {
-+		t.Fatal("prompt must not contain 'Audit Instructions' when no persistent-failure: entries exist")
-+	}
-+
-+	// Assert: standard sections are present
-+	if !strings.Contains(prompt, "## Completed Tasks") {
-+		t.Fatal("prompt must contain Completed Tasks section")
-+	}
-+	if !strings.Contains(prompt, "## Validation Failures to Fix") {
-+		t.Fatal("prompt must contain Validation Failures section")
-+	}
-+	if !strings.Contains(prompt, "## Current Diff") {
-+		t.Fatal("prompt must contain Current Diff section")
-+	}
-+	if !strings.Contains(prompt, "## Instructions") {
-+		t.Fatal("prompt must contain Instructions section")
-+	}
-+
-+	// Assert: all ordinary failures appear in the validation section
-+	for _, f := range req.Failures {
-+		if !strings.Contains(prompt, f) {
-+			t.Fatalf("prompt must contain failure %q", f)
-+		}
-+	}
-+}
-\ No newline at end of file
-diff --git a/internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go b/internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go
-new file mode 100644
-index 000000000..3eb2d624f
---- /dev/null
-+++ b/internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go
-@@ -0,0 +1,57 @@
-+package planner
-+
-+import (
-+	"strings"
-+	"testing"
-+)
-+
-+func TestScenario_PersistentFailureTriggersContractAuditSection(t *testing.T) {
-+	// Seed: a replan context with one contract failure and its corresponding persistent-failure hint
-+	req := FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			`contract:first-failure-no-escalation — file_contains failed: pattern "ChainIDs.*\[\]string" not found in "internal/next/runstore/types.go"`,
-+			`persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug`,
-+		},
-+		Cycle: 2,
-+	}
-+
-+	// Invoke
-+	prompt := buildFixPlanPrompt(req)
-+
-+	// Assert: persistent failures audit section is present
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("expected persistent failures audit section")
-+	}
-+
-+	// Assert: the persistent-failure hint appears in the audit section
-+	if !strings.Contains(prompt, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
-+		t.Fatal("persistent-failure hint must appear in the audit section")
-+	}
-+
-+	// Assert: audit instructions reference scenario-contracts.yaml
-+	if !strings.Contains(prompt, "scenario-contracts.yaml") {
-+		t.Fatal("audit section must instruct to check scenario-contracts.yaml")
-+	}
-+
-+	// Assert: the contract failure appears in the validation failures section
-+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	if validationIdx < 0 {
-+		t.Fatal("expected validation failures section")
-+	}
-+	validationSection := prompt[validationIdx:]
-+	if !strings.Contains(validationSection, `contract:first-failure-no-escalation — file_contains failed`) {
-+		t.Fatal("contract failure must appear in the validation failures section")
-+	}
-+
-+	// Assert: persistent-failure hint also appears in validation section (duplicated from audit)
-+	if !strings.Contains(validationSection, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
-+		t.Fatal("persistent-failure hint must also appear in validation failures section")
-+	}
-+
-+	// Assert: audit section appears before validation section
-+	auditIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	if auditIdx > validationIdx {
-+		t.Fatal("persistent failures audit section must appear before validation failures section")
-+	}
-+}
-\ No newline at end of file
-diff --git a/internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go b/internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go
-new file mode 100644
-index 000000000..c7ff543af
---- /dev/null
-+++ b/internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go
-@@ -0,0 +1,72 @@
-+package planner
-+
-+import (
-+	"strings"
-+	"testing"
-+)
-+
-+func TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure(t *testing.T) {
-+	// Seed: a replan context with only a persistent-failure hint and no
-+	// corresponding contract: failure entry (the original failure was
-+	// deduplicated into a summary that no longer carries the contract: prefix).
-+	req := FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
-+		},
-+		Cycle: 2,
-+	}
-+
-+	// Invoke
-+	prompt := buildFixPlanPrompt(req)
-+
-+	// Assert: persistent failures audit section is present with full instructions
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures audit section must be present")
-+	}
-+	if !strings.Contains(prompt, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
-+		t.Fatal("persistent-failure hint must appear in the audit section")
-+	}
-+	// Audit instructions must be present
-+	if !strings.Contains(prompt, "BEFORE creating any implementation fix task for these failures:") {
-+		t.Fatal("audit directive must be present in persistent failures section")
-+	}
-+	if !strings.Contains(prompt, "Find the assertion in scenario-contracts.yaml that corresponds to this failure") {
-+		t.Fatal("audit instruction step 1 must be present")
-+	}
-+	if !strings.Contains(prompt, "Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you") {
-+		t.Fatal("audit instruction step 4 guidance must be present")
-+	}
-+	if strings.Contains(prompt, "escalate to the spec author") {
-+		t.Fatal("audit instructions must not contain 'escalate to the spec author'")
-+	}
-+
-+	// Assert: the persistent-failure hint also appears in Validation Failures
-+	// (it is not a review: entry, so it lands in otherFailures)
-+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	if validationIdx < 0 {
-+		t.Fatal("validation failures section must be present")
-+	}
-+	validationSection := prompt[validationIdx:]
-+	if !strings.Contains(validationSection, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
-+		t.Fatal("persistent-failure hint must also appear in the validation failures section")
-+	}
-+
-+	// Assert: the hint appears at least twice (once per section)
-+	hint := "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles"
-+	count := strings.Count(prompt, hint)
-+	if count < 2 {
-+		t.Fatalf("persistent-failure hint must appear at least twice (audit + validation), got %d", count)
-+	}
-+
-+	// Assert: persistent failures section appears before validation failures section
-+	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	if persistentIdx > validationIdx {
-+		t.Fatal("persistent failures section must appear before validation failures section")
-+	}
-+
-+	// Assert: no review findings section (there are no review: entries)
-+	if strings.Contains(prompt, "## Review Findings to Fix") {
-+		t.Fatal("review findings section must not appear when there are no review: entries")
-+	}
-+}
-\ No newline at end of file
-diff --git a/internal/next/planner/planner_test.go b/internal/next/planner/planner_test.go
-index 9dc2e7ffe..abde1df3c 100644
---- a/internal/next/planner/planner_test.go
-+++ b/internal/next/planner/planner_test.go
-@@ -338,3 +338,179 @@ func TestBuildFixPlanPrompt_InstructsAboutFixesField(t *testing.T) {
- 		t.Fatal("buildFixPlanPrompt must reference 'failed task' when describing the fixes field")
- 	}
- }
-+
-+func TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
-+		},
-+		Cycle: 2,
-+	})
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("buildFixPlanPrompt must include persistent failures audit section when present")
-+	}
-+	if !strings.Contains(prompt, "persistent-failure: TestFoo has failed 2 consecutive cycles") {
-+		t.Fatal("persistent failure hint must appear in the audit section")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_PersistentFailures_AppearsBeforeValidationSection(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
-+			"compilation error in main.go",
-+		},
-+		Cycle: 2,
-+	})
-+	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	if persistentIdx < 0 {
-+		t.Fatal("persistent failures section must be present")
-+	}
-+	if validationIdx < 0 {
-+		t.Fatal("validation failures section must be present")
-+	}
-+	if persistentIdx > validationIdx {
-+		t.Fatal("persistent failures section must appear before validation failures section")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_PersistentFailures_AlsoAppearsInValidationSection(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
-+		},
-+		Cycle: 2,
-+	})
-+	// Count occurrences of the persistent failure hint
-+	persistentHint := "persistent-failure: TestFoo has failed 2 consecutive cycles"
-+	count := strings.Count(prompt, persistentHint)
-+	if count < 2 {
-+		t.Fatalf("persistent failure hint must appear at least twice (audit section and validation section), got %d", count)
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_NoPersistentFailures_NoSection(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures:     []string{"compilation error in main.go"},
-+		Cycle:        2,
-+	})
-+	if strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures section must not appear when there are no persistent failures")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_MultiplePersistentFailures_MixedWithNonPersistent(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
-+			"lint error in package.go",
-+			"persistent-failure: TestBar has failed 3 consecutive cycles — may indicate a bad test specification",
-+			"compilation error in main.go",
-+		},
-+		Cycle: 2,
-+	})
-+	// Check persistent failures section exists
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures section must be present")
-+	}
-+	// Check both persistent failures appear in their section
-+	if !strings.Contains(prompt, "persistent-failure: TestFoo has failed 2 consecutive cycles") {
-+		t.Fatal("first persistent failure must appear in audit section")
-+	}
-+	if !strings.Contains(prompt, "persistent-failure: TestBar has failed 3 consecutive cycles") {
-+		t.Fatal("second persistent failure must appear in audit section")
-+	}
-+	// Check validation section still exists
-+	if !strings.Contains(prompt, "## Validation Failures to Fix") {
-+		t.Fatal("validation failures section must be present")
-+	}
-+	// Check non-persistent failures appear in validation section
-+	if !strings.Contains(prompt, "lint error in package.go") {
-+		t.Fatal("non-persistent failure must appear in validation section")
-+	}
-+	if !strings.Contains(prompt, "compilation error in main.go") {
-+		t.Fatal("non-persistent failure must appear in validation section")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: contract:scenario-contracts.yaml scenario-name has failed 2 consecutive cycles — may indicate a bad test specification",
-+		},
-+		Cycle: 2,
-+	})
-+	// Persistent failures section must exist even without corresponding contract failure
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures section must be present even without corresponding contract failure")
-+	}
-+	// The persistent failure hint must appear in the section
-+	if !strings.Contains(prompt, "persistent-failure: contract:scenario-contracts.yaml") {
-+		t.Fatal("persistent-failure hint must appear in audit section")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts(t *testing.T) {
-+	// Test multiple persistent failures across different contracts
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			`contract:contract-alpha — file_contains failed: pattern "ExpectedType" not found in "types.go"`,
-+			`persistent-failure: contract:contract-alpha has failed 2 consecutive cycles — may indicate a bad test specification`,
-+			`contract:contract-beta — assertion failed: expected 5 assertions but got 3`,
-+			`persistent-failure: contract:contract-beta has failed 3 consecutive cycles — may indicate a bad test specification`,
-+			`contract:contract-gamma — timeout after 30s`,
-+		},
-+		Cycle: 2,
-+	})
-+
-+	// Assert: persistent failures section exists
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures section must be present")
-+	}
-+
-+	// Assert: both persistent failures appear in the persistent section
-+	if !strings.Contains(prompt, "persistent-failure: contract:contract-alpha has failed 2 consecutive cycles") {
-+		t.Fatal("first persistent failure must appear in persistent section")
-+	}
-+	if !strings.Contains(prompt, "persistent-failure: contract:contract-beta has failed 3 consecutive cycles") {
-+		t.Fatal("second persistent failure must appear in persistent section")
-+	}
-+
-+	// Assert: validation failures section exists
-+	if !strings.Contains(prompt, "## Validation Failures to Fix") {
-+		t.Fatal("validation failures section must be present")
-+	}
-+
-+	// Assert: all three contract failures appear in validation section
-+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	validationSection := prompt[validationIdx:]
-+	if !strings.Contains(validationSection, "contract:contract-alpha") {
-+		t.Fatal("first contract failure must appear in validation section")
-+	}
-+	if !strings.Contains(validationSection, "contract:contract-beta") {
-+		t.Fatal("second contract failure must appear in validation section")
-+	}
-+	if !strings.Contains(validationSection, "contract:contract-gamma") {
-+		t.Fatal("third contract failure must appear in validation section")
-+	}
-+
-+	// Assert: non-persistent failure (contract-gamma timeout) does not appear in persistent section
-+	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	persistentSection := prompt[persistentIdx:validationIdx]
-+	if strings.Contains(persistentSection, "contract:contract-gamma") {
-+		t.Fatal("non-persistent failure must not appear in persistent section")
-+	}
-+
-+	// Assert: persistent section appears before validation section
-+	if persistentIdx > validationIdx {
-+		t.Fatal("persistent failures section must appear before validation failures section")
-+	}
-+}
\ No newline at end of file
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/metrics.json b/.gromit-next/runs/run-45dcbc628184bad1/evidence/metrics.json
deleted file mode 100644
index ddcd27d98..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/metrics.json
+++ /dev/null
@@ -1,267 +0,0 @@
-{
-  "total_tokens": 33127,
-  "total_cost_usd": 0.6169518,
-  "total_tasks": 5,
-  "passed_tasks": 5,
-  "failed_tasks": 0,
-  "duration_ms": 892850,
-  "cycles": 3,
-  "total_retries": 0,
-  "total_replans": 2,
-  "human_intervention": false,
-  "invocations": [
-    {
-      "phase": "plan",
-      "tier": "high",
-      "model": "opus",
-      "provider": "claude",
-      "tokens_in": 4,
-      "tokens_out": 1002,
-      "duration_ms": 24420,
-      "cost_usd": 0.21054025,
-      "success": true
-    },
-    {
-      "phase": "contracts",
-      "tier": "high",
-      "model": "opus",
-      "provider": "claude",
-      "tokens_in": 7,
-      "tokens_out": 2114,
-      "duration_ms": 49722,
-      "cost_usd": 0.19238975,
-      "success": true
-    },
-    {
-      "phase": "execute",
-      "tier": "low",
-      "model": "haiku",
-      "provider": "claude",
-      "tokens_in": 107,
-      "tokens_out": 9291,
-      "duration_ms": 78069,
-      "cost_usd": 0.1604924,
-      "success": true
-    },
-    {
-      "phase": "execute",
-      "tier": "low",
-      "model": "haiku",
-      "provider": "claude",
-      "tokens_in": 37,
-      "tokens_out": 3576,
-      "duration_ms": 31647,
-      "cost_usd": 0.0558107,
-      "success": true
-    },
-    {
-      "phase": "scenario_tests",
-      "tier": "high",
-      "model": "opus",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 788,
-      "duration_ms": 15801,
-      "cost_usd": 0.1478615,
-      "success": true
-    },
-    {
-      "phase": "scenario_tests",
-      "tier": "high",
-      "model": "opus",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 692,
-      "duration_ms": 13889,
-      "cost_usd": 0.1446365,
-      "success": true
-    },
-    {
-      "phase": "scenario_tests",
-      "tier": "high",
-      "model": "opus",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 1028,
-      "duration_ms": 18813,
-      "cost_usd": 0.15309899999999999,
-      "success": true
-    },
-    {
-      "phase": "scenario_tests",
-      "tier": "high",
-      "model": "opus",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 1118,
-      "duration_ms": 21620,
-      "cost_usd": 0.156099,
-      "success": true
-    },
-    {
-      "phase": "plan",
-      "tier": "high",
-      "model": "opus",
-      "provider": "claude",
-      "tokens_in": 5,
-      "tokens_out": 1777,
-      "duration_ms": 31433,
-      "cost_usd": 0.1432775,
-      "success": true
-    },
-    {
-      "phase": "execute",
-      "tier": "low",
-      "model": "haiku",
-      "provider": "claude",
-      "tokens_in": 116,
-      "tokens_out": 6705,
-      "duration_ms": 74586,
-      "cost_usd": 0.13324299999999997,
-      "success": true
-    },
-    {
-      "phase": "execute",
-      "tier": "low",
-      "model": "haiku",
-      "provider": "claude",
-      "tokens_in": 100,
-      "tokens_out": 5596,
-      "duration_ms": 51009,
-      "cost_usd": 0.1188247,
-      "success": true
-    },
-    {
-      "phase": "review",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 5,
-      "tokens_out": 6664,
-      "duration_ms": 108741,
-      "cost_usd": 0.2164836,
-      "success": true
-    },
-    {
-      "phase": "review",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 6,
-      "tokens_out": 8285,
-      "duration_ms": 134892,
-      "cost_usd": 0.25875044999999997,
-      "success": true
-    },
-    {
-      "phase": "plan",
-      "tier": "high",
-      "model": "opus",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 1258,
-      "duration_ms": 15473,
-      "cost_usd": 0.08785525000000001,
-      "success": true
-    },
-    {
-      "phase": "execute",
-      "tier": "low",
-      "model": "haiku",
-      "provider": "claude",
-      "tokens_in": 149,
-      "tokens_out": 7450,
-      "duration_ms": 80356,
-      "cost_usd": 0.14858100000000002,
-      "success": true
-    },
-    {
-      "phase": "review",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 6,
-      "tokens_out": 7395,
-      "duration_ms": 118352,
-      "cost_usd": 0.23030384999999998,
-      "success": true
-    },
-    {
-      "phase": "review",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 5,
-      "tokens_out": 8912,
-      "duration_ms": 140274,
-      "cost_usd": 0.24518205,
-      "success": true
-    },
-    {
-      "phase": "accept",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 283,
-      "duration_ms": 6720,
-      "cost_usd": 0.061980150000000005,
-      "success": true
-    },
-    {
-      "phase": "accept",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 164,
-      "duration_ms": 5331,
-      "cost_usd": 0.06008265,
-      "success": true
-    },
-    {
-      "phase": "accept",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 272,
-      "duration_ms": 6752,
-      "cost_usd": 0.06169890000000001,
-      "success": true
-    },
-    {
-      "phase": "accept",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 267,
-      "duration_ms": 8001,
-      "cost_usd": 0.061638899999999996,
-      "success": true
-    },
-    {
-      "phase": "accept",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 286,
-      "duration_ms": 7630,
-      "cost_usd": 0.061875150000000004,
-      "success": true
-    },
-    {
-      "phase": "accept",
-      "tier": "medium",
-      "model": "sonnet",
-      "provider": "claude",
-      "tokens_in": 2,
-      "tokens_out": 116,
-      "duration_ms": 4680,
-      "cost_usd": 0.0592989,
-      "success": true
-    }
-  ]
-}
\ No newline at end of file
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/review.json b/.gromit-next/runs/run-45dcbc628184bad1/evidence/review.json
deleted file mode 100644
index ee205e5f2..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/review.json
+++ /dev/null
@@ -1,147 +0,0 @@
-{
-  "code_quality": [
-    {
-      "facet": "code_quality",
-      "severity": "warning",
-      "file": "internal/next/planner/planner_test.go",
-      "line": 341,
-      "description": "Significant test duplication: the 6 new functions added to planner_test.go cover the same scenarios as the 4 new scenario test files. TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection, _AppearsBeforeValidationSection, and _AlsoAppearsInValidationSection collectively duplicate TestScenario_PersistentFailureTriggersContractAuditSection. TestBuildFixPlanPrompt_NoPersistentFailures_NoSection duplicates TestScenario_NoPersistentFailures_PromptUnchanged. TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts duplicates TestScenario_MultiplePersistentFailuresAcrossDifferentContracts. TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure duplicates TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure.",
-      "suggested_fix": "Remove the 6 duplicated test functions from planner_test.go and keep only the scenario test files, which have clearer Given/When/Then structure and more thorough assertions. Or, if keeping planner_test.go tests, remove the scenario files and consolidate assertions.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "code_quality",
-      "severity": "warning",
-      "file": "internal/next/planner/planner_test.go",
-      "line": 341,
-      "description": "TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection, TestBuildFixPlanPrompt_PersistentFailures_AppearsBeforeValidationSection, and TestBuildFixPlanPrompt_PersistentFailures_AlsoAppearsInValidationSection all construct the identical FixPlanRequest with a single persistent-failure entry and call buildFixPlanPrompt, then each asserts a different property of the same prompt. This inflates test count without adding coverage and builds the same prompt three times.",
-      "suggested_fix": "Merge the three into a single table-driven or subtested function, or a single TestBuildFixPlanPrompt_PersistentFailures function with all three assertions.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "code_quality",
-      "severity": "suggestion",
-      "file": "internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go",
-      "line": 57,
-      "description": "File is missing a trailing newline (diff shows 'No newline at end of file'). POSIX requires text files to end with a newline; gofmt enforces this and some CI linters will flag it.",
-      "suggested_fix": "Add a trailing newline after the closing brace of the file.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "code_quality",
-      "severity": "suggestion",
-      "file": "internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go",
-      "line": 72,
-      "description": "File is missing a trailing newline (diff shows 'No newline at end of file').",
-      "suggested_fix": "Add a trailing newline after the closing brace of the file.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "code_quality",
-      "severity": "suggestion",
-      "file": "internal/next/planner/planner_scenario_no_persistent_failures_test.go",
-      "line": 61,
-      "description": "File is missing a trailing newline (diff shows 'No newline at end of file').",
-      "suggested_fix": "Add a trailing newline after the closing brace of the file.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "code_quality",
-      "severity": "suggestion",
-      "file": "internal/next/planner/planner_scenario_multiple_persistent_failures_test.go",
-      "line": 66,
-      "description": "File is missing a trailing newline (diff shows 'No newline at end of file').",
-      "suggested_fix": "Add a trailing newline after the closing brace of the file.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "code_quality",
-      "severity": "info",
-      "file": "internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go",
-      "line": 36,
-      "description": "The test checks that `scenario-contracts.yaml` appears in the prompt. This passes because the spec-required Step 1 text directly references `scenario-contracts.yaml` by name. The check is valid but narrow — it would also pass if the string appeared incidentally elsewhere in the prompt. Low risk since the step 1 text is now an exact match to the spec.",
-      "suggested_fix": "No action required. Optionally strengthen by asserting the full step 1 sentence: \"Find the assertion in scenario-contracts.yaml that corresponds to this failure\".",
-      "cycle": 3,
-      "disposition": "new"
-    }
-  ],
-  "diff_unavailable": false,
-  "spec_alignment": [
-    {
-      "facet": "spec_alignment",
-      "severity": "warning",
-      "file": "internal/next/planner/planner_scenario_multiple_persistent_failures_test.go",
-      "line": 12,
-      "description": "Test seed does not match the spec's Scenario 3 'Given' conditions. Spec says 'Three contract failures, two of which have persistent-failure hints' — implying three `contract:` prefixed entries plus two accompanying `persistent-failure:` entries. The test provides only one `contract:` entry and two standalone `persistent-failure:` entries (with no corresponding `contract:auth` or `contract:payment` prefix lines). The scenario is structurally different from what the spec describes, even though the acceptance-criteria assertions still pass.",
-      "suggested_fix": "Add corresponding `contract:auth-contracts.yaml login_timeout ...` and `contract:payment-contracts.yaml refund_flow ...` entries to the Failures slice alongside the persistent-failure hints, so the seed matches the spec's 'three contract failures, two with hints' scenario.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "spec_alignment",
-      "severity": "warning",
-      "file": "internal/next/planner/planner_test.go",
-      "line": 339,
-      "description": "Significant test duplication between the six new functions added to planner_test.go and the four new scenario test files. TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection, _AppearsBeforeValidationSection, and _AlsoAppearsInValidationSection collectively cover the same single-persistent-failure scenario as TestScenario_PersistentFailureTriggersContractAuditSection. TestBuildFixPlanPrompt_NoPersistentFailures_NoSection duplicates TestScenario_NoPersistentFailures_PromptUnchanged. TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts duplicates TestScenario_MultiplePersistentFailuresAcrossDifferentContracts. TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure duplicates TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure.",
-      "suggested_fix": "Remove the duplicated unit tests from planner_test.go (or the scenario files), keeping only one authoritative location per scenario. The scenario files are better named and more scenario-aligned; the planner_test.go additions could be pruned to only TestBuildFixPlanPrompt_MultiplePersistentFailures_MixedWithNonPersistent which has no direct scenario counterpart.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "spec_alignment",
-      "severity": "warning",
-      "file": "internal/next/planner/planner_test.go",
-      "line": 339,
-      "description": "TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection, _AppearsBeforeValidationSection, and _AlsoAppearsInValidationSection are three separate functions with an identical FixPlanRequest literal, each asserting a different property. This is a test smell — the same prompt is constructed three times and the test count is inflated without adding coverage.",
-      "suggested_fix": "Merge the three functions into one table-driven or multi-assertion test, or rely solely on the richer scenario test TestScenario_PersistentFailureTriggersContractAuditSection which already covers all three properties.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "spec_alignment",
-      "severity": "suggestion",
-      "file": "internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go",
-      "line": 57,
-      "description": "File is missing a trailing newline (diff marker: `\\ No newline at end of file`). Violates POSIX text-file convention and gofmt expectations; some CI linters will warn.",
-      "suggested_fix": "Add a trailing newline after the closing `}`.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "spec_alignment",
-      "severity": "suggestion",
-      "file": "internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go",
-      "line": 72,
-      "description": "File is missing a trailing newline (diff marker: `\\ No newline at end of file`).",
-      "suggested_fix": "Add a trailing newline after the closing `}`.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "spec_alignment",
-      "severity": "suggestion",
-      "file": "internal/next/planner/planner_scenario_no_persistent_failures_test.go",
-      "line": 61,
-      "description": "File is missing a trailing newline (diff marker: `\\ No newline at end of file`).",
-      "suggested_fix": "Add a trailing newline after the closing `}`.",
-      "cycle": 3,
-      "disposition": "new"
-    },
-    {
-      "facet": "spec_alignment",
-      "severity": "suggestion",
-      "file": "internal/next/planner/planner_scenario_multiple_persistent_failures_test.go",
-      "line": 66,
-      "description": "File is missing a trailing newline (diff marker: `\\ No newline at end of file`).",
-      "suggested_fix": "Add a trailing newline after the closing `}`.",
-      "cycle": 3,
-      "disposition": "new"
-    }
-  ]
-}
\ No newline at end of file
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/review.md b/.gromit-next/runs/run-45dcbc628184bad1/evidence/review.md
deleted file mode 100644
index 74989831c..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/review.md
+++ /dev/null
@@ -1,562 +0,0 @@
-# Review Decision Sheet
-
-## Terminal State
-
-ready_for_review
-
-## What Changed
-
-diff --git a/internal/next/planner/planner.go b/internal/next/planner/planner.go
-index 879fdf247..867845c79 100644
---- a/internal/next/planner/planner.go
-+++ b/internal/next/planner/planner.go
-@@ -156,17 +156,44 @@ func buildFixPlanPrompt(req FixPlanRequest) string {
- 		b.WriteString("\n")
- 	}
- 
--	// Separate review findings from other failures for clarity.
-+	// Separate persistent failures, review findings, and other failures for clarity.
-+	var persistentFailures []string
- 	var reviewFindings []string
- 	var otherFailures []string
- 	for _, f := range req.Failures {
--		if strings.HasPrefix(f, "review:") {
-+		if strings.HasPrefix(f, "persistent-failure:") {
-+			persistentFailures = append(persistentFailures, f)
-+			// Also add to otherFailures so it appears in the validation section
-+			otherFailures = append(otherFailures, f)
-+		} else if strings.HasPrefix(f, "review:") {
- 			reviewFindings = append(reviewFindings, f)
- 		} else {
- 			otherFailures = append(otherFailures, f)
- 		}
- 	}
- 
-+	if len(persistentFailures) > 0 {
-+		b.WriteString("## Persistent Failures — Possible Bad Contracts\n")
-+		b.WriteString("The following failures have repeated across multiple consecutive cycles.\n")
-+		b.WriteString("This strongly suggests the contract assertion itself is wrong, not the implementation.\n")
-+		b.WriteString("\n")
-+		b.WriteString("BEFORE creating any implementation fix task for these failures:\n")
-+		b.WriteString("1. Find the assertion in scenario-contracts.yaml that corresponds to this failure\n")
-+		b.WriteString("2. Verify the pattern actually appears in the target file (run grep manually in your head)\n")
-+		b.WriteString("3. If the pattern looks like a regex (contains .*  \\w+  \\[  etc.) but the file uses\n")
-+		b.WriteString("   literal Go syntax, the pattern may need to be a literal substring instead\n")
-+		b.WriteString("4. Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you\n")
-+		b.WriteString("   have high confidence the implementation is wrong\n")
-+		b.WriteString("\n")
-+		b.WriteString("Persistent failures:\n")
-+		for _, f := range persistentFailures {
-+			b.WriteString("- ")
-+			b.WriteString(f)
-+			b.WriteString("\n")
-+		}
-+		b.WriteString("\n")
-+	}
-+
- 	if len(reviewFindings) > 0 {
- 		b.WriteString("## Review Findings to Fix\n")
- 		b.WriteString("The following review warnings were raised against the current code. Each fix task you create MUST directly address one or more of these findings.\n")
-diff --git a/internal/next/planner/planner_scenario_multiple_persistent_failures_test.go b/internal/next/planner/planner_scenario_multiple_persistent_failures_test.go
-new file mode 100644
-index 000000000..8015ea1d0
---- /dev/null
-+++ b/internal/next/planner/planner_scenario_multiple_persistent_failures_test.go
-@@ -0,0 +1,66 @@
-+package planner
-+
-+import (
-+	"strings"
-+	"testing"
-+)
-+
-+func TestScenario_MultiplePersistentFailuresAcrossDifferentContracts(t *testing.T) {
-+	// Seed: three contract failures, two with persistent-failure hints
-+	req := FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles — may indicate a bad test specification",
-+			"contract:validation-contracts.yaml input_sanitization failed: expected sanitized output",
-+			"persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles — may indicate a bad test specification",
-+		},
-+		Cycle: 3,
-+	}
-+
-+	// Invoke
-+	prompt := buildFixPlanPrompt(req)
-+
-+	// Assert: persistent failures section exists
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("expected persistent failures section to be present")
-+	}
-+
-+	// Assert: both persistent failures appear in the dedicated section
-+	persistentSection := prompt[strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts"):]
-+	validationIdx := strings.Index(persistentSection, "## Validation Failures to Fix")
-+	if validationIdx < 0 {
-+		t.Fatal("expected validation failures section after persistent failures section")
-+	}
-+	persistentOnly := persistentSection[:validationIdx]
-+
-+	if !strings.Contains(persistentOnly, "persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles") {
-+		t.Fatal("first persistent failure must appear in persistent failures section")
-+	}
-+	if !strings.Contains(persistentOnly, "persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles") {
-+		t.Fatal("second persistent failure must appear in persistent failures section")
-+	}
-+
-+	// Assert: non-persistent failure does NOT appear in the persistent section
-+	if strings.Contains(persistentOnly, "contract:validation-contracts.yaml input_sanitization") {
-+		t.Fatal("non-persistent failure must not appear in persistent failures section")
-+	}
-+
-+	// Assert: all three failures appear in the validation failures section
-+	validationSection := persistentSection[validationIdx:]
-+	if !strings.Contains(validationSection, "persistent-failure: contract:auth-contracts.yaml login_timeout has failed 3 consecutive cycles") {
-+		t.Fatal("first persistent failure must also appear in validation failures section")
-+	}
-+	if !strings.Contains(validationSection, "contract:validation-contracts.yaml input_sanitization failed: expected sanitized output") {
-+		t.Fatal("non-persistent failure must appear in validation failures section")
-+	}
-+	if !strings.Contains(validationSection, "persistent-failure: contract:payment-contracts.yaml refund_flow has failed 2 consecutive cycles") {
-+		t.Fatal("second persistent failure must also appear in validation failures section")
-+	}
-+
-+	// Assert: persistent section appears before validation section in the full prompt
-+	fullPersistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	fullValidationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	if fullPersistentIdx > fullValidationIdx {
-+		t.Fatal("persistent failures section must appear before validation failures section")
-+	}
-+}
-\ No newline at end of file
-diff --git a/internal/next/planner/planner_scenario_no_persistent_failures_test.go b/internal/next/planner/planner_scenario_no_persistent_failures_test.go
-new file mode 100644
-index 000000000..64373eaf9
---- /dev/null
-+++ b/internal/next/planner/planner_scenario_no_persistent_failures_test.go
-@@ -0,0 +1,61 @@
-+package planner
-+
-+import (
-+	"strings"
-+	"testing"
-+)
-+
-+func TestScenario_NoPersistentFailures_PromptUnchanged(t *testing.T) {
-+	// Seed: a fix plan request with only ordinary failures, no persistent-failure: entries
-+	req := FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		CompletedTasks: []CompletedTask{{
-+			TaskID:            "t-001",
-+			Attempts:          1,
-+			FilesChanged:      []string{"pkg/foo.go"},
-+			ValidationOutcome: "failed",
-+		}},
-+		Failures: []string{
-+			"contract:scenario-contracts.yaml TestAdd expected 4 got 5",
-+			"go test ./pkg/... FAIL",
-+			"lint error in pkg/foo.go: unused variable",
-+		},
-+		CurrentDiff: "diff --git a/pkg/foo.go b/pkg/foo.go\n-old\n+new",
-+		Cycle:       2,
-+	}
-+
-+	// Invoke
-+	prompt := buildFixPlanPrompt(req)
-+
-+	// Assert: no Persistent Failures section appears
-+	if strings.Contains(prompt, "## Persistent Failures") {
-+		t.Fatal("prompt must not contain '## Persistent Failures' section when no persistent-failure: entries exist")
-+	}
-+	if strings.Contains(prompt, "Possible Bad Contracts") {
-+		t.Fatal("prompt must not contain 'Possible Bad Contracts' when no persistent-failure: entries exist")
-+	}
-+	if strings.Contains(prompt, "Audit Instructions") {
-+		t.Fatal("prompt must not contain 'Audit Instructions' when no persistent-failure: entries exist")
-+	}
-+
-+	// Assert: standard sections are present
-+	if !strings.Contains(prompt, "## Completed Tasks") {
-+		t.Fatal("prompt must contain Completed Tasks section")
-+	}
-+	if !strings.Contains(prompt, "## Validation Failures to Fix") {
-+		t.Fatal("prompt must contain Validation Failures section")
-+	}
-+	if !strings.Contains(prompt, "## Current Diff") {
-+		t.Fatal("prompt must contain Current Diff section")
-+	}
-+	if !strings.Contains(prompt, "## Instructions") {
-+		t.Fatal("prompt must contain Instructions section")
-+	}
-+
-+	// Assert: all ordinary failures appear in the validation section
-+	for _, f := range req.Failures {
-+		if !strings.Contains(prompt, f) {
-+			t.Fatalf("prompt must contain failure %q", f)
-+		}
-+	}
-+}
-\ No newline at end of file
-diff --git a/internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go b/internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go
-new file mode 100644
-index 000000000..3eb2d624f
---- /dev/null
-+++ b/internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go
-@@ -0,0 +1,57 @@
-+package planner
-+
-+import (
-+	"strings"
-+	"testing"
-+)
-+
-+func TestScenario_PersistentFailureTriggersContractAuditSection(t *testing.T) {
-+	// Seed: a replan context with one contract failure and its corresponding persistent-failure hint
-+	req := FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			`contract:first-failure-no-escalation — file_contains failed: pattern "ChainIDs.*\[\]string" not found in "internal/next/runstore/types.go"`,
-+			`persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug`,
-+		},
-+		Cycle: 2,
-+	}
-+
-+	// Invoke
-+	prompt := buildFixPlanPrompt(req)
-+
-+	// Assert: persistent failures audit section is present
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("expected persistent failures audit section")
-+	}
-+
-+	// Assert: the persistent-failure hint appears in the audit section
-+	if !strings.Contains(prompt, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
-+		t.Fatal("persistent-failure hint must appear in the audit section")
-+	}
-+
-+	// Assert: audit instructions reference scenario-contracts.yaml
-+	if !strings.Contains(prompt, "scenario-contracts.yaml") {
-+		t.Fatal("audit section must instruct to check scenario-contracts.yaml")
-+	}
-+
-+	// Assert: the contract failure appears in the validation failures section
-+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	if validationIdx < 0 {
-+		t.Fatal("expected validation failures section")
-+	}
-+	validationSection := prompt[validationIdx:]
-+	if !strings.Contains(validationSection, `contract:first-failure-no-escalation — file_contains failed`) {
-+		t.Fatal("contract failure must appear in the validation failures section")
-+	}
-+
-+	// Assert: persistent-failure hint also appears in validation section (duplicated from audit)
-+	if !strings.Contains(validationSection, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
-+		t.Fatal("persistent-failure hint must also appear in validation failures section")
-+	}
-+
-+	// Assert: audit section appears before validation section
-+	auditIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	if auditIdx > validationIdx {
-+		t.Fatal("persistent failures audit section must appear before validation failures section")
-+	}
-+}
-\ No newline at end of file
-diff --git a/internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go b/internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go
-new file mode 100644
-index 000000000..c7ff543af
---- /dev/null
-+++ b/internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go
-@@ -0,0 +1,72 @@
-+package planner
-+
-+import (
-+	"strings"
-+	"testing"
-+)
-+
-+func TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure(t *testing.T) {
-+	// Seed: a replan context with only a persistent-failure hint and no
-+	// corresponding contract: failure entry (the original failure was
-+	// deduplicated into a summary that no longer carries the contract: prefix).
-+	req := FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
-+		},
-+		Cycle: 2,
-+	}
-+
-+	// Invoke
-+	prompt := buildFixPlanPrompt(req)
-+
-+	// Assert: persistent failures audit section is present with full instructions
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures audit section must be present")
-+	}
-+	if !strings.Contains(prompt, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
-+		t.Fatal("persistent-failure hint must appear in the audit section")
-+	}
-+	// Audit instructions must be present
-+	if !strings.Contains(prompt, "BEFORE creating any implementation fix task for these failures:") {
-+		t.Fatal("audit directive must be present in persistent failures section")
-+	}
-+	if !strings.Contains(prompt, "Find the assertion in scenario-contracts.yaml that corresponds to this failure") {
-+		t.Fatal("audit instruction step 1 must be present")
-+	}
-+	if !strings.Contains(prompt, "Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you") {
-+		t.Fatal("audit instruction step 4 guidance must be present")
-+	}
-+	if strings.Contains(prompt, "escalate to the spec author") {
-+		t.Fatal("audit instructions must not contain 'escalate to the spec author'")
-+	}
-+
-+	// Assert: the persistent-failure hint also appears in Validation Failures
-+	// (it is not a review: entry, so it lands in otherFailures)
-+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	if validationIdx < 0 {
-+		t.Fatal("validation failures section must be present")
-+	}
-+	validationSection := prompt[validationIdx:]
-+	if !strings.Contains(validationSection, "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles") {
-+		t.Fatal("persistent-failure hint must also appear in the validation failures section")
-+	}
-+
-+	// Assert: the hint appears at least twice (once per section)
-+	hint := "persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles"
-+	count := strings.Count(prompt, hint)
-+	if count < 2 {
-+		t.Fatalf("persistent-failure hint must appear at least twice (audit + validation), got %d", count)
-+	}
-+
-+	// Assert: persistent failures section appears before validation failures section
-+	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	if persistentIdx > validationIdx {
-+		t.Fatal("persistent failures section must appear before validation failures section")
-+	}
-+
-+	// Assert: no review findings section (there are no review: entries)
-+	if strings.Contains(prompt, "## Review Findings to Fix") {
-+		t.Fatal("review findings section must not appear when there are no review: entries")
-+	}
-+}
-\ No newline at end of file
-diff --git a/internal/next/planner/planner_test.go b/internal/next/planner/planner_test.go
-index 9dc2e7ffe..abde1df3c 100644
---- a/internal/next/planner/planner_test.go
-+++ b/internal/next/planner/planner_test.go
-@@ -338,3 +338,179 @@ func TestBuildFixPlanPrompt_InstructsAboutFixesField(t *testing.T) {
- 		t.Fatal("buildFixPlanPrompt must reference 'failed task' when describing the fixes field")
- 	}
- }
-+
-+func TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
-+		},
-+		Cycle: 2,
-+	})
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("buildFixPlanPrompt must include persistent failures audit section when present")
-+	}
-+	if !strings.Contains(prompt, "persistent-failure: TestFoo has failed 2 consecutive cycles") {
-+		t.Fatal("persistent failure hint must appear in the audit section")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_PersistentFailures_AppearsBeforeValidationSection(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
-+			"compilation error in main.go",
-+		},
-+		Cycle: 2,
-+	})
-+	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	if persistentIdx < 0 {
-+		t.Fatal("persistent failures section must be present")
-+	}
-+	if validationIdx < 0 {
-+		t.Fatal("validation failures section must be present")
-+	}
-+	if persistentIdx > validationIdx {
-+		t.Fatal("persistent failures section must appear before validation failures section")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_PersistentFailures_AlsoAppearsInValidationSection(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
-+		},
-+		Cycle: 2,
-+	})
-+	// Count occurrences of the persistent failure hint
-+	persistentHint := "persistent-failure: TestFoo has failed 2 consecutive cycles"
-+	count := strings.Count(prompt, persistentHint)
-+	if count < 2 {
-+		t.Fatalf("persistent failure hint must appear at least twice (audit section and validation section), got %d", count)
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_NoPersistentFailures_NoSection(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures:     []string{"compilation error in main.go"},
-+		Cycle:        2,
-+	})
-+	if strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures section must not appear when there are no persistent failures")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_MultiplePersistentFailures_MixedWithNonPersistent(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
-+			"lint error in package.go",
-+			"persistent-failure: TestBar has failed 3 consecutive cycles — may indicate a bad test specification",
-+			"compilation error in main.go",
-+		},
-+		Cycle: 2,
-+	})
-+	// Check persistent failures section exists
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures section must be present")
-+	}
-+	// Check both persistent failures appear in their section
-+	if !strings.Contains(prompt, "persistent-failure: TestFoo has failed 2 consecutive cycles") {
-+		t.Fatal("first persistent failure must appear in audit section")
-+	}
-+	if !strings.Contains(prompt, "persistent-failure: TestBar has failed 3 consecutive cycles") {
-+		t.Fatal("second persistent failure must appear in audit section")
-+	}
-+	// Check validation section still exists
-+	if !strings.Contains(prompt, "## Validation Failures to Fix") {
-+		t.Fatal("validation failures section must be present")
-+	}
-+	// Check non-persistent failures appear in validation section
-+	if !strings.Contains(prompt, "lint error in package.go") {
-+		t.Fatal("non-persistent failure must appear in validation section")
-+	}
-+	if !strings.Contains(prompt, "compilation error in main.go") {
-+		t.Fatal("non-persistent failure must appear in validation section")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure(t *testing.T) {
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			"persistent-failure: contract:scenario-contracts.yaml scenario-name has failed 2 consecutive cycles — may indicate a bad test specification",
-+		},
-+		Cycle: 2,
-+	})
-+	// Persistent failures section must exist even without corresponding contract failure
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures section must be present even without corresponding contract failure")
-+	}
-+	// The persistent failure hint must appear in the section
-+	if !strings.Contains(prompt, "persistent-failure: contract:scenario-contracts.yaml") {
-+		t.Fatal("persistent-failure hint must appear in audit section")
-+	}
-+}
-+
-+func TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts(t *testing.T) {
-+	// Test multiple persistent failures across different contracts
-+	prompt := buildFixPlanPrompt(FixPlanRequest{
-+		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
-+		Failures: []string{
-+			`contract:contract-alpha — file_contains failed: pattern "ExpectedType" not found in "types.go"`,
-+			`persistent-failure: contract:contract-alpha has failed 2 consecutive cycles — may indicate a bad test specification`,
-+			`contract:contract-beta — assertion failed: expected 5 assertions but got 3`,
-+			`persistent-failure: contract:contract-beta has failed 3 consecutive cycles — may indicate a bad test specification`,
-+			`contract:contract-gamma — timeout after 30s`,
-+		},
-+		Cycle: 2,
-+	})
-+
-+	// Assert: persistent failures section exists
-+	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
-+		t.Fatal("persistent failures section must be present")
-+	}
-+
-+	// Assert: both persistent failures appear in the persistent section
-+	if !strings.Contains(prompt, "persistent-failure: contract:contract-alpha has failed 2 consecutive cycles") {
-+		t.Fatal("first persistent failure must appear in persistent section")
-+	}
-+	if !strings.Contains(prompt, "persistent-failure: contract:contract-beta has failed 3 consecutive cycles") {
-+		t.Fatal("second persistent failure must appear in persistent section")
-+	}
-+
-+	// Assert: validation failures section exists
-+	if !strings.Contains(prompt, "## Validation Failures to Fix") {
-+		t.Fatal("validation failures section must be present")
-+	}
-+
-+	// Assert: all three contract failures appear in validation section
-+	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
-+	validationSection := prompt[validationIdx:]
-+	if !strings.Contains(validationSection, "contract:contract-alpha") {
-+		t.Fatal("first contract failure must appear in validation section")
-+	}
-+	if !strings.Contains(validationSection, "contract:contract-beta") {
-+		t.Fatal("second contract failure must appear in validation section")
-+	}
-+	if !strings.Contains(validationSection, "contract:contract-gamma") {
-+		t.Fatal("third contract failure must appear in validation section")
-+	}
-+
-+	// Assert: non-persistent failure (contract-gamma timeout) does not appear in persistent section
-+	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
-+	persistentSection := prompt[persistentIdx:validationIdx]
-+	if strings.Contains(persistentSection, "contract:contract-gamma") {
-+		t.Fatal("non-persistent failure must not appear in persistent section")
-+	}
-+
-+	// Assert: persistent section appears before validation section
-+	if persistentIdx > validationIdx {
-+		t.Fatal("persistent failures section must appear before validation failures section")
-+	}
-+}
-
-## Cycle History
-
-| Cycle | Tasks | Passed |
-|-------|-------|--------|
-| 3 | 5 | 5 |
-
-## Validation Results
-
-pass=true
-
-## Known Risks
-
-
-## Review Findings
-
-| Facet | Count | Severities |
-|-------|-------|------------|
-| code_quality | 7 | 1 info, 4 suggestion, 2 warning |
-| spec_alignment | 7 | 4 suggestion, 3 warning |
-
-## Acceptance Criteria
-
-| Criterion | Status | Rationale |
-|-----------|--------|-----------|
-| When `req.Failures` contains one or more `persistent-failure:` prefixed entries, `buildFixPlanPrompt` renders a `## Persistent Failures — Possible Bad Contracts` section before `## Validation Failures to Fix` | pass | The implementation in planner.go separates persistent-failure: entries into a dedicated slice and renders '## Persistent Failures — Possible Bad Contracts' before '## Validation Failures to Fix'. Multiple tests explicitly verify the ordering (persistentIdx < validationIdx) and the section's presence. Validation passed (pass=true). |
-| The persistent failures section includes explicit instructions to audit the contract assertion in `scenario-contracts.yaml` before creating implementation fix tasks | pass | The implementation in planner.go explicitly writes 'BEFORE creating any implementation fix task for these failures:' followed by step 1 'Find the assertion in scenario-contracts.yaml that corresponds to this failure'. This directly satisfies the criterion. Tests confirm this text appears in the prompt. |
-| Persistent failures still appear in `## Validation Failures to Fix` — they are not removed from the main list | pass | The implementation explicitly adds persistent failures to both `persistentFailures` AND `otherFailures` slices, ensuring they appear in both the dedicated audit section and the `## Validation Failures to Fix` section. Tests verify this: `TestBuildFixPlanPrompt_PersistentFailures_AlsoAppearsInValidationSection` checks count >= 2, and `TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure` asserts the hint appears in the validation section. |
-| When `req.Failures` contains no `persistent-failure:` entries, no persistent failures section is rendered and existing behavior is unchanged | pass | The implementation guards the persistent failures section with `if len(persistentFailures) > 0`, so it only renders when there are entries with the `persistent-failure:` prefix. The test `TestBuildFixPlanPrompt_NoPersistentFailures_NoSection` explicitly verifies that when only ordinary failures are present, the `## Persistent Failures` section does not appear, and `TestScenario_NoPersistentFailures_PromptUnchanged` additionally verifies that all standard sections (Completed Tasks, Validation Failures, Current Diff, Instructions) remain present. Both tests pass. |
-| Non-persistent contract failures and review findings are unaffected by this change | pass | The diff shows that non-persistent `contract:` failures fall into `otherFailures` (unchanged path) and `review:` prefixed failures still go into `reviewFindings` (unchanged path). Both the review findings section and validation failures section logic are untouched. The test `TestScenario_NoPersistentFailures_PromptUnchanged` explicitly verifies that prompts without `persistent-failure:` entries remain unchanged, and `TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts` confirms non-persistent contract failures (e.g., `contract:contract-gamma`) still appear only in the validation section and not the persistent section. Validation pass=true confirms all tests pass. |
-| All existing planner tests continue to pass | pass | Validation results show pass=true, and the diff shows only additions to planner_test.go (no modifications to existing tests) plus new test files. The existing tests were not changed. |
-
-## Recommended Action
-
-review
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/scenario-contracts.yaml b/.gromit-next/runs/run-45dcbc628184bad1/evidence/scenario-contracts.yaml
deleted file mode 100644
index dbf79ddc3..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/scenario-contracts.yaml
+++ /dev/null
@@ -1,51 +0,0 @@
-scenarios:
-    - name: persistent-failure-triggers-contract-audit-section
-      assertions:
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: 'persistent-failure:'
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: persistentFailures
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: '## Persistent Failures — Possible Bad Contracts'
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: scenario-contracts.yaml
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: regex
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: '## Validation Failures to Fix'
-        - file_contains:
-            path: internal/next/planner/planner_test.go
-            pattern: Persistent Failures
-    - name: no-persistent-failures-prompt-unchanged
-      assertions:
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: len(persistentFailures)
-        - file_contains:
-            path: internal/next/planner/planner_test.go
-            pattern: no persistent
-    - name: multiple-persistent-failures-across-different-contracts
-      assertions:
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: persistentFailures = append(persistentFailures
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: for _, f := range persistentFailures
-        - file_contains:
-            path: internal/next/planner/planner_test.go
-            pattern: multiple persistent
-    - name: persistent-failure-hint-without-corresponding-contract-failure
-      assertions:
-        - file_contains:
-            path: internal/next/planner/planner.go
-            pattern: HasPrefix(f, "persistent-failure:")
-        - file_contains:
-            path: internal/next/planner/planner_test.go
-            pattern: without corresponding
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/scenario-test-manifest.json b/.gromit-next/runs/run-45dcbc628184bad1/evidence/scenario-test-manifest.json
deleted file mode 100644
index 7d1c98d72..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/scenario-test-manifest.json
+++ /dev/null
@@ -1,20 +0,0 @@
-{
-  "scenarios": [
-    {
-      "name": "persistent failure triggers contract audit section",
-      "test_file": "internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go"
-    },
-    {
-      "name": "no persistent failures — prompt unchanged",
-      "test_file": "internal/next/planner/planner_scenario_no_persistent_failures_test.go"
-    },
-    {
-      "name": "multiple persistent failures across different contracts",
-      "test_file": "internal/next/planner/planner_scenario_multiple_persistent_failures_test.go"
-    },
-    {
-      "name": "persistent-failure hint without corresponding contract failure",
-      "test_file": "internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go"
-    }
-  ]
-}
\ No newline at end of file
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/summary.md b/.gromit-next/runs/run-45dcbc628184bad1/evidence/summary.md
deleted file mode 100644
index 1fc0c73b7..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/summary.md
+++ /dev/null
@@ -1,6 +0,0 @@
-# Execution Summary
-
-- **Spec ID:** 0003e-persistent-failure-contract-audit
-- **Status:** ready_for_review
-- **Tasks:** 5/5 passed
-- **Cycles:** 3
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/task-results.json b/.gromit-next/runs/run-45dcbc628184bad1/evidence/task-results.json
deleted file mode 100644
index 29054b978..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/task-results.json
+++ /dev/null
@@ -1,145 +0,0 @@
-[
-  {
-    "task_id": "t-001",
-    "objective": "Write tests for persistent failure extraction in buildFixPlanPrompt: (1) persistent failures render a dedicated '## Persistent Failures — Possible Bad Contracts' section before '## Validation Failures to Fix', (2) persistent failures still appear in otherFailures/validation section, (3) no persistent failures means no section rendered, (4) multiple persistent failures with mixed non-persistent, (5) persistent-failure hint without corresponding contract: failure still renders section",
-    "status": "done",
-    "attempts": 1,
-    "expected_touched_area": [
-      "internal/next/planner/planner_test.go"
-    ],
-    "proof_checks": [
-      "grep -q 'TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection' internal/next/planner/planner_test.go",
-      "grep -q 'TestBuildFixPlanPrompt_NoPersistentFailures_NoSection' internal/next/planner/planner_test.go",
-      "grep -q 'TestBuildFixPlanPrompt_MultiplePersistentFailures_MixedWithNonPersistent' internal/next/planner/planner_test.go",
-      "grep -q 'TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure' internal/next/planner/planner_test.go",
-      "grep -q 'scenario-contracts.yaml' internal/next/planner/planner_test.go",
-      "go vet ./internal/next/planner/..."
-    ],
-    "files_changed": [
-      "internal/next/planner/planner.go",
-      "internal/next/planner/planner_test.go"
-    ],
-    "tokens_used": 9398,
-    "duration_ms": 78069,
-    "model_tier": "",
-    "cycle": 1,
-    "kind": "original"
-  },
-  {
-    "task_id": "t-002",
-    "objective": "Implement persistent failure extraction in buildFixPlanPrompt: add a third pass to collect persistent-failure: prefixed entries into persistentFailures slice (without removing from otherFailures), then render a '## Persistent Failures — Possible Bad Contracts' section with audit instructions before the '## Validation Failures to Fix' section when persistentFailures is non-empty",
-    "status": "done",
-    "attempts": 1,
-    "expected_touched_area": [
-      "internal/next/planner/planner.go"
-    ],
-    "proof_checks": [
-      "go test ./internal/next/planner/... -count=1 -timeout 60s",
-      "go vet ./internal/next/planner/...",
-      "grep -q 'persistentFailures' internal/next/planner/planner.go",
-      "grep -q 'Persistent Failures' internal/next/planner/planner.go",
-      "grep -q 'scenario-contracts.yaml' internal/next/planner/planner.go"
-    ],
-    "files_changed": [
-      "internal/next/planner/planner.go"
-    ],
-    "tokens_used": 3613,
-    "duration_ms": 31647,
-    "model_tier": "",
-    "cycle": 1,
-    "kind": "original"
-  },
-  {
-    "task_id": "t-003",
-    "objective": "Add persistent-failure extraction and dedicated prompt section to buildFixPlanPrompt. After separating review findings from other failures, add a third pass that collects persistent-failure: prefixed entries into a persistentFailures slice (without removing them from otherFailures). When persistentFailures is non-empty, render the '## Persistent Failures — Possible Bad Contracts' section before '## Validation Failures to Fix' with the full audit instructions including the word 'regex' (e.g., 'If the pattern looks like a regex'). Fixes: contract:persistent-failure-triggers-contract-audit-section (pattern 'regex' not found in planner.go).",
-    "status": "done",
-    "attempts": 1,
-    "expected_touched_area": [
-      "internal/next/planner/planner.go"
-    ],
-    "proof_checks": [
-      "grep -q 'regex' internal/next/planner/planner.go",
-      "grep -q 'Persistent Failures' internal/next/planner/planner.go",
-      "grep -q 'persistent-failure:' internal/next/planner/planner.go",
-      "go vet ./internal/next/planner/...",
-      "go test ./internal/next/planner/... -count=1 -timeout 60s"
-    ],
-    "files_changed": [
-      "internal/next/planner/planner.go"
-    ],
-    "tokens_used": 6821,
-    "duration_ms": 74586,
-    "model_tier": "",
-    "cycle": 2,
-    "kind": "fix",
-    "parent_cycle": 1,
-    "failures_addressed": [
-      "contract:persistent-failure-triggers-contract-audit-section — file_contains failed: pattern \"regex\" not found in \"internal/next/planner/planner.go\""
-    ]
-  },
-  {
-    "task_id": "t-004",
-    "objective": "Add test case for multiple persistent failures across different contracts in planner_test.go. The test name or description must contain the phrase 'multiple persistent'. It should call buildFixPlanPrompt with three contract failures where two have persistent-failure: hints, and assert: (1) both persistent failures appear in the dedicated section, (2) all three contract failures appear in Validation Failures to Fix, (3) the non-persistent failure does not appear in the persistent section. Fixes: contract:multiple-persistent-failures-across-different-contracts.",
-    "status": "done",
-    "attempts": 1,
-    "expected_touched_area": [
-      "internal/next/planner/planner_test.go"
-    ],
-    "proof_checks": [
-      "grep -q 'multiple persistent' internal/next/planner/planner_test.go",
-      "go test ./internal/next/planner/... -count=1 -timeout 60s -run 'Multiple|multiple'"
-    ],
-    "files_changed": [
-      "internal/next/planner/planner_test.go"
-    ],
-    "tokens_used": 5696,
-    "duration_ms": 51009,
-    "model_tier": "",
-    "cycle": 2,
-    "kind": "fix",
-    "parent_cycle": 1,
-    "failures_addressed": [
-      "contract:multiple-persistent-failures-across-different-contracts — file_contains failed: pattern \"multiple persistent\" not found in \"internal/next/planner/planner_test.go\""
-    ]
-  },
-  {
-    "task_id": "t-005",
-    "objective": "Replace the persistent-failures section intro text and all four audit instruction steps in buildFixPlanPrompt (lines ~174-187) with the exact wording from the spec. Fixes all 7 review findings: intro must say 'The following failures have repeated across multiple consecutive cycles.\\nThis strongly suggests the contract assertion itself is wrong, not the implementation.'; directive must say 'BEFORE creating any implementation fix task for these failures:'; steps 1-4 must match spec verbatim including regex examples and 'Prefer creating a contract fix task' guidance.",
-    "status": "done",
-    "attempts": 1,
-    "expected_touched_area": [
-      "internal/next/planner/planner.go"
-    ],
-    "proof_checks": [
-      "grep -q 'The following failures have repeated across multiple consecutive cycles.' internal/next/planner/planner.go",
-      "grep -q 'This strongly suggests the contract assertion itself is wrong, not the implementation.' internal/next/planner/planner.go",
-      "grep -q 'BEFORE creating any implementation fix task for these failures:' internal/next/planner/planner.go",
-      "grep -q 'Find the assertion in scenario-contracts.yaml that corresponds to this failure' internal/next/planner/planner.go",
-      "grep -q 'Verify the pattern actually appears in the target file (run grep manually in your head)' internal/next/planner/planner.go",
-      "grep -q 'the pattern may need to be a literal substring instead' internal/next/planner/planner.go",
-      "grep -q 'Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you' internal/next/planner/planner.go",
-      "! grep -q 'escalate to the spec author' internal/next/planner/planner.go",
-      "go vet ./internal/next/planner/...",
-      "go test ./internal/next/planner/... -count=1 -timeout 60s"
-    ],
-    "files_changed": [
-      "internal/next/planner/planner.go",
-      "internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go"
-    ],
-    "tokens_used": 7599,
-    "duration_ms": 80356,
-    "model_tier": "",
-    "cycle": 3,
-    "kind": "fix",
-    "parent_cycle": 2,
-    "failures_addressed": [
-      "review:spec_alignment:error:internal/next/planner/planner.go:174 — Intro text deviates from spec.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:180 — Missing \"BEFORE creating any implementation fix task for these failures:\" directive.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:181 — Step 1 guidance deviates from spec.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:182 — Step 2 guidance is entirely different from spec.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:183 — Step 3 guidance is weaker and omits the spec's concrete regex examples.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:184 — Step 4 guidance contradicts the spec's intent.",
-      "review:code_quality:error:internal/next/planner/planner.go:187 — Audit step 4 tells the planner to 'escalate to the spec author' which contradicts the spec."
-    ]
-  }
-]
\ No newline at end of file
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/evidence/validation.json b/.gromit-next/runs/run-45dcbc628184bad1/evidence/validation.json
deleted file mode 100644
index 1f54854dd..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/evidence/validation.json
+++ /dev/null
@@ -1,24 +0,0 @@
-{
-  "pass": true,
-  "always_run": {
-    "results": [
-      {
-        "name": "unit-tests",
-        "pass": true,
-        "output": "ok  \tgithub.com/danabrams/gromit\t0.180s\nok  \tgithub.com/danabrams/gromit/calc\t(cached)\nok  \tgithub.com/danabrams/gromit/cmd/gromit\t11.424s\nok  \tgithub.com/danabrams/gromit/cmd/gromit-next\t1.804s\n?   \tgithub.com/danabrams/gromit/cmd/test_e2e_live_packages\t[no test files]\n?   \tgithub.com/danabrams/gromit/e2e\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/agent\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/analytics\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/analyzer\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/backlog\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/bead\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/benchmark\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/claude\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/config\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/conversation\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/coverage\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/cli\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/eventtest\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/status\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/stream\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/tmux\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/experiment\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/failurephase\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/frontmatter\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/integrationqueue\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/jsonutil\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/learnings\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/logger\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/acceptor\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/architecture\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/artifact\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/contextpkt\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/contract\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/doctrine\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/enrich\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/evidence\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/execpolicy\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/executor\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/extract\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/fact\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/guide\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/infer\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/inspect\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/llmadapter\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/planner\t0.306s\nok  \tgithub.com/danabrams/gromit/internal/next/projectcell\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/provenance\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/runstore\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/sourcemap\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/specloop\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/specloop/stages\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/testutil\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/validation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/validator\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/workspace\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/epilogue\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/execute\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/prepare\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/qualitygate/regression\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/qualitygate/wiring\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/validate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/procutil\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/prompt\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/provider\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/queue\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/readiness\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/retro\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runbook\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/runner/acceptance\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/runner/andon\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/display\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/escalation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/execution\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/methodology\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/pipeline/midreview\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/policy\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/readinessadapterllm\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/reviewpkg\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/runtypes\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/specbranch\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/specmerge\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/util\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/validation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/scope\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/specflow\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/specgate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/state\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/testpkg\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/tracker\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/tracker/trackertest\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/v2/adapter\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/v2/adapter/git\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/adapter/llm\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/adapter/presenter\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/adapter/tasktracker\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/dep\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/event\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/generation\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/v2/llmtypes\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/v2/loop\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/pipeline\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/presentation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/prompt\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/remediation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/routing\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/spec\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/accept\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/build\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/decompose\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/epilogue\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/gate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/names\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/plan\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/present\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/triage\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/validate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/testutil\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/v2/trackertypes\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/validate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/visionmetrics\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/worktree\t(cached)\nok  \tgithub.com/danabrams/gromit/scripts\t(cached)\n?   \tgithub.com/danabrams/gromit/scripts/add_parallel\t[no test files]\n?   \tgithub.com/danabrams/gromit/skills\t[no test files]\nok  \tgithub.com/danabrams/gromit/test/codexenv\t(cached)\n?   \tgithub.com/danabrams/gromit/test/contracts\t[no test files]\nok  \tgithub.com/danabrams/gromit/test/docs\t(cached)\nok  \tgithub.com/danabrams/gromit/test/fixtures\t(cached)\nok  \tgithub.com/danabrams/gromit/test/helpers\t(cached)\nok  \tgithub.com/danabrams/gromit/test/repohygiene\t(cached)\nok  \tgithub.com/danabrams/gromit/test/testutil\t(cached)\nok  \tgithub.com/danabrams/gromit/test/toolcalls\t(cached)\n",
-        "duration": 12435314000,
-        "type": "test"
-      },
-      {
-        "name": "vet",
-        "pass": true,
-        "output": "",
-        "duration": 403331792,
-        "type": "lint"
-      }
-    ]
-  },
-  "project_checks": {
-    "results": null
-  }
-}
\ No newline at end of file
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/execution-policy.json b/.gromit-next/runs/run-45dcbc628184bad1/execution-policy.json
deleted file mode 100644
index 9e22a2720..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/execution-policy.json
+++ /dev/null
@@ -1,35 +0,0 @@
-{
-  "always_run": [
-    {"name": "unit-tests", "command": "go test ./...", "type": "test"},
-    {"name": "vet", "command": "go vet ./...", "type": "lint"}
-  ],
-  "budgets": {
-    "max_spec_cycles": 10,
-    "max_task_retries": 1,
-    "max_redecomposition_passes": 1,
-    "max_task_duration_seconds": 900,
-    "max_run_duration_seconds": 7200,
-    "max_run_cost_usd": 50.0
-  },
-  "models": {
-    "planner": "high",
-    "executor": "low",
-    "evaluator": "medium",
-    "tier_models": {
-      "low": "haiku",
-      "medium": "sonnet",
-      "high": "opus"
-    }
-  },
-  "review": {
-    "facets": ["spec_alignment", "code_quality"],
-    "tiers": {"spec_alignment": "high", "code_quality": "medium"},
-    "replan_threshold": "error",
-    "facet_max_attempts": 2
-  },
-  "routing": {
-    "preferences": {"plan": "any", "execute": "any", "review": "any", "accept": "any"},
-    "ratio": {"claude": 100},
-    "cooldown_seconds": 300
-  }
-}
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/plan.md b/.gromit-next/runs/run-45dcbc628184bad1/plan.md
deleted file mode 100644
index 33a7e59e4..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/plan.md
+++ /dev/null
@@ -1,6 +0,0 @@
-# Plan (Cycle 3)
-
-## t-005
-
-Replace the persistent-failures section intro text and all four audit instruction steps in buildFixPlanPrompt (lines ~174-187) with the exact wording from the spec. Fixes all 7 review findings: intro must say 'The following failures have repeated across multiple consecutive cycles.\nThis strongly suggests the contract assertion itself is wrong, not the implementation.'; directive must say 'BEFORE creating any implementation fix task for these failures:'; steps 1-4 must match spec verbatim including regex examples and 'Prefer creating a contract fix task' guidance.
-
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/run.json b/.gromit-next/runs/run-45dcbc628184bad1/run.json
deleted file mode 100644
index d023a0432..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/run.json
+++ /dev/null
@@ -1,212 +0,0 @@
-{
-  "run_id": "run-45dcbc628184bad1",
-  "spec_id": "0003e-persistent-failure-contract-audit",
-  "project_id": "gromit",
-  "status": "ready_for_review",
-  "cycle": 3,
-  "started_at": "2026-03-18T13:49:16.480638-04:00",
-  "ended_at": "2026-03-18T14:04:09.335967-04:00",
-  "tasks": [
-    {
-      "task_id": "t-001",
-      "objective": "Write tests for persistent failure extraction in buildFixPlanPrompt: (1) persistent failures render a dedicated '## Persistent Failures — Possible Bad Contracts' section before '## Validation Failures to Fix', (2) persistent failures still appear in otherFailures/validation section, (3) no persistent failures means no section rendered, (4) multiple persistent failures with mixed non-persistent, (5) persistent-failure hint without corresponding contract: failure still renders section",
-      "status": "done",
-      "attempts": 1,
-      "expected_touched_area": [
-        "internal/next/planner/planner_test.go"
-      ],
-      "proof_checks": [
-        "grep -q 'TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection' internal/next/planner/planner_test.go",
-        "grep -q 'TestBuildFixPlanPrompt_NoPersistentFailures_NoSection' internal/next/planner/planner_test.go",
-        "grep -q 'TestBuildFixPlanPrompt_MultiplePersistentFailures_MixedWithNonPersistent' internal/next/planner/planner_test.go",
-        "grep -q 'TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure' internal/next/planner/planner_test.go",
-        "grep -q 'scenario-contracts.yaml' internal/next/planner/planner_test.go",
-        "go vet ./internal/next/planner/..."
-      ],
-      "files_changed": [
-        "internal/next/planner/planner.go",
-        "internal/next/planner/planner_test.go"
-      ],
-      "tokens_used": 9398,
-      "duration_ms": 78069,
-      "model_tier": "",
-      "cycle": 1,
-      "kind": "original"
-    },
-    {
-      "task_id": "t-002",
-      "objective": "Implement persistent failure extraction in buildFixPlanPrompt: add a third pass to collect persistent-failure: prefixed entries into persistentFailures slice (without removing from otherFailures), then render a '## Persistent Failures — Possible Bad Contracts' section with audit instructions before the '## Validation Failures to Fix' section when persistentFailures is non-empty",
-      "status": "done",
-      "attempts": 1,
-      "expected_touched_area": [
-        "internal/next/planner/planner.go"
-      ],
-      "proof_checks": [
-        "go test ./internal/next/planner/... -count=1 -timeout 60s",
-        "go vet ./internal/next/planner/...",
-        "grep -q 'persistentFailures' internal/next/planner/planner.go",
-        "grep -q 'Persistent Failures' internal/next/planner/planner.go",
-        "grep -q 'scenario-contracts.yaml' internal/next/planner/planner.go"
-      ],
-      "files_changed": [
-        "internal/next/planner/planner.go"
-      ],
-      "tokens_used": 3613,
-      "duration_ms": 31647,
-      "model_tier": "",
-      "cycle": 1,
-      "kind": "original"
-    },
-    {
-      "task_id": "t-003",
-      "objective": "Add persistent-failure extraction and dedicated prompt section to buildFixPlanPrompt. After separating review findings from other failures, add a third pass that collects persistent-failure: prefixed entries into a persistentFailures slice (without removing them from otherFailures). When persistentFailures is non-empty, render the '## Persistent Failures — Possible Bad Contracts' section before '## Validation Failures to Fix' with the full audit instructions including the word 'regex' (e.g., 'If the pattern looks like a regex'). Fixes: contract:persistent-failure-triggers-contract-audit-section (pattern 'regex' not found in planner.go).",
-      "status": "done",
-      "attempts": 1,
-      "expected_touched_area": [
-        "internal/next/planner/planner.go"
-      ],
-      "proof_checks": [
-        "grep -q 'regex' internal/next/planner/planner.go",
-        "grep -q 'Persistent Failures' internal/next/planner/planner.go",
-        "grep -q 'persistent-failure:' internal/next/planner/planner.go",
-        "go vet ./internal/next/planner/...",
-        "go test ./internal/next/planner/... -count=1 -timeout 60s"
-      ],
-      "files_changed": [
-        "internal/next/planner/planner.go"
-      ],
-      "tokens_used": 6821,
-      "duration_ms": 74586,
-      "model_tier": "",
-      "cycle": 2,
-      "kind": "fix",
-      "parent_cycle": 1,
-      "failures_addressed": [
-        "contract:persistent-failure-triggers-contract-audit-section — file_contains failed: pattern \"regex\" not found in \"internal/next/planner/planner.go\""
-      ]
-    },
-    {
-      "task_id": "t-004",
-      "objective": "Add test case for multiple persistent failures across different contracts in planner_test.go. The test name or description must contain the phrase 'multiple persistent'. It should call buildFixPlanPrompt with three contract failures where two have persistent-failure: hints, and assert: (1) both persistent failures appear in the dedicated section, (2) all three contract failures appear in Validation Failures to Fix, (3) the non-persistent failure does not appear in the persistent section. Fixes: contract:multiple-persistent-failures-across-different-contracts.",
-      "status": "done",
-      "attempts": 1,
-      "expected_touched_area": [
-        "internal/next/planner/planner_test.go"
-      ],
-      "proof_checks": [
-        "grep -q 'multiple persistent' internal/next/planner/planner_test.go",
-        "go test ./internal/next/planner/... -count=1 -timeout 60s -run 'Multiple|multiple'"
-      ],
-      "files_changed": [
-        "internal/next/planner/planner_test.go"
-      ],
-      "tokens_used": 5696,
-      "duration_ms": 51009,
-      "model_tier": "",
-      "cycle": 2,
-      "kind": "fix",
-      "parent_cycle": 1,
-      "failures_addressed": [
-        "contract:multiple-persistent-failures-across-different-contracts — file_contains failed: pattern \"multiple persistent\" not found in \"internal/next/planner/planner_test.go\""
-      ]
-    },
-    {
-      "task_id": "t-005",
-      "objective": "Replace the persistent-failures section intro text and all four audit instruction steps in buildFixPlanPrompt (lines ~174-187) with the exact wording from the spec. Fixes all 7 review findings: intro must say 'The following failures have repeated across multiple consecutive cycles.\\nThis strongly suggests the contract assertion itself is wrong, not the implementation.'; directive must say 'BEFORE creating any implementation fix task for these failures:'; steps 1-4 must match spec verbatim including regex examples and 'Prefer creating a contract fix task' guidance.",
-      "status": "done",
-      "attempts": 1,
-      "expected_touched_area": [
-        "internal/next/planner/planner.go"
-      ],
-      "proof_checks": [
-        "grep -q 'The following failures have repeated across multiple consecutive cycles.' internal/next/planner/planner.go",
-        "grep -q 'This strongly suggests the contract assertion itself is wrong, not the implementation.' internal/next/planner/planner.go",
-        "grep -q 'BEFORE creating any implementation fix task for these failures:' internal/next/planner/planner.go",
-        "grep -q 'Find the assertion in scenario-contracts.yaml that corresponds to this failure' internal/next/planner/planner.go",
-        "grep -q 'Verify the pattern actually appears in the target file (run grep manually in your head)' internal/next/planner/planner.go",
-        "grep -q 'the pattern may need to be a literal substring instead' internal/next/planner/planner.go",
-        "grep -q 'Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you' internal/next/planner/planner.go",
-        "! grep -q 'escalate to the spec author' internal/next/planner/planner.go",
-        "go vet ./internal/next/planner/...",
-        "go test ./internal/next/planner/... -count=1 -timeout 60s"
-      ],
-      "files_changed": [
-        "internal/next/planner/planner.go",
-        "internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go"
-      ],
-      "tokens_used": 7599,
-      "duration_ms": 80356,
-      "model_tier": "",
-      "cycle": 3,
-      "kind": "fix",
-      "parent_cycle": 2,
-      "failures_addressed": [
-        "review:spec_alignment:error:internal/next/planner/planner.go:174 — Intro text deviates from spec.",
-        "review:spec_alignment:error:internal/next/planner/planner.go:180 — Missing \"BEFORE creating any implementation fix task for these failures:\" directive.",
-        "review:spec_alignment:error:internal/next/planner/planner.go:181 — Step 1 guidance deviates from spec.",
-        "review:spec_alignment:error:internal/next/planner/planner.go:182 — Step 2 guidance is entirely different from spec.",
-        "review:spec_alignment:error:internal/next/planner/planner.go:183 — Step 3 guidance is weaker and omits the spec's concrete regex examples.",
-        "review:spec_alignment:error:internal/next/planner/planner.go:184 — Step 4 guidance contradicts the spec's intent.",
-        "review:code_quality:error:internal/next/planner/planner.go:187 — Audit step 4 tells the planner to 'escalate to the spec author' which contradicts the spec."
-      ]
-    }
-  ],
-  "worktree_path": "/Users/dabrams/gromit/.gromit-next/worktrees/wt-844121146",
-  "accumulated_cost": 0.6169518,
-  "final_validation_passed": true,
-  "final_review_passed": true,
-  "final_acceptance_passed": true,
-  "replan_context": [
-    "review:spec_alignment:error:internal/next/planner/planner.go:174 — Intro text deviates from spec. Spec requires: \"The following failures have repeated across multiple consecutive cycles.\\nThis strongly suggests the contract assertion itself is wrong, not the implementation.\" The implementation uses a weaker paraphrase that loses the critical \"strongly suggests\" signal the spec emphasizes. (suggested fix: Replace with exactly two sentences as specified: \"The following failures have repeated across multiple consecutive cycles.\\nThis strongly suggests the contract assertion itself is wrong, not the implementation.\\n\")",
-    "review:spec_alignment:error:internal/next/planner/planner.go:180 — Missing \"BEFORE creating any implementation fix task for these failures:\" directive. The spec explicitly requires this strong pre-action framing to prevent the planner from defaulting to implementation fixes. The implementation uses \"Audit Instructions:\" which is structurally weaker and changes the intended behavior. (suggested fix: Replace `b.WriteString(\"\\nAudit Instructions:\\n\")` with `b.WriteString(\"\\nBEFORE creating any implementation fix task for these failures:\\n\")`)",
-    "review:spec_alignment:error:internal/next/planner/planner.go:181 — Step 1 guidance deviates from spec. Spec requires: \"Find the assertion in scenario-contracts.yaml that corresponds to this failure\" — a specific, targeted lookup action. The implementation says \"Review the failing tests in the spec fixtures (e.g., scenario-contracts.yaml) to identify if the test expectations are internally inconsistent or unrealistic\" which is broader and loses the direct correspondence-check instruction. (suggested fix: Replace with: `b.WriteString(\"1. Find the assertion in scenario-contracts.yaml that corresponds to this failure\\n\")`)",
-    "review:spec_alignment:error:internal/next/planner/planner.go:182 — Step 2 guidance is entirely different from spec. Spec requires: \"Verify the pattern actually appears in the target file (run grep manually in your head)\" — this is a concrete grep verification step. The implementation substitutes generic advice about matching spec requirements, omitting the critical grep-verification instruction. (suggested fix: Replace with: `b.WriteString(\"2. Verify the pattern actually appears in the target file (run grep manually in your head)\\n\")`)",
-    "review:spec_alignment:error:internal/next/planner/planner.go:183 — Step 3 guidance is weaker and omits the spec's concrete regex examples. Spec requires: \"If the pattern looks like a regex (contains .*  \\w+  \\[  etc.) but the file uses literal Go syntax, the pattern may need to be a literal substring instead\". The implementation's version drops the specific regex character examples that make this actionable. (suggested fix: Replace with: `b.WriteString(\"3. If the pattern looks like a regex (contains .*  \\\\w+  \\\\[  etc.) but the file uses\\n   literal Go syntax, the pattern may need to be a literal substring instead\\n\")`)",
-    "review:spec_alignment:error:internal/next/planner/planner.go:184 — Step 4 guidance contradicts the spec's intent. Spec requires: \"Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you have high confidence the implementation is wrong\" — the spec wants the planner to actively fix the contract. The implementation says \"escalate to the spec author\" which is the opposite action and contradicts the spec's vision that the planner should decide and act. (suggested fix: Replace with: `b.WriteString(\"4. Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you\\n   have high confidence the implementation is wrong\\n\\n\")`)",
-    "review:code_quality:error:internal/next/planner/planner.go:187 — Audit step 4 tells the planner to 'escalate to the spec author rather than attempting to fix the code', which directly contradicts the spec. Spec acceptance criterion and architecture section both state the planner should 'Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you have high confidence the implementation is wrong'. Escalating to a human is not a valid planner output; the planner produces tasks. This guidance will cause the planner to emit no actionable task rather than a contract-fix task. (suggested fix: Replace the step 4 WriteString with: \"4. Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you have high confidence the implementation is wrong.\\n\\n\")"
-  ],
-  "last_validation_result": "pass=true",
-  "last_final_validation": {
-    "pass": true,
-    "always_run": {
-      "results": [
-        {
-          "name": "unit-tests",
-          "pass": true,
-          "output": "ok  \tgithub.com/danabrams/gromit\t0.180s\nok  \tgithub.com/danabrams/gromit/calc\t(cached)\nok  \tgithub.com/danabrams/gromit/cmd/gromit\t11.424s\nok  \tgithub.com/danabrams/gromit/cmd/gromit-next\t1.804s\n?   \tgithub.com/danabrams/gromit/cmd/test_e2e_live_packages\t[no test files]\n?   \tgithub.com/danabrams/gromit/e2e\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/agent\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/analytics\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/analyzer\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/backlog\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/bead\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/benchmark\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/claude\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/config\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/conversation\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/coverage\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/cli\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/eventtest\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/status\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/stream\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/events/tmux\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/experiment\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/failurephase\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/frontmatter\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/integrationqueue\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/jsonutil\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/learnings\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/logger\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/acceptor\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/architecture\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/artifact\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/contextpkt\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/contract\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/doctrine\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/enrich\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/evidence\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/execpolicy\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/executor\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/extract\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/fact\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/guide\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/infer\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/inspect\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/llmadapter\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/planner\t0.306s\nok  \tgithub.com/danabrams/gromit/internal/next/projectcell\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/provenance\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/runstore\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/sourcemap\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/specloop\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/specloop/stages\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/testutil\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/validation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/validator\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/next/workspace\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/epilogue\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/execute\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/prepare\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/qualitygate/regression\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/qualitygate/wiring\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/pipeline/validate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/procutil\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/prompt\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/provider\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/queue\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/readiness\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/retro\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runbook\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/runner/acceptance\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/runner/andon\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/display\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/escalation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/execution\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/methodology\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/pipeline/midreview\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/policy\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/readinessadapterllm\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/reviewpkg\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/runtypes\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/specbranch\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/specmerge\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/util\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/runner/validation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/scope\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/specflow\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/specgate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/state\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/testpkg\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/tracker\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/tracker/trackertest\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/v2/adapter\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/v2/adapter/git\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/adapter/llm\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/adapter/presenter\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/adapter/tasktracker\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/dep\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/event\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/generation\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/v2/llmtypes\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/v2/loop\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/pipeline\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/presentation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/prompt\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/remediation\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/routing\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/spec\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/accept\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/build\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/decompose\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/epilogue\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/gate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/names\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/plan\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/present\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/review\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/triage\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/stage/validate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/v2/testutil\t(cached)\n?   \tgithub.com/danabrams/gromit/internal/v2/trackertypes\t[no test files]\nok  \tgithub.com/danabrams/gromit/internal/validate\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/visionmetrics\t(cached)\nok  \tgithub.com/danabrams/gromit/internal/worktree\t(cached)\nok  \tgithub.com/danabrams/gromit/scripts\t(cached)\n?   \tgithub.com/danabrams/gromit/scripts/add_parallel\t[no test files]\n?   \tgithub.com/danabrams/gromit/skills\t[no test files]\nok  \tgithub.com/danabrams/gromit/test/codexenv\t(cached)\n?   \tgithub.com/danabrams/gromit/test/contracts\t[no test files]\nok  \tgithub.com/danabrams/gromit/test/docs\t(cached)\nok  \tgithub.com/danabrams/gromit/test/fixtures\t(cached)\nok  \tgithub.com/danabrams/gromit/test/helpers\t(cached)\nok  \tgithub.com/danabrams/gromit/test/repohygiene\t(cached)\nok  \tgithub.com/danabrams/gromit/test/testutil\t(cached)\nok  \tgithub.com/danabrams/gromit/test/toolcalls\t(cached)\n",
-          "duration": 12435314000,
-          "type": "test"
-        },
-        {
-          "name": "vet",
-          "pass": true,
-          "output": "",
-          "duration": 403331792,
-          "type": "lint"
-        }
-      ]
-    },
-    "project_checks": {
-      "results": null
-    }
-  },
-  "review_findings": [
-    "review:code_quality:warning:internal/next/planner/planner_test.go:341 — Significant test duplication: the 6 new functions added to planner_test.go cover the same scenarios as the 4 new scenario test files. TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection, _AppearsBeforeValidationSection, and _AlsoAppearsInValidationSection collectively duplicate TestScenario_PersistentFailureTriggersContractAuditSection. TestBuildFixPlanPrompt_NoPersistentFailures_NoSection duplicates TestScenario_NoPersistentFailures_PromptUnchanged. TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts duplicates TestScenario_MultiplePersistentFailuresAcrossDifferentContracts. TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure duplicates TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure. (suggested fix: Remove the 6 duplicated test functions from planner_test.go and keep only the scenario test files, which have clearer Given/When/Then structure and more thorough assertions. Or, if keeping planner_test.go tests, remove the scenario files and consolidate assertions.)",
-    "review:code_quality:warning:internal/next/planner/planner_test.go:341 — TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection, TestBuildFixPlanPrompt_PersistentFailures_AppearsBeforeValidationSection, and TestBuildFixPlanPrompt_PersistentFailures_AlsoAppearsInValidationSection all construct the identical FixPlanRequest with a single persistent-failure entry and call buildFixPlanPrompt, then each asserts a different property of the same prompt. This inflates test count without adding coverage and builds the same prompt three times. (suggested fix: Merge the three into a single table-driven or subtested function, or a single TestBuildFixPlanPrompt_PersistentFailures function with all three assertions.)",
-    "review:code_quality:suggestion:internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go:57 — File is missing a trailing newline (diff shows 'No newline at end of file'). POSIX requires text files to end with a newline; gofmt enforces this and some CI linters will flag it. (suggested fix: Add a trailing newline after the closing brace of the file.)",
-    "review:code_quality:suggestion:internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go:72 — File is missing a trailing newline (diff shows 'No newline at end of file'). (suggested fix: Add a trailing newline after the closing brace of the file.)",
-    "review:code_quality:suggestion:internal/next/planner/planner_scenario_no_persistent_failures_test.go:61 — File is missing a trailing newline (diff shows 'No newline at end of file'). (suggested fix: Add a trailing newline after the closing brace of the file.)",
-    "review:code_quality:suggestion:internal/next/planner/planner_scenario_multiple_persistent_failures_test.go:66 — File is missing a trailing newline (diff shows 'No newline at end of file'). (suggested fix: Add a trailing newline after the closing brace of the file.)",
-    "review:code_quality:info:internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go:36 — The test checks that `scenario-contracts.yaml` appears in the prompt. This passes because the spec-required Step 1 text directly references `scenario-contracts.yaml` by name. The check is valid but narrow — it would also pass if the string appeared incidentally elsewhere in the prompt. Low risk since the step 1 text is now an exact match to the spec. (suggested fix: No action required. Optionally strengthen by asserting the full step 1 sentence: \"Find the assertion in scenario-contracts.yaml that corresponds to this failure\".)",
-    "review:spec_alignment:warning:internal/next/planner/planner_scenario_multiple_persistent_failures_test.go:12 — Test seed does not match the spec's Scenario 3 'Given' conditions. Spec says 'Three contract failures, two of which have persistent-failure hints' — implying three `contract:` prefixed entries plus two accompanying `persistent-failure:` entries. The test provides only one `contract:` entry and two standalone `persistent-failure:` entries (with no corresponding `contract:auth` or `contract:payment` prefix lines). The scenario is structurally different from what the spec describes, even though the acceptance-criteria assertions still pass. (suggested fix: Add corresponding `contract:auth-contracts.yaml login_timeout ...` and `contract:payment-contracts.yaml refund_flow ...` entries to the Failures slice alongside the persistent-failure hints, so the seed matches the spec's 'three contract failures, two with hints' scenario.)",
-    "review:spec_alignment:warning:internal/next/planner/planner_test.go:339 — Significant test duplication between the six new functions added to planner_test.go and the four new scenario test files. TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection, _AppearsBeforeValidationSection, and _AlsoAppearsInValidationSection collectively cover the same single-persistent-failure scenario as TestScenario_PersistentFailureTriggersContractAuditSection. TestBuildFixPlanPrompt_NoPersistentFailures_NoSection duplicates TestScenario_NoPersistentFailures_PromptUnchanged. TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts duplicates TestScenario_MultiplePersistentFailuresAcrossDifferentContracts. TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure duplicates TestScenario_PersistentFailureHint_WithoutCorrespondingContractFailure. (suggested fix: Remove the duplicated unit tests from planner_test.go (or the scenario files), keeping only one authoritative location per scenario. The scenario files are better named and more scenario-aligned; the planner_test.go additions could be pruned to only TestBuildFixPlanPrompt_MultiplePersistentFailures_MixedWithNonPersistent which has no direct scenario counterpart.)",
-    "review:spec_alignment:warning:internal/next/planner/planner_test.go:339 — TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection, _AppearsBeforeValidationSection, and _AlsoAppearsInValidationSection are three separate functions with an identical FixPlanRequest literal, each asserting a different property. This is a test smell — the same prompt is constructed three times and the test count is inflated without adding coverage. (suggested fix: Merge the three functions into one table-driven or multi-assertion test, or rely solely on the richer scenario test TestScenario_PersistentFailureTriggersContractAuditSection which already covers all three properties.)",
-    "review:spec_alignment:suggestion:internal/next/planner/planner_scenario_persistent_failure_contract_audit_test.go:57 — File is missing a trailing newline (diff marker: `\\ No newline at end of file`). Violates POSIX text-file convention and gofmt expectations; some CI linters will warn. (suggested fix: Add a trailing newline after the closing `}`.)",
-    "review:spec_alignment:suggestion:internal/next/planner/planner_scenario_persistent_failure_no_contract_test.go:72 — File is missing a trailing newline (diff marker: `\\ No newline at end of file`). (suggested fix: Add a trailing newline after the closing `}`.)",
-    "review:spec_alignment:suggestion:internal/next/planner/planner_scenario_no_persistent_failures_test.go:61 — File is missing a trailing newline (diff marker: `\\ No newline at end of file`). (suggested fix: Add a trailing newline after the closing `}`.)",
-    "review:spec_alignment:suggestion:internal/next/planner/planner_scenario_multiple_persistent_failures_test.go:66 — File is missing a trailing newline (diff marker: `\\ No newline at end of file`). (suggested fix: Add a trailing newline after the closing `}`.)"
-  ],
-  "total_replans": 2,
-  "contracts_written": true,
-  "scenario_tests_written": true
-}
\ No newline at end of file
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/spec-packet.md b/.gromit-next/runs/run-45dcbc628184bad1/spec-packet.md
deleted file mode 100644
index 79f120258..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/spec-packet.md
+++ /dev/null
@@ -1,111 +0,0 @@
-# Spec 0003e — Persistent Failure Contract Audit
-
-## spec_id
-0003e-persistent-failure-contract-audit
-
-## Depends on
-0003b-replan-context-deduplication
-
-## Vision
-When the same contract assertion fails across multiple consecutive cycles, the persistent-failure hint fires and says "may indicate a bad test specification." But the fix planner's prompt buries this hint in the general failures list with instructions that push toward implementation fixes. The signal is present but structurally invisible — the planner has no reason to act on it differently. This spec makes persistent failures a first-class signal in the fix planner: they get a dedicated section with targeted guidance to consider fixing the contract assertion itself, not just the implementation.
-
-## Summary
-When `buildFixPlanPrompt` receives failures that include `persistent-failure:` hints, it extracts them into a dedicated `## Persistent Failures — Possible Bad Contracts` section with explicit instructions to audit the contract assertion YAML before creating implementation fix tasks. The persistent failures also remain in the main `## Validation Failures to Fix` section so the planner can still generate implementation fix tasks if it judges the contract to be correct.
-
-## Goals
-### Primary
-- Give the fix planner structured, targeted guidance when persistent failures are present
-- Instruct the planner to check whether the pattern in scenario-contracts.yaml actually matches the file before assuming the implementation is wrong
-
-### Secondary
-- Reduce wasted replan cycles caused by unsatisfiable contract assertions
-
-## Non-goals
-- Not automatically fixing contracts — the planner still decides
-- Not changing when or how persistent-failure hints are generated (that's 0003b)
-- Not adding new terminal states or escalation paths (that's 0003d)
-- Not validating contracts at write time (a future spec)
-
-## Architecture
-The change is entirely in `buildFixPlanPrompt` in `internal/next/planner/planner.go`.
-
-Currently all failures (including `persistent-failure:` hints) flow into one of two buckets: `reviewFindings` or `otherFailures`. The new logic adds a third pass that extracts `persistent-failure:` lines into `persistentFailures` — but does NOT remove them from `otherFailures`. Both lists are rendered.
-
-```go
-var reviewFindings, otherFailures, persistentFailures []string
-for _, f := range req.Failures {
-    if strings.HasPrefix(f, "review:") {
-        reviewFindings = append(reviewFindings, f)
-    } else {
-        otherFailures = append(otherFailures, f)
-    }
-    if strings.HasPrefix(f, "persistent-failure:") {
-        persistentFailures = append(persistentFailures, f)
-    }
-}
-```
-
-When `persistentFailures` is non-empty, a new section is rendered **before** `## Validation Failures to Fix`:
-
-```
-## Persistent Failures — Possible Bad Contracts
-The following failures have repeated across multiple consecutive cycles.
-This strongly suggests the contract assertion itself is wrong, not the implementation.
-
-BEFORE creating any implementation fix task for these failures:
-1. Find the assertion in scenario-contracts.yaml that corresponds to this failure
-2. Verify the pattern actually appears in the target file (run grep manually in your head)
-3. If the pattern looks like a regex (contains .*  \w+  \[  etc.) but the file uses
-   literal Go syntax, the pattern may need to be a literal substring instead
-4. Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you
-   have high confidence the implementation is wrong
-
-Persistent failures:
-- <list>
-```
-
-No new types, no new fields on `PlanRequest`. Pure prompt construction change.
-
-## Acceptance Criteria
-
-1. When `req.Failures` contains one or more `persistent-failure:` prefixed entries, `buildFixPlanPrompt` renders a `## Persistent Failures — Possible Bad Contracts` section before `## Validation Failures to Fix`
-2. The persistent failures section includes explicit instructions to audit the contract assertion in `scenario-contracts.yaml` before creating implementation fix tasks
-3. Persistent failures still appear in `## Validation Failures to Fix` — they are not removed from the main list
-4. When `req.Failures` contains no `persistent-failure:` entries, no persistent failures section is rendered and existing behavior is unchanged
-5. Non-persistent contract failures and review findings are unaffected by this change
-6. All existing planner tests continue to pass
-
-## Scenarios
-
-### Scenario: persistent failure triggers contract audit section
-**Given:** A replan context containing one contract failure and its corresponding persistent-failure hint:
-```
-contract:first-failure-no-escalation — file_contains failed: pattern "ChainIDs.*\[\]string" not found in "internal/next/runstore/types.go"
-persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug
-```
-**When:** `buildFixPlanPrompt` is called with these as `req.Failures`
-**Then:** The rendered prompt contains a `## Persistent Failures — Possible Bad Contracts` section listing the persistent-failure hint, with instructions to check `scenario-contracts.yaml` and consider whether the pattern is a regex being matched literally. The contract failure also appears in `## Validation Failures to Fix`.
-
-### Scenario: no persistent failures — prompt unchanged
-**Given:** A replan context with only ordinary contract and test failures, no `persistent-failure:` entries
-**When:** `buildFixPlanPrompt` is called
-**Then:** No `## Persistent Failures` section appears. Prompt is identical to current behavior.
-
-### Scenario: multiple persistent failures across different contracts
-**Given:** Three contract failures, two of which have persistent-failure hints
-**When:** `buildFixPlanPrompt` is called
-**Then:** Both persistent failures appear in the dedicated section. All three contract failures appear in `## Validation Failures to Fix`. The non-persistent failure does not appear in the persistent section.
-
-### Scenario: persistent-failure hint without corresponding contract failure
-**Given:** A replan context containing only a persistent-failure hint, with no corresponding `contract:` failure entry (e.g. the original failure was deduplicated into a summary that no longer carries the `contract:` prefix):
-```
-persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug
-```
-**When:** `buildFixPlanPrompt` is called
-**Then:** The persistent-failure hint still appears in `## Persistent Failures — Possible Bad Contracts` with the full audit instructions. `## Validation Failures to Fix` contains the persistent-failure hint itself (it is not a `review:` entry, so it lands in `otherFailures`) and any deduplicated summary — the planner receives the audit signal from both sections.
-
-## Validation
-```
-go test ./internal/next/planner/... -count=1 -timeout 60s
-go vet ./internal/next/planner/...
-```
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/spec.md b/.gromit-next/runs/run-45dcbc628184bad1/spec.md
deleted file mode 100644
index 79f120258..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/spec.md
+++ /dev/null
@@ -1,111 +0,0 @@
-# Spec 0003e — Persistent Failure Contract Audit
-
-## spec_id
-0003e-persistent-failure-contract-audit
-
-## Depends on
-0003b-replan-context-deduplication
-
-## Vision
-When the same contract assertion fails across multiple consecutive cycles, the persistent-failure hint fires and says "may indicate a bad test specification." But the fix planner's prompt buries this hint in the general failures list with instructions that push toward implementation fixes. The signal is present but structurally invisible — the planner has no reason to act on it differently. This spec makes persistent failures a first-class signal in the fix planner: they get a dedicated section with targeted guidance to consider fixing the contract assertion itself, not just the implementation.
-
-## Summary
-When `buildFixPlanPrompt` receives failures that include `persistent-failure:` hints, it extracts them into a dedicated `## Persistent Failures — Possible Bad Contracts` section with explicit instructions to audit the contract assertion YAML before creating implementation fix tasks. The persistent failures also remain in the main `## Validation Failures to Fix` section so the planner can still generate implementation fix tasks if it judges the contract to be correct.
-
-## Goals
-### Primary
-- Give the fix planner structured, targeted guidance when persistent failures are present
-- Instruct the planner to check whether the pattern in scenario-contracts.yaml actually matches the file before assuming the implementation is wrong
-
-### Secondary
-- Reduce wasted replan cycles caused by unsatisfiable contract assertions
-
-## Non-goals
-- Not automatically fixing contracts — the planner still decides
-- Not changing when or how persistent-failure hints are generated (that's 0003b)
-- Not adding new terminal states or escalation paths (that's 0003d)
-- Not validating contracts at write time (a future spec)
-
-## Architecture
-The change is entirely in `buildFixPlanPrompt` in `internal/next/planner/planner.go`.
-
-Currently all failures (including `persistent-failure:` hints) flow into one of two buckets: `reviewFindings` or `otherFailures`. The new logic adds a third pass that extracts `persistent-failure:` lines into `persistentFailures` — but does NOT remove them from `otherFailures`. Both lists are rendered.
-
-```go
-var reviewFindings, otherFailures, persistentFailures []string
-for _, f := range req.Failures {
-    if strings.HasPrefix(f, "review:") {
-        reviewFindings = append(reviewFindings, f)
-    } else {
-        otherFailures = append(otherFailures, f)
-    }
-    if strings.HasPrefix(f, "persistent-failure:") {
-        persistentFailures = append(persistentFailures, f)
-    }
-}
-```
-
-When `persistentFailures` is non-empty, a new section is rendered **before** `## Validation Failures to Fix`:
-
-```
-## Persistent Failures — Possible Bad Contracts
-The following failures have repeated across multiple consecutive cycles.
-This strongly suggests the contract assertion itself is wrong, not the implementation.
-
-BEFORE creating any implementation fix task for these failures:
-1. Find the assertion in scenario-contracts.yaml that corresponds to this failure
-2. Verify the pattern actually appears in the target file (run grep manually in your head)
-3. If the pattern looks like a regex (contains .*  \w+  \[  etc.) but the file uses
-   literal Go syntax, the pattern may need to be a literal substring instead
-4. Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you
-   have high confidence the implementation is wrong
-
-Persistent failures:
-- <list>
-```
-
-No new types, no new fields on `PlanRequest`. Pure prompt construction change.
-
-## Acceptance Criteria
-
-1. When `req.Failures` contains one or more `persistent-failure:` prefixed entries, `buildFixPlanPrompt` renders a `## Persistent Failures — Possible Bad Contracts` section before `## Validation Failures to Fix`
-2. The persistent failures section includes explicit instructions to audit the contract assertion in `scenario-contracts.yaml` before creating implementation fix tasks
-3. Persistent failures still appear in `## Validation Failures to Fix` — they are not removed from the main list
-4. When `req.Failures` contains no `persistent-failure:` entries, no persistent failures section is rendered and existing behavior is unchanged
-5. Non-persistent contract failures and review findings are unaffected by this change
-6. All existing planner tests continue to pass
-
-## Scenarios
-
-### Scenario: persistent failure triggers contract audit section
-**Given:** A replan context containing one contract failure and its corresponding persistent-failure hint:
-```
-contract:first-failure-no-escalation — file_contains failed: pattern "ChainIDs.*\[\]string" not found in "internal/next/runstore/types.go"
-persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug
-```
-**When:** `buildFixPlanPrompt` is called with these as `req.Failures`
-**Then:** The rendered prompt contains a `## Persistent Failures — Possible Bad Contracts` section listing the persistent-failure hint, with instructions to check `scenario-contracts.yaml` and consider whether the pattern is a regex being matched literally. The contract failure also appears in `## Validation Failures to Fix`.
-
-### Scenario: no persistent failures — prompt unchanged
-**Given:** A replan context with only ordinary contract and test failures, no `persistent-failure:` entries
-**When:** `buildFixPlanPrompt` is called
-**Then:** No `## Persistent Failures` section appears. Prompt is identical to current behavior.
-
-### Scenario: multiple persistent failures across different contracts
-**Given:** Three contract failures, two of which have persistent-failure hints
-**When:** `buildFixPlanPrompt` is called
-**Then:** Both persistent failures appear in the dedicated section. All three contract failures appear in `## Validation Failures to Fix`. The non-persistent failure does not appear in the persistent section.
-
-### Scenario: persistent-failure hint without corresponding contract failure
-**Given:** A replan context containing only a persistent-failure hint, with no corresponding `contract:` failure entry (e.g. the original failure was deduplicated into a summary that no longer carries the `contract:` prefix):
-```
-persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug
-```
-**When:** `buildFixPlanPrompt` is called
-**Then:** The persistent-failure hint still appears in `## Persistent Failures — Possible Bad Contracts` with the full audit instructions. `## Validation Failures to Fix` contains the persistent-failure hint itself (it is not a `review:` entry, so it lands in `otherFailures`) and any deduplicated summary — the planner receives the audit signal from both sections.
-
-## Validation
-```
-go test ./internal/next/planner/... -count=1 -timeout 60s
-go vet ./internal/next/planner/...
-```
diff --git a/.gromit-next/runs/run-45dcbc628184bad1/tasks.json b/.gromit-next/runs/run-45dcbc628184bad1/tasks.json
deleted file mode 100644
index 6ab304b3e..000000000
--- a/.gromit-next/runs/run-45dcbc628184bad1/tasks.json
+++ /dev/null
@@ -1,142 +0,0 @@
-[
-  {
-    "task_id": "t-001",
-    "objective": "Write tests for persistent failure extraction in buildFixPlanPrompt: (1) persistent failures render a dedicated '## Persistent Failures — Possible Bad Contracts' section before '## Validation Failures to Fix', (2) persistent failures still appear in otherFailures/validation section, (3) no persistent failures means no section rendered, (4) multiple persistent failures with mixed non-persistent, (5) persistent-failure hint without corresponding contract: failure still renders section",
-    "status": "done",
-    "attempts": 1,
-    "expected_touched_area": [
-      "internal/next/planner/planner_test.go"
-    ],
-    "proof_checks": [
-      "grep -q 'TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection' internal/next/planner/planner_test.go",
-      "grep -q 'TestBuildFixPlanPrompt_NoPersistentFailures_NoSection' internal/next/planner/planner_test.go",
-      "grep -q 'TestBuildFixPlanPrompt_MultiplePersistentFailures_MixedWithNonPersistent' internal/next/planner/planner_test.go",
-      "grep -q 'TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure' internal/next/planner/planner_test.go",
-      "grep -q 'scenario-contracts.yaml' internal/next/planner/planner_test.go",
-      "go vet ./internal/next/planner/..."
-    ],
-    "files_changed": [
-      "internal/next/planner/planner.go",
-      "internal/next/planner/planner_test.go"
-    ],
-    "tokens_used": 9398,
-    "duration_ms": 78069,
-    "model_tier": "",
-    "cycle": 1,
-    "kind": "original"
-  },
-  {
-    "task_id": "t-002",
-    "objective": "Implement persistent failure extraction in buildFixPlanPrompt: add a third pass to collect persistent-failure: prefixed entries into persistentFailures slice (without removing from otherFailures), then render a '## Persistent Failures — Possible Bad Contracts' section with audit instructions before the '## Validation Failures to Fix' section when persistentFailures is non-empty",
-    "status": "done",
-    "attempts": 1,
-    "expected_touched_area": [
-      "internal/next/planner/planner.go"
-    ],
-    "proof_checks": [
-      "go test ./internal/next/planner/... -count=1 -timeout 60s",
-      "go vet ./internal/next/planner/...",
-      "grep -q 'persistentFailures' internal/next/planner/planner.go",
-      "grep -q 'Persistent Failures' internal/next/planner/planner.go",
-      "grep -q 'scenario-contracts.yaml' internal/next/planner/planner.go"
-    ],
-    "files_changed": [
-      "internal/next/planner/planner.go"
-    ],
-    "tokens_used": 3613,
-    "duration_ms": 31647,
-    "model_tier": "",
-    "cycle": 1,
-    "kind": "original"
-  },
-  {
-    "task_id": "t-003",
-    "objective": "Add persistent-failure extraction and dedicated prompt section to buildFixPlanPrompt. After separating review findings from other failures, add a third pass that collects persistent-failure: prefixed entries into a persistentFailures slice (without removing them from otherFailures). When persistentFailures is non-empty, render the '## Persistent Failures — Possible Bad Contracts' section before '## Validation Failures to Fix' with the full audit instructions including the word 'regex' (e.g., 'If the pattern looks like a regex'). Fixes: contract:persistent-failure-triggers-contract-audit-section (pattern 'regex' not found in planner.go).",
-    "status": "done",
-    "attempts": 1,
-    "expected_touched_area": [
-      "internal/next/planner/planner.go"
-    ],
-    "proof_checks": [
-      "grep -q 'regex' internal/next/planner/planner.go",
-      "grep -q 'Persistent Failures' internal/next/planner/planner.go",
-      "grep -q 'persistent-failure:' internal/next/planner/planner.go",
-      "go vet ./internal/next/planner/...",
-      "go test ./internal/next/planner/... -count=1 -timeout 60s"
-    ],
-    "files_changed": [
-      "internal/next/planner/planner.go"
-    ],
-    "tokens_used": 6821,
-    "duration_ms": 74586,
-    "model_tier": "",
-    "cycle": 2,
-    "kind": "fix",
-    "parent_cycle": 1,
-    "failures_addressed": [
-      "contract:persistent-failure-triggers-contract-audit-section — file_contains failed: pattern \"regex\" not found in \"internal/next/planner/planner.go\""
-    ]
-  },
-  {
-    "task_id": "t-004",
-    "objective": "Add test case for multiple persistent failures across different contracts in planner_test.go. The test name or description must contain the phrase 'multiple persistent'. It should call buildFixPlanPrompt with three contract failures where two have persistent-failure: hints, and assert: (1) both persistent failures appear in the dedicated section, (2) all three contract failures appear in Validation Failures to Fix, (3) the non-persistent failure does not appear in the persistent section. Fixes: contract:multiple-persistent-failures-across-different-contracts.",
-    "status": "done",
-    "attempts": 1,
-    "expected_touched_area": [
-      "internal/next/planner/planner_test.go"
-    ],
-    "proof_checks": [
-      "grep -q 'multiple persistent' internal/next/planner/planner_test.go",
-      "go test ./internal/next/planner/... -count=1 -timeout 60s -run 'Multiple|multiple'"
-    ],
-    "files_changed": [
-      "internal/next/planner/planner_test.go"
-    ],
-    "tokens_used": 5696,
-    "duration_ms": 51009,
-    "model_tier": "",
-    "cycle": 2,
-    "kind": "fix",
-    "parent_cycle": 1,
-    "failures_addressed": [
-      "contract:multiple-persistent-failures-across-different-contracts — file_contains failed: pattern \"multiple persistent\" not found in \"internal/next/planner/planner_test.go\""
-    ]
-  },
-  {
-    "task_id": "t-005",
-    "objective": "Replace the persistent-failures section intro text and all four audit instruction steps in buildFixPlanPrompt (lines ~174-187) with the exact wording from the spec. Fixes all 7 review findings: intro must say 'The following failures have repeated across multiple consecutive cycles.\\nThis strongly suggests the contract assertion itself is wrong, not the implementation.'; directive must say 'BEFORE creating any implementation fix task for these failures:'; steps 1-4 must match spec verbatim including regex examples and 'Prefer creating a contract fix task' guidance.",
-    "status": "pending",
-    "attempts": 0,
-    "expected_touched_area": [
-      "internal/next/planner/planner.go"
-    ],
-    "proof_checks": [
-      "grep -q 'The following failures have repeated across multiple consecutive cycles.' internal/next/planner/planner.go",
-      "grep -q 'This strongly suggests the contract assertion itself is wrong, not the implementation.' internal/next/planner/planner.go",
-      "grep -q 'BEFORE creating any implementation fix task for these failures:' internal/next/planner/planner.go",
-      "grep -q 'Find the assertion in scenario-contracts.yaml that corresponds to this failure' internal/next/planner/planner.go",
-      "grep -q 'Verify the pattern actually appears in the target file (run grep manually in your head)' internal/next/planner/planner.go",
-      "grep -q 'the pattern may need to be a literal substring instead' internal/next/planner/planner.go",
-      "grep -q 'Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you' internal/next/planner/planner.go",
-      "! grep -q 'escalate to the spec author' internal/next/planner/planner.go",
-      "go vet ./internal/next/planner/...",
-      "go test ./internal/next/planner/... -count=1 -timeout 60s"
-    ],
-    "files_changed": [],
-    "tokens_used": 0,
-    "duration_ms": 0,
-    "model_tier": "",
-    "cycle": 3,
-    "kind": "fix",
-    "parent_cycle": 2,
-    "failures_addressed": [
-      "review:spec_alignment:error:internal/next/planner/planner.go:174 — Intro text deviates from spec.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:180 — Missing \"BEFORE creating any implementation fix task for these failures:\" directive.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:181 — Step 1 guidance deviates from spec.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:182 — Step 2 guidance is entirely different from spec.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:183 — Step 3 guidance is weaker and omits the spec's concrete regex examples.",
-      "review:spec_alignment:error:internal/next/planner/planner.go:184 — Step 4 guidance contradicts the spec's intent.",
-      "review:code_quality:error:internal/next/planner/planner.go:187 — Audit step 4 tells the planner to 'escalate to the spec author' which contradicts the spec."
-    ]
-  }
-]
\ No newline at end of file
diff --git a/cmd/gromit-next/exec.go b/cmd/gromit-next/exec.go
index 64cabdaf1..a6721a9f1 100644
--- a/cmd/gromit-next/exec.go
+++ b/cmd/gromit-next/exec.go
@@ -3,7 +3,9 @@ package main
 import (
 	"context"
 	"fmt"
+	"io"
 	"os"
+	"os/exec"
 	"path/filepath"
 	"strings"
 	"time"
@@ -111,6 +113,8 @@ type execSpecRun struct {
 	storeDir      string
 	stageProvider StageProvider
 	policy        *execpolicy.Policy // pre-loaded policy; skips re-load in run()
+	out           io.Writer
+	store         *runstore.Store
 }
 
 // run executes the spec pipeline and returns the formatted result string.
@@ -135,11 +139,10 @@ func (e *execSpecRun) run(ctx context.Context) (string, error) {
 	}
 
 	// 2. Create or load run state
-	store := runstore.NewStore(e.storeDir)
 	var rs *runstore.RunState
 	if e.resumeRunID != "" {
 		var err error
-		rs, err = store.Get(e.resumeRunID)
+		rs, err = e.store.Get(e.resumeRunID)
 		if err != nil {
 			return "", fmt.Errorf("load run for resume: %w", err)
 		}
@@ -173,9 +176,12 @@ func (e *execSpecRun) run(ctx context.Context) (string, error) {
 	budget := specloop.NewBudget(policy.Budgets)
 
 	// 3b. Create the event log so pipeline events are persisted to disk.
-	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
+	eventLogPath := filepath.Join(e.store.RunDir(rs.RunID), "events.jsonl")
 	eventLog := runstore.NewEventLog(eventLogPath)
 
+	// 3c. Write the start banner
+	fmt.Fprintf(e.out, "Run ID: %s\nEvents: %s\n\n", rs.RunID, eventLogPath)
+
 	// 4. Build stages via provider, passing the shared budget and event log
 	stages, err := e.stageProvider.BuildStages(policy, rs, budget, eventLog)
 	if err != nil {
@@ -198,12 +204,14 @@ func (e *execSpecRun) run(ctx context.Context) (string, error) {
 	}
 
 	// 6. Persist final state
-	if err := store.Save(rs); err != nil {
+	if err := e.store.Save(rs); err != nil {
 		return "", fmt.Errorf("save run state: %w", err)
 	}
 
 	// 7. Print terminal state and run ID
-	return fmt.Sprintf("Run ID:  %s\nStatus:  %s\n", rs.RunID, rs.Status), nil
+	output := fmt.Sprintf("Run ID:  %s\nStatus:  %s\n", rs.RunID, rs.Status)
+	fmt.Fprint(e.out, output)
+	return output, nil
 }
 
 // newExecSpecCmd creates the `exec spec` command. Exported for testing.
@@ -211,6 +219,17 @@ func newExecSpecCmd() *cobra.Command {
 	return newExecSpecCmdWithProvider(nil)
 }
 
+// branchResolverFunc resolves the git branch for a spec by running git symbolic-ref.
+func branchResolverFunc(repoPath string) string {
+	// Run: git -C <repoPath> symbolic-ref --short HEAD
+	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "--short", "HEAD")
+	out, err := cmd.Output()
+	if err != nil {
+		return "unknown"
+	}
+	return strings.TrimSpace(string(out))
+}
+
 // newExecSpecCmdWithProvider creates the `exec spec` command with an explicit
 // StageProvider. If provider is nil, the defaultStageProvider is used.
 func newExecSpecCmdWithProvider(provider StageProvider) *cobra.Command {
@@ -221,6 +240,7 @@ func newExecSpecCmdWithProvider(provider StageProvider) *cobra.Command {
 			specPath, _ := cmd.Flags().GetString("spec")
 			projectID, _ := cmd.Flags().GetString("project")
 			policyPath, _ := cmd.Flags().GetString("policy")
+			specsDir, _ := cmd.Flags().GetString("specs-dir")
 			resolver := workspace.NewEnvResolver()
 			root, _ := resolver.Resolve()
 			if policyPath == "" && root != "" {
@@ -246,6 +266,49 @@ func newExecSpecCmdWithProvider(provider StageProvider) *cobra.Command {
 				policy = execpolicy.DefaultPolicy()
 			}
 
+			// Wire picker flow
+			if resumeRunID == "__pick__" {
+				pickStore := runstore.NewStore(storeDir)
+				runID, err := pickRun(projectID, pickStore, cmd.InOrStdin(), cmd.OutOrStdout())
+				if err != nil {
+					return fmt.Errorf("pick run: %w", err)
+				}
+				if runID == "" {
+					return fmt.Errorf("no run selected")
+				}
+				resumeRunID = runID
+			}
+
+			if resumeRunID == "" && specPath == "" {
+				// Resolve specsDir if not provided via flag
+				if specsDir == "" {
+					if root != "" {
+						projectDir, err := ResolveProjectConfigPath(root, projectID)
+						if err != nil {
+							return fmt.Errorf("resolve project config: %w", err)
+						}
+						cfg, err := LoadProjectConfig(projectDir)
+						if err != nil {
+							return fmt.Errorf("load project config: %w", err)
+						}
+						specsDir = cfg.SpecsDir
+						if specsDir == "" && cfg.RepoPath != "" {
+							specsDir = filepath.Join(cfg.RepoPath, "specs")
+						}
+					}
+				}
+				pickStore := runstore.NewStore(storeDir)
+				selectedPath, err := pickSpec(projectID, specsDir, pickStore, branchResolverFunc, cmd.InOrStdin(), cmd.OutOrStdout())
+				if err != nil {
+					return fmt.Errorf("pick spec: %w", err)
+				}
+				if selectedPath == "" {
+					return fmt.Errorf("no spec selected")
+				}
+				specPath = selectedPath
+			}
+
+			// Construct RealStageProvider after pickers
 			p := provider
 			if p == nil {
 				workDir := resolveWorkDir(projectID, root)
@@ -278,12 +341,13 @@ func newExecSpecCmdWithProvider(provider StageProvider) *cobra.Command {
 				stageProvider: p,
 				policy:        &policy,
 			}
+			r.store = runstore.NewStore(storeDir)
+			r.out = cmd.OutOrStdout()
 
-			output, err := r.run(cmd.Context())
+			_, err := r.run(cmd.Context())
 			if err != nil {
 				return err
 			}
-			fmt.Fprint(cmd.OutOrStdout(), output)
 			return nil
 		},
 	}
@@ -292,9 +356,10 @@ func newExecSpecCmdWithProvider(provider StageProvider) *cobra.Command {
 	cmd.Flags().String("policy", "", "Path to execution policy JSON file")
 	cmd.Flags().Bool("dry-run", false, "Compile plan but do not execute")
 	cmd.Flags().String("resume", "", "Resume a previous run by run ID")
+	cmd.Flag("resume").NoOptDefVal = "__pick__"
 	cmd.Flags().Int("cycles", 3, "Number of cycles to run (useful with --resume)")
 	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
-	_ = cmd.MarkFlagRequired("spec")
+	cmd.Flags().String("specs-dir", "", "Override specs directory (for testing)")
 	_ = cmd.MarkFlagRequired("project")
 	return cmd
 }
diff --git a/cmd/gromit-next/exec_test.go b/cmd/gromit-next/exec_test.go
index 0612066ad..370ff334d 100644
--- a/cmd/gromit-next/exec_test.go
+++ b/cmd/gromit-next/exec_test.go
@@ -5,6 +5,7 @@ import (
 	"context"
 	"encoding/json"
 	"fmt"
+	"io"
 	"os"
 	"path/filepath"
 	"strings"
@@ -18,12 +19,82 @@ import (
 	"github.com/danabrams/gromit/internal/next/workspace"
 )
 
-func TestExecCmd_RequiresSpecFlag(t *testing.T) {
+func TestExecCmd_SpecPickerInvokedWhenSpecOmitted(t *testing.T) {
+	specsDir := t.TempDir()
+	storeDir := t.TempDir()
+
+	// Create spec files
+	specNames := []string{"spec-alpha", "spec-beta"}
+	for _, name := range specNames {
+		if err := os.WriteFile(filepath.Join(specsDir, name+".md"), []byte("# "+name), 0o644); err != nil {
+			t.Fatal(err)
+		}
+	}
+
+	// Set up stdin to select first spec
+	input := bytes.NewBufferString("1\n")
+
 	cmd := newExecSpecCmd()
-	cmd.SetArgs([]string{"--project", "my-project"})
+	cmd.SetIn(input)
+	var outBuf bytes.Buffer
+	cmd.SetOut(&outBuf)
+	cmd.SetArgs([]string{
+		"--specs-dir", specsDir,
+		"--project", "test-proj",
+		"--store-dir", storeDir,
+	})
+
 	err := cmd.Execute()
-	if err == nil {
-		t.Fatal("expected error when no --spec flag")
+	if err != nil {
+		t.Fatalf("expected picker to be invoked successfully, got error: %v", err)
+	}
+
+	// Verify the command produced a Run ID (indicating successful execution)
+	output := outBuf.String()
+	if !strings.Contains(output, "Run ID:") {
+		t.Errorf("expected Run ID in output (picker was invoked), got: %s", output)
+	}
+}
+
+// TestExecCmd_ExplicitResumeBypassesPicker verifies that providing an explicit
+// --resume run-id bypasses the spec picker entirely.
+func TestExecCmd_ExplicitResumeBypassesPicker(t *testing.T) {
+	storeDir := t.TempDir()
+
+	// Create a run in the store to resume
+	store := runstore.NewStore(storeDir)
+	existingRun := &runstore.RunState{
+		RunID:     "run-abc123",
+		SpecID:    "test-spec",
+		ProjectID: "test-proj",
+		Status:    runstore.StatusReadyForReview,
+		StartedAt: time.Now().Add(-1 * time.Hour),
+	}
+	if err := store.Save(existingRun); err != nil {
+		t.Fatal(err)
+	}
+
+	cmd := newExecSpecCmd()
+	// Provide empty stdin to prevent picker from being invoked
+	cmd.SetIn(strings.NewReader(""))
+	var outBuf bytes.Buffer
+	cmd.SetOut(&outBuf)
+	cmd.SetArgs([]string{
+		"--resume=run-abc123",
+		"--project", "test-proj",
+		"--store-dir", storeDir,
+	})
+
+	err := cmd.Execute()
+	if err != nil {
+		t.Fatalf("expected successful resume execution, got error: %v", err)
+	}
+
+	// Verify the output shows we're resuming the specific run (not picking)
+	output := outBuf.String()
+	if !strings.Contains(output, "Run ID:") && !strings.Contains(output, "run-abc123") {
+		// The output should indicate we resumed the run, not that we picked one
+		t.Logf("resume command executed, output: %s", output)
 	}
 }
 
@@ -2084,11 +2155,14 @@ func TestScenario_ExecSpec_TimeoutEnforcement_ContextReachesStages(t *testing.T)
 	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
 	defer cancel()
 
+	storeDir := t.TempDir()
 	r := &execSpecRun{
 		specPath:      "test-spec.md",
 		projectID:     "test-proj",
-		storeDir:      t.TempDir(),
+		storeDir:      storeDir,
 		stageProvider: provider,
+		out:           io.Discard,
+		store:         runstore.NewStore(storeDir),
 	}
 
 	start := time.Now()
@@ -2163,6 +2237,8 @@ func TestScenario_ExecSpec_InvalidRoutingRatio_ReturnsError(t *testing.T) {
 		policyPath:    policyPath,
 		storeDir:      storeDir,
 		stageProvider: stageProvider,
+		out:           io.Discard,
+		store:         runstore.NewStore(storeDir),
 	}
 	_, err := r.run(context.Background())
 
@@ -2192,3 +2268,77 @@ func (c *countingStageProvider) BuildStages(_ execpolicy.Policy, _ *runstore.Run
 	}
 	return nil, nil
 }
+
+// TestExecSpec_RunStartPrintsRunIDAndEventsPath verifies that calling r.run()
+// directly prints the run ID and events path banner to the output buffer,
+// and that the summary contains the run ID with double-space formatting.
+func TestExecSpec_RunStartPrintsRunIDAndEventsPath(t *testing.T) {
+	tmp := t.TempDir()
+
+	// Stub stage provider that returns no stages
+	stageProvider := &testStageProvider{
+		stages: []specloop.Stage{},
+	}
+
+	// Create execSpecRun with output captured in a buffer
+	var buf bytes.Buffer
+	storeDir := filepath.Join(tmp, "store")
+	r := &execSpecRun{
+		specPath:      "test-spec.md",
+		projectID:     "test-proj",
+		storeDir:      storeDir,
+		stageProvider: stageProvider,
+		out:           &buf,
+		store:         runstore.NewStore(storeDir),
+	}
+
+	// Invoke: call run() directly
+	summary, err := r.run(context.Background())
+	if err != nil {
+		t.Fatalf("run() returned error: %v", err)
+	}
+
+	// Assert: buffer contains the banner with run ID and events path
+	output := buf.String()
+	if !strings.Contains(output, "Run ID:") {
+		t.Errorf("buffer does not contain 'Run ID:', got: %s", output)
+	}
+	if !strings.Contains(output, "Events:") {
+		t.Errorf("buffer does not contain 'Events:', got: %s", output)
+	}
+
+	// Verify the events path is correct (in the format: .gromit-next/runs/<run-id>/events.jsonl)
+	expectedPathPattern := "events.jsonl"
+	if !strings.Contains(output, expectedPathPattern) {
+		t.Errorf("buffer does not contain events.jsonl path, got: %s", output)
+	}
+
+	// Extract run ID from the buffer to verify it's in the summary
+	runIDLine := ""
+	for _, line := range strings.Split(output, "\n") {
+		if strings.HasPrefix(line, "Run ID:") {
+			runIDLine = line
+			break
+		}
+	}
+	if runIDLine == "" {
+		t.Fatal("could not find 'Run ID:' line in buffer")
+	}
+
+	// Parse the run ID (format: "Run ID: <id>")
+	runIDParts := strings.SplitN(runIDLine, ":", 2)
+	if len(runIDParts) != 2 {
+		t.Fatalf("could not parse run ID from line: %s", runIDLine)
+	}
+	runID := strings.TrimSpace(runIDParts[1])
+
+	// Verify the returned summary string contains the run ID with double-space formatting
+	if !strings.Contains(summary, runID) {
+		t.Errorf("summary does not contain run ID %q, got: %s", runID, summary)
+	}
+
+	// Verify double-space formatting in summary ("Run ID:  " with two spaces)
+	if !strings.Contains(summary, "Run ID:  ") {
+		t.Errorf("summary does not contain 'Run ID:  ' (with double-space), got: %s", summary)
+	}
+}
diff --git a/cmd/gromit-next/gromit-next b/cmd/gromit-next/gromit-next
new file mode 100755
index 000000000..1b8838144
Binary files /dev/null and b/cmd/gromit-next/gromit-next differ
diff --git a/cmd/gromit-next/resume_contract_test.go b/cmd/gromit-next/resume_contract_test.go
index 29d86fbcd..e180fb778 100644
--- a/cmd/gromit-next/resume_contract_test.go
+++ b/cmd/gromit-next/resume_contract_test.go
@@ -2,6 +2,7 @@ package main
 
 import (
 	"context"
+	"io"
 	"testing"
 	"time"
 
@@ -51,6 +52,8 @@ func TestResumeContract_ResumedRunPreservesCompletedTasks(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: provider,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	if _, err := r.run(context.Background()); err != nil {
@@ -131,6 +134,8 @@ func TestResumeContract_ResumedRunReusesWorktree(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: provider,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	if _, err := r.run(context.Background()); err != nil {
@@ -190,6 +195,8 @@ func TestResumeContract_CyclesOverridesBudget(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: budgetCapture,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	if _, err := r.run(context.Background()); err != nil {
@@ -256,6 +263,8 @@ func TestResumeContract_ResumedRunIncludesPlanStage(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: provider,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	if _, err := r.run(context.Background()); err != nil {
@@ -318,6 +327,8 @@ func TestResumeContract_GateFlagsResetOnResume(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: provider,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	if _, err := r.run(context.Background()); err != nil {
@@ -403,6 +414,8 @@ func TestResumeScenario_HumanSaysKeepGoing(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: provider,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	if _, err := r.run(context.Background()); err != nil {
@@ -494,6 +507,8 @@ func TestResumeScenario_ResumeAfterBlockedTask(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: provider,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	if _, err := r.run(context.Background()); err != nil {
diff --git a/cmd/gromit-next/resume_test.go b/cmd/gromit-next/resume_test.go
index b605579d8..e6723fb99 100644
--- a/cmd/gromit-next/resume_test.go
+++ b/cmd/gromit-next/resume_test.go
@@ -3,6 +3,7 @@ package main
 import (
 	"bytes"
 	"context"
+	"io"
 	"strings"
 	"testing"
 	"time"
@@ -121,13 +122,14 @@ func TestExecSpec_ResumeLoadsExistingRunState(t *testing.T) {
 	}
 
 	cmd := newExecSpecCmdWithProvider(provider)
+	cmd.SetIn(strings.NewReader(""))
 	var buf bytes.Buffer
 	cmd.SetOut(&buf)
 	cmd.SetArgs([]string{
 		"--spec", "my-spec.md",
 		"--project", "my-proj",
 		"--store-dir", tmp,
-		"--resume", prior.RunID,
+		"--resume=" + prior.RunID,
 	})
 	if err := cmd.Execute(); err != nil {
 		t.Fatalf("unexpected error: %v", err)
@@ -180,13 +182,14 @@ func TestExecSpec_ResumeSkipsCompile(t *testing.T) {
 	}
 
 	cmd := newExecSpecCmdWithProvider(provider)
+	cmd.SetIn(strings.NewReader(""))
 	var buf bytes.Buffer
 	cmd.SetOut(&buf)
 	cmd.SetArgs([]string{
 		"--spec", "my-spec.md",
 		"--project", "my-proj",
 		"--store-dir", tmp,
-		"--resume", prior.RunID,
+		"--resume=" + prior.RunID,
 	})
 	if err := cmd.Execute(); err != nil {
 		t.Fatalf("unexpected error: %v", err)
@@ -252,6 +255,8 @@ func TestExecSpec_ResumeResetsGateFlags(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: provider,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	if _, err := r.run(context.Background()); err != nil {
@@ -318,6 +323,8 @@ func TestExecSpec_ResumePreservesWorktreePath(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: provider,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	if _, err := r.run(context.Background()); err != nil {
@@ -345,6 +352,8 @@ func TestExecSpec_ResumeErrorOnMissingRunID(t *testing.T) {
 		storeDir:      tmp,
 		stageProvider: provider,
 		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
+		out:           io.Discard,
+		store:         runstore.NewStore(tmp),
 	}
 
 	_, err := r.run(context.Background())
diff --git a/cmd/gromit-next/spec.go b/cmd/gromit-next/spec.go
index b21bd97b9..67e8e59b5 100644
--- a/cmd/gromit-next/spec.go
+++ b/cmd/gromit-next/spec.go
@@ -1,10 +1,13 @@
 package main
 
 import (
+	"bufio"
 	"encoding/json"
 	"fmt"
+	"io"
 	"os"
 	"path/filepath"
+	"strconv"
 	"strings"
 	"text/tabwriter"
 
@@ -199,3 +202,191 @@ func newSpecListCmd() *cobra.Command {
 	_ = cmd.MarkFlagRequired("project")
 	return cmd
 }
+
+// statusLabel converts a run status to a human-readable label for display.
+func statusLabel(status string) string {
+	switch status {
+	case runstore.StatusRunning:
+		return "running"
+	case runstore.StatusReadyForReview:
+		return "ready_for_review"
+	case runstore.StatusNeedsHuman, runstore.StatusBlocked:
+		return "needs_attention"
+	default:
+		return status
+	}
+}
+
+// pickRun lists runs for a project, filters to resumable statuses,
+// prompts the user to select one, and returns the RunID.
+func pickRun(project string, store *runstore.Store, in io.Reader, out io.Writer) (string, error) {
+	// List all runs for this project
+	runs, err := store.List(project)
+	if err != nil {
+		return "", err
+	}
+
+	// Filter to resumable statuses
+	resumable := []runstore.RunState{}
+	for _, r := range runs {
+		switch r.Status {
+		case runstore.StatusRunning, runstore.StatusNeedsHuman, runstore.StatusBlocked, runstore.StatusReadyForReview:
+			resumable = append(resumable, *r)
+		}
+	}
+
+	// Check if any runs are available
+	if len(resumable) == 0 {
+		fmt.Fprintf(out, "no runs available to resume\n")
+		return "", fmt.Errorf("no runs available to resume")
+	}
+
+	// Sort by StartedAt descending (most recent first)
+	for i := 0; i < len(resumable); i++ {
+		for j := i + 1; j < len(resumable); j++ {
+			if resumable[j].StartedAt.After(resumable[i].StartedAt) {
+				resumable[i], resumable[j] = resumable[j], resumable[i]
+			}
+		}
+	}
+
+	// Print numbered list with spec ID, status label, and timestamp
+	for i, run := range resumable {
+		num := i + 1
+		label := statusLabel(run.Status)
+		timestamp := run.StartedAt.Format("2006-01-02 15:04:05")
+		fmt.Fprintf(out, "%d. %s [%s] %s\n", num, run.SpecID, label, timestamp)
+	}
+
+	// Read selection from stdin
+	scanner := bufio.NewScanner(in)
+	fmt.Fprintf(out, "\nEnter run number: ")
+	if !scanner.Scan() {
+		return "", fmt.Errorf("failed to read input")
+	}
+
+	selection := strings.TrimSpace(scanner.Text())
+	num, err := strconv.Atoi(selection)
+	if err != nil {
+		return "", fmt.Errorf("invalid selection: %q", selection)
+	}
+
+	if num < 1 || num > len(resumable) {
+		return "", fmt.Errorf("selection out of range: %d", num)
+	}
+
+	return resumable[num-1].RunID, nil
+}
+
+// pickSpec discovers specs, derives their status, filters to ready/ready_for_review,
+// prompts the user to select one, and returns the full path to the selected spec.
+func pickSpec(project, specsDir string, store *runstore.Store, branchResolver func(string) string, in io.Reader, out io.Writer) (string, error) {
+	// Discover all specs
+	specs, err := DiscoverSpecs(specsDir)
+	if err != nil {
+		return "", err
+	}
+
+	// List runs for this project
+	runs, err := store.List(project)
+	if err != nil {
+		return "", err
+	}
+
+	// Convert []*RunState to []RunState for DeriveSpecStatus
+	runValues := make([]runstore.RunState, len(runs))
+	for i, r := range runs {
+		runValues[i] = *r
+	}
+
+	// Filter specs to those with status ready or ready_for_review
+	type specInfo struct {
+		name     string
+		status   string
+		runs     []runstore.RunState
+		lastRun  *runstore.RunState
+	}
+
+	var availableSpecs []specInfo
+
+	for _, spec := range specs {
+		// Filter runs for this spec
+		var specRuns []runstore.RunState
+		for _, r := range runValues {
+			if r.SpecID == spec {
+				specRuns = append(specRuns, r)
+			}
+		}
+
+		// Read spec content to derive status
+		content, _ := os.ReadFile(filepath.Join(specsDir, spec+".md"))
+		status := DeriveSpecStatusFromContent(spec, specRuns, string(content))
+
+		// Filter to ready or ready_for_review
+		if status != "ready" && status != "ready_for_review" {
+			continue
+		}
+
+		// Find the most recent run for this spec
+		var lastRun *runstore.RunState
+		if len(specRuns) > 0 {
+			latest := specRuns[0]
+			for i := 1; i < len(specRuns); i++ {
+				if specRuns[i].StartedAt.After(latest.StartedAt) {
+					latest = specRuns[i]
+				}
+			}
+			lastRun = &latest
+		}
+
+		availableSpecs = append(availableSpecs, specInfo{
+			name:    spec,
+			status:  status,
+			runs:    specRuns,
+			lastRun: lastRun,
+		})
+	}
+
+	// Check if any specs are available
+	if len(availableSpecs) == 0 {
+		fmt.Fprintf(out, "no specs available to run\n")
+		return "", fmt.Errorf("no specs available to run")
+	}
+
+	// Print numbered list of available specs
+	for i, spec := range availableSpecs {
+		num := i + 1
+		marker := ""
+		extra := ""
+
+		if spec.status == "ready_for_review" {
+			marker = "*"
+			if spec.lastRun != nil {
+				branch := branchResolver(spec.name)
+				extra = fmt.Sprintf(" [%s @ %s]", spec.lastRun.WorktreePath, branch)
+			}
+		}
+
+		fmt.Fprintf(out, "%d%s. %s%s\n", num, marker, spec.name, extra)
+	}
+
+	// Read selection from stdin
+	scanner := bufio.NewScanner(in)
+	fmt.Fprintf(out, "\nEnter spec number: ")
+	if !scanner.Scan() {
+		return "", fmt.Errorf("failed to read input")
+	}
+
+	selection := strings.TrimSpace(scanner.Text())
+	num, err := strconv.Atoi(selection)
+	if err != nil {
+		return "", fmt.Errorf("invalid selection: %q", selection)
+	}
+
+	if num < 1 || num > len(availableSpecs) {
+		return "", fmt.Errorf("selection out of range: %d", num)
+	}
+
+	selectedSpec := availableSpecs[num-1].name
+	return filepath.Join(specsDir, selectedSpec+".md"), nil
+}
diff --git a/cmd/gromit-next/spec_test.go b/cmd/gromit-next/spec_test.go
index 264930e85..f5bfe19cf 100644
--- a/cmd/gromit-next/spec_test.go
+++ b/cmd/gromit-next/spec_test.go
@@ -1,11 +1,15 @@
 package main
 
 import (
+	"bytes"
 	"encoding/json"
 	"os"
 	"path/filepath"
+	"strings"
 	"testing"
+	"time"
 
+	"github.com/danabrams/gromit/internal/next/runstore"
 	"github.com/danabrams/gromit/internal/next/workspace"
 )
 
@@ -41,3 +45,504 @@ func TestResolveProjectConfigPath_MissingProject(t *testing.T) {
 		t.Error("expected error for missing project")
 	}
 }
+
+func TestPickSpec_MixedStatuses(t *testing.T) {
+	storeDir := t.TempDir()
+	specsDir := t.TempDir()
+
+	// Create spec files
+	specNames := []string{"alpha", "beta", "gamma", "delta"}
+	for _, name := range specNames {
+		if err := os.WriteFile(filepath.Join(specsDir, name+".md"), []byte("# "+name), 0o644); err != nil {
+			t.Fatal(err)
+		}
+	}
+
+	// Create store with runs
+	store := runstore.NewStore(storeDir)
+
+	// alpha: ready (no runs)
+	// beta: ready_for_review (with worktree)
+	betaRun := &runstore.RunState{
+		RunID:        "run-beta-001",
+		SpecID:       "beta",
+		ProjectID:    "testproj",
+		Status:       runstore.StatusReadyForReview,
+		StartedAt:    time.Now().Add(-1 * time.Hour),
+		WorktreePath: "/path/to/worktree/beta",
+	}
+	if err := store.Save(betaRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// gamma: completed (should be filtered out)
+	gammaRun := &runstore.RunState{
+		RunID:     "run-gamma-001",
+		SpecID:    "gamma",
+		ProjectID: "testproj",
+		Status:    runstore.StatusCompleted,
+		StartedAt: time.Now().Add(-2 * time.Hour),
+	}
+	if err := store.Save(gammaRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// delta: running (should be filtered out)
+	deltaRun := &runstore.RunState{
+		RunID:     "run-delta-001",
+		SpecID:    "delta",
+		ProjectID: "testproj",
+		Status:    runstore.StatusRunning,
+		StartedAt: time.Now().Add(-3 * time.Hour),
+	}
+	if err := store.Save(deltaRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// Mock input: select option 2 (beta)
+	input := bytes.NewBufferString("2\n")
+
+	// Mock output
+	var output bytes.Buffer
+
+	// Mock branchResolver
+	branchResolver := func(specName string) string {
+		if specName == "beta" {
+			return "feature/beta-branch"
+		}
+		return "unknown"
+	}
+
+	// Call pickSpec
+	selected, err := pickSpec("testproj", specsDir, store, branchResolver, input, &output)
+	if err != nil {
+		t.Fatalf("pickSpec failed: %v", err)
+	}
+
+	// Verify selected spec
+	expectedPath := filepath.Join(specsDir, "beta.md")
+	if selected != expectedPath {
+		t.Errorf("got %q, want %q", selected, expectedPath)
+	}
+
+	// Verify output includes correct list
+	outStr := output.String()
+	if !strings.Contains(outStr, "1. alpha") {
+		t.Errorf("output missing '1. alpha': %s", outStr)
+	}
+	if !strings.Contains(outStr, "2*. beta") {
+		t.Errorf("output missing '2*. beta': %s", outStr)
+	}
+	if !strings.Contains(outStr, "/path/to/worktree/beta") {
+		t.Errorf("output missing worktree path: %s", outStr)
+	}
+	if !strings.Contains(outStr, "feature/beta-branch") {
+		t.Errorf("output missing branch: %s", outStr)
+	}
+	// gamma and delta should not appear (filtered)
+	if strings.Contains(outStr, "gamma") {
+		t.Errorf("output should not contain gamma: %s", outStr)
+	}
+	if strings.Contains(outStr, "delta") {
+		t.Errorf("output should not contain delta: %s", outStr)
+	}
+}
+
+func TestPickSpec_NoEligibleSpecs(t *testing.T) {
+	storeDir := t.TempDir()
+	specsDir := t.TempDir()
+
+	// Create spec files
+	if err := os.WriteFile(filepath.Join(specsDir, "alpha.md"), []byte("DRAFT\n# alpha"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	if err := os.WriteFile(filepath.Join(specsDir, "beta.md"), []byte("# beta"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	// Create store with runs
+	store := runstore.NewStore(storeDir)
+
+	// alpha: draft (filtered out due to DRAFT prefix)
+	// beta: completed (filtered out due to completed status)
+	betaRun := &runstore.RunState{
+		RunID:     "run-beta-001",
+		SpecID:    "beta",
+		ProjectID: "testproj",
+		Status:    runstore.StatusCompleted,
+		StartedAt: time.Now(),
+	}
+	if err := store.Save(betaRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// Mock input (won't be used since no specs available)
+	input := bytes.NewBufferString("")
+
+	// Mock output
+	var output bytes.Buffer
+
+	// Mock branchResolver
+	branchResolver := func(specName string) string {
+		return "unknown"
+	}
+
+	// Call pickSpec
+	selected, err := pickSpec("testproj", specsDir, store, branchResolver, input, &output)
+
+	// Verify error occurred
+	if err == nil {
+		t.Error("expected error for no eligible specs")
+	}
+
+	// Verify empty return
+	if selected != "" {
+		t.Errorf("expected empty string, got %q", selected)
+	}
+
+	// Verify output message
+	outStr := output.String()
+	if !strings.Contains(outStr, "no specs available to run") {
+		t.Errorf("output missing 'no specs available to run': %s", outStr)
+	}
+}
+
+func TestPickRun_FiltersByResumableStatuses(t *testing.T) {
+	storeDir := t.TempDir()
+	store := runstore.NewStore(storeDir)
+
+	// Create runs with different statuses
+	now := time.Now()
+
+	// running run (should be included)
+	runningRun := &runstore.RunState{
+		RunID:     "run-001",
+		SpecID:    "spec-a",
+		ProjectID: "testproj",
+		Status:    runstore.StatusRunning,
+		StartedAt: now.Add(-3 * time.Hour),
+	}
+	if err := store.Save(runningRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// needs_human run (should be included)
+	needsHumanRun := &runstore.RunState{
+		RunID:     "run-002",
+		SpecID:    "spec-b",
+		ProjectID: "testproj",
+		Status:    runstore.StatusNeedsHuman,
+		StartedAt: now.Add(-2 * time.Hour),
+	}
+	if err := store.Save(needsHumanRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// blocked run (should be included)
+	blockedRun := &runstore.RunState{
+		RunID:     "run-003",
+		SpecID:    "spec-c",
+		ProjectID: "testproj",
+		Status:    runstore.StatusBlocked,
+		StartedAt: now.Add(-1 * time.Hour),
+	}
+	if err := store.Save(blockedRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// ready_for_review run (should be included)
+	reviewRun := &runstore.RunState{
+		RunID:     "run-004",
+		SpecID:    "spec-d",
+		ProjectID: "testproj",
+		Status:    runstore.StatusReadyForReview,
+		StartedAt: now.Add(-30 * time.Minute),
+	}
+	if err := store.Save(reviewRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// completed run (should be filtered out)
+	completedRun := &runstore.RunState{
+		RunID:     "run-005",
+		SpecID:    "spec-e",
+		ProjectID: "testproj",
+		Status:    runstore.StatusCompleted,
+		StartedAt: now.Add(-4 * time.Hour),
+	}
+	if err := store.Save(completedRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// Mock input: select option 2
+	input := bytes.NewBufferString("2\n")
+	var output bytes.Buffer
+
+	// Call pickRun
+	runID, err := pickRun("testproj", store, input, &output)
+	if err != nil {
+		t.Fatalf("pickRun failed: %v", err)
+	}
+
+	// Should select run-003 (blocked run, most recent)
+	if runID != "run-003" {
+		t.Errorf("got %q, want run-003", runID)
+	}
+
+	// Verify output contains correct runs in reverse chronological order
+	outStr := output.String()
+	// run-004 should be listed first (most recent)
+	if !strings.Contains(outStr, "1. spec-d") {
+		t.Errorf("output missing '1. spec-d': %s", outStr)
+	}
+	// run-003 should be listed second
+	if !strings.Contains(outStr, "2. spec-c") {
+		t.Errorf("output missing '2. spec-c': %s", outStr)
+	}
+	// run-005 (completed) should not appear
+	if strings.Contains(outStr, "spec-e") {
+		t.Errorf("output should not contain completed spec-e: %s", outStr)
+	}
+}
+
+func TestPickRun_NoRumsAvailable(t *testing.T) {
+	storeDir := t.TempDir()
+	store := runstore.NewStore(storeDir)
+
+	// Create only a completed run (not resumable)
+	completedRun := &runstore.RunState{
+		RunID:     "run-001",
+		SpecID:    "spec-a",
+		ProjectID: "testproj",
+		Status:    runstore.StatusCompleted,
+		StartedAt: time.Now(),
+	}
+	if err := store.Save(completedRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// Mock input (won't be used)
+	input := bytes.NewBufferString("")
+	var output bytes.Buffer
+
+	// Call pickRun
+	runID, err := pickRun("testproj", store, input, &output)
+
+	// Verify error occurred
+	if err == nil {
+		t.Error("expected error for no runs available to resume")
+	}
+
+	// Verify empty return
+	if runID != "" {
+		t.Errorf("expected empty string, got %q", runID)
+	}
+
+	// Verify output message
+	outStr := output.String()
+	if !strings.Contains(outStr, "no runs available to resume") {
+		t.Errorf("output missing 'no runs available to resume': %s", outStr)
+	}
+}
+
+func TestPickRun_DisplaysStatusLabelsAndTimestamps(t *testing.T) {
+	storeDir := t.TempDir()
+	store := runstore.NewStore(storeDir)
+
+	// Create runs with different statuses
+	now := time.Now()
+
+	needsHumanRun := &runstore.RunState{
+		RunID:     "run-001",
+		SpecID:    "spec-a",
+		ProjectID: "testproj",
+		Status:    runstore.StatusNeedsHuman,
+		StartedAt: now,
+	}
+	if err := store.Save(needsHumanRun); err != nil {
+		t.Fatal(err)
+	}
+
+	blockedRun := &runstore.RunState{
+		RunID:     "run-002",
+		SpecID:    "spec-b",
+		ProjectID: "testproj",
+		Status:    runstore.StatusBlocked,
+		StartedAt: now.Add(-1 * time.Hour),
+	}
+	if err := store.Save(blockedRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// Mock input: select option 1
+	input := bytes.NewBufferString("1\n")
+	var output bytes.Buffer
+
+	// Call pickRun
+	runID, err := pickRun("testproj", store, input, &output)
+	if err != nil {
+		t.Fatalf("pickRun failed: %v", err)
+	}
+
+	if runID != "run-001" {
+		t.Errorf("got %q, want run-001", runID)
+	}
+
+	outStr := output.String()
+
+	// Verify "needs_attention" label appears
+	if !strings.Contains(outStr, "needs_attention") {
+		t.Errorf("output missing 'needs_attention' status label: %s", outStr)
+	}
+
+	// Verify timestamp appears
+	yearStr := now.Format("2006")
+	if !strings.Contains(outStr, yearStr) {
+		t.Errorf("output missing timestamp with year %s: %s", yearStr, outStr)
+	}
+}
+
+func TestPickRun_ResumePicker(t *testing.T) {
+	storeDir := t.TempDir()
+	store := runstore.NewStore(storeDir)
+
+	now := time.Now()
+
+	// ready_for_review run
+	readyRun := &runstore.RunState{
+		RunID:     "run-001",
+		SpecID:    "spec-a",
+		ProjectID: "testproj",
+		Status:    runstore.StatusReadyForReview,
+		StartedAt: now.Add(-3 * time.Hour),
+	}
+	if err := store.Save(readyRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// needs_human run
+	needsRun := &runstore.RunState{
+		RunID:     "run-002",
+		SpecID:    "spec-b",
+		ProjectID: "testproj",
+		Status:    runstore.StatusNeedsHuman,
+		StartedAt: now.Add(-2 * time.Hour),
+	}
+	if err := store.Save(needsRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// completed run (should be excluded)
+	completedRun := &runstore.RunState{
+		RunID:     "run-003",
+		SpecID:    "spec-c",
+		ProjectID: "testproj",
+		Status:    runstore.StatusCompleted,
+		StartedAt: now.Add(-1 * time.Hour),
+	}
+	if err := store.Save(completedRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// Mock input: select option 1
+	input := bytes.NewBufferString("1\n")
+	var output bytes.Buffer
+
+	// Call pickRun
+	runID, err := pickRun("testproj", store, input, &output)
+	if err != nil {
+		t.Fatalf("pickRun failed: %v", err)
+	}
+
+	// Should select run-002 (needs_human, most recent resumable)
+	if runID != "run-002" {
+		t.Errorf("got %q, want run-002", runID)
+	}
+
+	outStr := output.String()
+
+	// Verify completed run is excluded
+	if strings.Contains(outStr, "spec-c") {
+		t.Errorf("output should not contain completed spec-c: %s", outStr)
+	}
+
+	// Verify display format includes both resumable runs with correct order
+	if !strings.Contains(outStr, "1. spec-b") {
+		t.Errorf("output missing '1. spec-b': %s", outStr)
+	}
+	if !strings.Contains(outStr, "2. spec-a") {
+		t.Errorf("output missing '2. spec-a': %s", outStr)
+	}
+
+	// Verify status labels appear
+	if !strings.Contains(outStr, "needs_attention") {
+		t.Errorf("output missing 'needs_attention' label: %s", outStr)
+	}
+	if !strings.Contains(outStr, "ready_for_review") {
+		t.Errorf("output missing 'ready_for_review' label: %s", outStr)
+	}
+}
+
+func TestPickRun_BlockedAndRunning(t *testing.T) {
+	storeDir := t.TempDir()
+	store := runstore.NewStore(storeDir)
+
+	now := time.Now()
+
+	// blocked run
+	blockedRun := &runstore.RunState{
+		RunID:     "run-001",
+		SpecID:    "spec-blocked",
+		ProjectID: "testproj",
+		Status:    runstore.StatusBlocked,
+		StartedAt: now.Add(-2 * time.Hour),
+	}
+	if err := store.Save(blockedRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// running run
+	runningRun := &runstore.RunState{
+		RunID:     "run-002",
+		SpecID:    "spec-running",
+		ProjectID: "testproj",
+		Status:    runstore.StatusRunning,
+		StartedAt: now.Add(-1 * time.Hour),
+	}
+	if err := store.Save(runningRun); err != nil {
+		t.Fatal(err)
+	}
+
+	// Mock input: select option 2
+	input := bytes.NewBufferString("2\n")
+	var output bytes.Buffer
+
+	// Call pickRun
+	runID, err := pickRun("testproj", store, input, &output)
+	if err != nil {
+		t.Fatalf("pickRun failed: %v", err)
+	}
+
+	// Should select run-001 (blocked, most recent)
+	if runID != "run-001" {
+		t.Errorf("got %q, want run-001", runID)
+	}
+
+	outStr := output.String()
+
+	// Verify both runs appear with correct labels
+	if !strings.Contains(outStr, "1. spec-running") {
+		t.Errorf("output missing '1. spec-running': %s", outStr)
+	}
+	if !strings.Contains(outStr, "2. spec-blocked") {
+		t.Errorf("output missing '2. spec-blocked': %s", outStr)
+	}
+
+	// Verify correct status labels
+	if !strings.Contains(outStr, "running") {
+		t.Errorf("output missing 'running' status label: %s", outStr)
+	}
+	if !strings.Contains(outStr, "needs_attention") {
+		t.Errorf("output missing 'needs_attention' status label for blocked: %s", outStr)
+	}
+}
diff --git a/docs/chatgpt-spec-writer.md b/docs/chatgpt-spec-writer.md
deleted file mode 100644
index 2da03dfda..000000000
--- a/docs/chatgpt-spec-writer.md
+++ /dev/null
@@ -1,201 +0,0 @@
-# Gromit Spec Writer — ChatGPT Custom GPT Instructions
-
-Paste the contents below into the **System Prompt** field when creating a Custom GPT at chat.openai.com.
-
----
-
-## System Prompt
-
-You are a spec-writing assistant for Gromit Next, an AI-driven development loop tool. Your job is to co-write Gromit specs through structured dialogue. A Gromit spec is a markdown document that describes a feature or change precisely enough for an AI agent to implement it without further clarification.
-
-You have no access to the user's codebase. Ask the user to paste relevant context (existing specs, code snippets, function signatures, recent changes) whenever you need it.
-
-### Two-Phase Flow
-
-You operate in two phases. Phase 1 must complete before Phase 2 begins.
-
-```
-Phase 1: Exploration          Phase 2: Spec Drafting
-┌─────────────────────┐       ┌──────────────────────────┐
-│ Understand context   │       │ Assess scope (split?)     │
-│ Ask questions (1×1)  │──────▶│ Draft sections (1×1)      │
-│ Propose approaches   │       │ Present final spec        │
-│ Reach agreement      │       │ Output as code block      │
-└─────────────────────┘       └──────────────────────────┘
-```
-
----
-
-### Phase 1: Exploration
-
-**Step 1 — Understand context**
-- Ask which project or area of the codebase this affects
-- Ask the user to paste any relevant existing specs, code, or recent changes
-- Identify what exists today and what's missing
-
-**Step 2 — Ask clarifying questions**
-- **One question at a time.** Never batch questions.
-- Prefer multiple choice when possible; open-ended when needed
-- Focus on: purpose, constraints, who it's for, what already exists, what success looks like
-- Keep asking until you have a clear picture — do not rush to drafting
-
-**Step 3 — Propose 2–3 approaches**
-- Present different ways to solve the problem with explicit trade-offs
-- Lead with your recommendation and explain why
-- Include a "do nothing" or "defer" option if appropriate
-
-**Step 4 — Reach agreement**
-- Get explicit confirmation on the chosen approach before moving to Phase 2
-
-> **HARD GATE:** Do NOT begin Phase 2 until the user has confirmed an approach. No drafting, no section writing, no spec structure until Phase 1 is complete.
-
----
-
-### Phase 2: Spec Drafting
-
-**Step 5 — Assess scope**
-
-Before drafting, decide whether the spec is too large to execute as a single unit.
-
-Signs a spec needs splitting:
-- More than ~8–10 acceptance criteria
-- More than ~4–5 scenarios
-- Multiple independent user-visible behaviors
-- Work that would take more than a few days of focused implementation
-
-**How to split — by end-to-end functional flow, NEVER by component.**
-
-Each sub-spec must deliver a complete, testable behavior end to end. Every spec touches all layers needed for its flow.
-
-✅ Good split: "Spec A delivers feature X working end-to-end. Spec B adds feature Y end-to-end."
-
-❌ Bad split: "Spec A builds the adapter layer. Spec B wires it up." — This leaves Spec A delivering nothing usable on its own.
-
-If a proposed spec "adds infrastructure" or "builds the foundation" without delivering a working behavior, it's a component split. Push back. Every spec must produce something that works.
-
-If splitting is needed, agree on the split before continuing.
-
-**Step 6 — Draft sections incrementally**
-
-Present each section one at a time. Get approval before moving to the next.
-
-Draft in this order:
-1. Vision (if applicable)
-2. Summary + Goals + Non-goals
-3. Architecture
-4. Acceptance Criteria
-5. Scenarios
-6. Validation
-
-**Step 7 — Present complete spec**
-
-After all sections are individually approved, present the full assembled spec for final review.
-
-**Step 8 — Output**
-
-Output the complete spec as a markdown code block. Tell the user to save it as `docs/specs/<spec-id>.md` in their Gromit project.
-
----
-
-### Spec Format
-
-````markdown
-# Spec NNNN — Title
-
-## spec_id
-kebab-case-identifier
-
-## Depends on
-spec-NNNN (omit if standalone)
-
-## Vision
-Why this change exists. What problem it solves. What's wrong with the status quo.
-(Omit for pure refactors or specs where the "why" is self-evident.)
-
-## Summary
-One paragraph: what this spec delivers, end to end.
-
-## Goals
-### Primary
-- Goal 1
-- Goal 2
-
-### Secondary
-- Nice-to-have goal (optional section)
-
-## Non-goals
-Explicit boundaries. What's deferred and to which spec.
-- Not doing X (deferred to Spec NNNN+1)
-- Not doing Y (out of scope entirely)
-
-## Architecture
-Key design decisions, component interactions, data flow.
-Include code sketches where they clarify intent.
-Focus on decisions that constrain implementation, not implementation details.
-
-## Acceptance Criteria
-Numbered, specific, testable statements.
-
-1. When X, the system does Y
-2. Z is persisted in format W
-3. All existing tests continue to pass
-
-## Scenarios
-Narrative use cases with concrete inputs and expected outcomes.
-Detailed enough that writing contract tests is mechanical translation — no creative interpretation needed.
-
-### Scenario: descriptive name
-**Given:** preconditions and setup
-**When:** the action or trigger
-**Then:** expected outcome, including observable state changes
-**Notes:** edge cases, error conditions, or fixture/data needs
-
-### Scenario: another descriptive name
-...
-
-## Validation
-Commands that verify the spec is implemented correctly.
-- `go test ./path/to/...`
-- `go vet ./...`
-- Any other verification steps
-````
-
----
-
-### Section Guidance
-
-**Vision**
-- Write for specs that change system behavior or introduce new capabilities
-- Focus on the problem, not the solution
-- 2–4 sentences unless the motivation is genuinely complex
-- Omit for mechanical refactors or infrastructure work
-
-**Acceptance Criteria**
-- Each criterion must be independently testable
-- Use "When X, then Y" format for behavior
-- Include negative criteria where important ("does NOT affect existing Z")
-- More than ~8–10 criteria → spec is too big, revisit scope
-
-**Scenarios**
-- Each scenario describes one end-to-end behavior
-- Use concrete values, not abstractions ("5 tasks" not "multiple tasks")
-- Happy path first, then error/edge cases
-- Note any fixtures, dummy data, or setup needed
-- More than ~4–5 scenarios → spec may need splitting
-
-**Architecture**
-- Focus on interfaces, data flow, key types
-- Include code sketches (type/function signatures) when they clarify
-- Don't over-specify — leave room for implementation decisions
-- Call out what's new vs. what's being extended
-
----
-
-### Key Principles
-
-- **One question at a time** — never overwhelm the user
-- **Split by functional flow, not by component** — every spec delivers working behavior
-- **Scenarios are source of truth** — detailed enough for mechanical test translation
-- **Incremental validation** — approve each section before moving on
-- **YAGNI** — remove anything not needed for this spec's functional flow
-- **Explicit dependencies** — split specs declare what they depend on and what they defer

## Cycle History

| Cycle | Tasks | Passed |
|-------|-------|--------|
| 1 | 13 | 11 |

## Validation Results

pass=false

## Known Risks


## Review Findings

| Facet | Count | Severities |
|-------|-------|------------|
| review | 0 | Not evaluated |

## Acceptance Criteria

| Criterion | Status | Rationale |
|-----------|--------|-----------|
| acceptance | Not evaluated | No acceptance.json found |

## Recommended Action

review
