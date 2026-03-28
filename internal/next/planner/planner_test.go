package planner

import (
	"context"
	"strings"
	"testing"
)

func TestPlanner_InvokesAgentAndParsesPlan(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":1,"kind":"original","tasks":[{"task_id":"t-001","objective":"x","expected_touched_area":["a/"],"proof_checks":["true"]}]}`
	agent := &fakeAgent{output: validJSON}
	p := NewPlanner(agent, "high")

	plan, err := p.CreatePlan(context.Background(), PlanRequest{
		SpecPacket: "build a thing", Cycle: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(plan.Tasks))
	}
	if !agent.called {
		t.Fatal("agent not called")
	}
}

type fakeAgent struct {
	output  string
	called  bool
	outputs []string // if set, returns outputs[callCount-1] on each call
	calls   int
	prompts []string // captures each prompt passed to Invoke
}

func (f *fakeAgent) Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error) {
	f.called = true
	f.calls++
	f.prompts = append(f.prompts, prompt)
	out := f.output
	if len(f.outputs) > 0 && f.calls <= len(f.outputs) {
		out = f.outputs[f.calls-1]
	}
	return AgentResult{Output: out, TokensIn: 100, TokensOut: 50, Cost: 0.01, Model: "fake-model"}, nil
}

func TestPlanner_CreateFixPlan(t *testing.T) {
	fixJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix lint","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["lint failure"]}]}`
	agent := &fakeAgent{output: fixJSON}
	p := NewPlanner(agent, "high")

	plan, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		CompletedTasks: []CompletedTask{{
			TaskID:            "t-001",
			Attempts:          1,
			FilesChanged:      []string{"a/foo.go"},
			ValidationOutcome: "passed",
		}},
		Failures:    []string{"lint failure in a/"},
		CurrentDiff: "diff --git ...",
		Cycle:       2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Kind != "fix" {
		t.Fatalf("want fix, got %s", plan.Kind)
	}
	if plan.Tasks[0].ParentCycle != 1 {
		t.Fatal("expected parent_cycle=1")
	}
}

func TestPlanner_CreatePlan_RetryOnParseFailure(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":1,"kind":"original","tasks":[{"task_id":"t-001","objective":"x","expected_touched_area":["a/"],"proof_checks":["true"]}]}`
	agent := &fakeAgent{
		outputs: []string{"not valid json", validJSON},
	}
	p := NewPlanner(agent, "high")

	plan, err := p.CreatePlan(context.Background(), PlanRequest{SpecPacket: "spec", Cycle: 1})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(plan.Tasks))
	}
	if agent.calls != 2 {
		t.Fatalf("expected 2 agent calls, got %d", agent.calls)
	}
}

func TestPlanner_CreatePlan_RetryExhaustion(t *testing.T) {
	agent := &fakeAgent{
		outputs: []string{"bad json", "still bad"},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreatePlan(context.Background(), PlanRequest{SpecPacket: "spec", Cycle: 1})
	if err == nil {
		t.Fatal("expected error after retry exhaustion")
	}
	if agent.calls != 2 {
		t.Fatalf("expected 2 agent calls, got %d", agent.calls)
	}
}

func TestPlanner_CreateFixPlan_RetryOnParseFailure(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-005","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{
		outputs: []string{"garbage", validJSON},
	}
	p := NewPlanner(agent, "high")

	plan, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if plan.Tasks[0].TaskID != "t-005" {
		t.Fatalf("unexpected task ID: %s", plan.Tasks[0].TaskID)
	}
}

func TestPlanner_CreateFixPlan_ValidatePlanWithPrior(t *testing.T) {
	// Task ID t-002 is <= prior max t-004, should fail validation
	badIDJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{
		outputs: []string{badIDJSON, badIDJSON},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err == nil {
		t.Fatal("expected error: fix plan task IDs must be > prior max")
	}
	if agent.calls != 2 {
		t.Fatalf("expected 2 calls (retry), got %d", agent.calls)
	}
}

func TestPlanner_CreateFixPlan_PriorValidationSucceeds(t *testing.T) {
	goodJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-005","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{output: goodJSON}
	p := NewPlanner(agent, "high")

	plan, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Tasks[0].TaskID != "t-005" {
		t.Fatalf("unexpected task ID: %s", plan.Tasks[0].TaskID)
	}
}

func TestBuildFixPlanPrompt_ForbidsReplanningCompletedTasks(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"lint error"},
		Cycle:        2,
	})
	if !strings.Contains(prompt, "Do NOT replan") {
		t.Fatal("fix plan prompt must forbid replanning completed tasks")
	}
}

func TestBuildFixPlanPrompt_IncludesSpecConstraints(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan:    Plan{SpecID: "s1", Cycle: 1},
		Failures:        []string{"format error"},
		Cycle:           2,
		SpecConstraints: "## Out-of-Scope\n- Do NOT modify any existing test files",
	})
	if !strings.Contains(prompt, "Do NOT modify any existing test files") {
		t.Fatal("fix plan prompt must include spec constraints")
	}
	if !strings.Contains(prompt, "HARD REQUIREMENTS") {
		t.Fatal("fix plan prompt must label spec constraints as HARD REQUIREMENTS")
	}
	// Constraints must appear before failures so the LLM anchors on them first
	constraintsIdx := strings.Index(prompt, "HARD REQUIREMENTS")
	failuresIdx := strings.Index(prompt, "Review Findings")
	if failuresIdx < 0 {
		failuresIdx = strings.Index(prompt, "Validation Failures")
	}
	if failuresIdx >= 0 && constraintsIdx > failuresIdx {
		t.Fatal("spec constraints must appear before failures in the fix plan prompt")
	}
}

func TestBuildFixPlanPrompt_NoSpecConstraintsSection_WhenEmpty(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"lint error"},
		Cycle:        2,
	})
	if strings.Contains(prompt, "HARD REQUIREMENTS") {
		t.Fatal("fix plan prompt must not include HARD REQUIREMENTS section when spec constraints are empty")
	}
}

func TestPlanner_CreatePlan_RetryFeedsBackParseError(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":1,"kind":"original","tasks":[{"task_id":"t-001","objective":"x","expected_touched_area":["a/"],"proof_checks":["true"]}]}`
	agent := &fakeAgent{
		outputs: []string{"not valid json", validJSON},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreatePlan(context.Background(), PlanRequest{SpecPacket: "spec", Cycle: 1})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if len(agent.prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(agent.prompts))
	}
	// First prompt should NOT contain error feedback
	if strings.Contains(agent.prompts[0], "Your previous output was invalid") {
		t.Fatal("first prompt should not contain error feedback")
	}
	// Second prompt should contain the parse error
	if !strings.Contains(agent.prompts[1], "Your previous output was invalid") {
		t.Fatal("second prompt should contain error feedback")
	}
	if !strings.Contains(agent.prompts[1], "Please produce valid JSON output") {
		t.Fatal("second prompt should ask for valid JSON")
	}
}

func TestPlanner_CreateFixPlan_RetryFeedsBackParseError(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-005","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{
		outputs: []string{"garbage", validJSON},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if len(agent.prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(agent.prompts))
	}
	if strings.Contains(agent.prompts[0], "Your previous output was invalid") {
		t.Fatal("first prompt should not contain error feedback")
	}
	if !strings.Contains(agent.prompts[1], "Your previous output was invalid") {
		t.Fatal("second prompt should contain error feedback")
	}
}

func TestBuildPlanPrompt_RequiresTestFileProofChecks(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if !strings.Contains(prompt, "*_test.go") {
		t.Fatal("buildPlanPrompt must instruct LLM to require proof checks for *_test.go files")
	}
	if !strings.Contains(prompt, "Do NOT rely solely on `go test ./...`") {
		t.Fatal("buildPlanPrompt must warn that go test passes even without new test assertions")
	}
}

func TestBuildFixPlanPrompt_RequiresTestFileProofChecks(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"missing test coverage"},
		Cycle:        2,
	})
	if !strings.Contains(prompt, "*_test.go") {
		t.Fatal("buildFixPlanPrompt must instruct LLM to require proof checks for *_test.go files")
	}
	if !strings.Contains(prompt, "Do NOT rely solely on `go test ./...`") {
		t.Fatal("buildFixPlanPrompt must warn that go test passes even without new test assertions")
	}
}

func TestPlanner_CreateFixPlan_RetryFeedsBackValidationError(t *testing.T) {
	// First output: valid JSON but task ID t-002 <= prior max t-004
	badIDJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	// Second output: valid JSON with task ID t-005 > prior max t-004
	goodJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-005","objective":"fix","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["err"]}]}`
	agent := &fakeAgent{
		outputs: []string{badIDJSON, goodJSON},
	}
	p := NewPlanner(agent, "high")

	_, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan:   Plan{SpecID: "s1", Cycle: 1},
		Failures:       []string{"err"},
		Cycle:          2,
		PriorMaxTaskID: "t-004",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if len(agent.prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(agent.prompts))
	}
	if strings.Contains(agent.prompts[0], "Your previous output was invalid") {
		t.Fatal("first prompt should not contain error feedback")
	}
	if !strings.Contains(agent.prompts[1], "prior-plan validation failed") {
		t.Fatal("second prompt should contain prior-plan validation error")
	}
}

func TestBuildFixPlanPrompt_InstructsAboutFixesField(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"compilation error in main.go"},
		Cycle:        2,
	})
	if !strings.Contains(prompt, "fixes") {
		t.Fatal("buildFixPlanPrompt must mention the 'fixes' field")
	}
	if !strings.Contains(prompt, "failed task") {
		t.Fatal("buildFixPlanPrompt must reference 'failed task' when describing the fixes field")
	}
}

func TestBuildFixPlanPrompt_PersistentFailures_RendersAuditSection(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures: []string{
			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
		},
		Cycle: 2,
	})
	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
		t.Fatal("buildFixPlanPrompt must include persistent failures audit section when present")
	}
	if !strings.Contains(prompt, "persistent-failure: TestFoo has failed 2 consecutive cycles") {
		t.Fatal("persistent failure hint must appear in the audit section")
	}
}

func TestBuildFixPlanPrompt_PersistentFailures_AppearsBeforeValidationSection(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures: []string{
			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
			"compilation error in main.go",
		},
		Cycle: 2,
	})
	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
	if persistentIdx < 0 {
		t.Fatal("persistent failures section must be present")
	}
	if validationIdx < 0 {
		t.Fatal("validation failures section must be present")
	}
	if persistentIdx > validationIdx {
		t.Fatal("persistent failures section must appear before validation failures section")
	}
}

func TestBuildFixPlanPrompt_PersistentFailures_AlsoAppearsInValidationSection(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures: []string{
			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
		},
		Cycle: 2,
	})
	// Count occurrences of the persistent failure hint
	persistentHint := "persistent-failure: TestFoo has failed 2 consecutive cycles"
	count := strings.Count(prompt, persistentHint)
	if count < 2 {
		t.Fatalf("persistent failure hint must appear at least twice (audit section and validation section), got %d", count)
	}
}

func TestBuildFixPlanPrompt_NoPersistentFailures_NoSection(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"compilation error in main.go"},
		Cycle:        2,
	})
	if strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
		t.Fatal("persistent failures section must not appear when there are no persistent failures")
	}
}

func TestBuildFixPlanPrompt_MultiplePersistentFailures_MixedWithNonPersistent(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures: []string{
			"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification",
			"lint error in package.go",
			"persistent-failure: TestBar has failed 3 consecutive cycles — may indicate a bad test specification",
			"compilation error in main.go",
		},
		Cycle: 2,
	})
	// Check persistent failures section exists
	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
		t.Fatal("persistent failures section must be present")
	}
	// Check both persistent failures appear in their section
	if !strings.Contains(prompt, "persistent-failure: TestFoo has failed 2 consecutive cycles") {
		t.Fatal("first persistent failure must appear in audit section")
	}
	if !strings.Contains(prompt, "persistent-failure: TestBar has failed 3 consecutive cycles") {
		t.Fatal("second persistent failure must appear in audit section")
	}
	// Check validation section still exists
	if !strings.Contains(prompt, "## Validation Failures to Fix") {
		t.Fatal("validation failures section must be present")
	}
	// Check non-persistent failures appear in validation section
	if !strings.Contains(prompt, "lint error in package.go") {
		t.Fatal("non-persistent failure must appear in validation section")
	}
	if !strings.Contains(prompt, "compilation error in main.go") {
		t.Fatal("non-persistent failure must appear in validation section")
	}
}

func TestBuildFixPlanPrompt_PersistentFailureHint_WithoutCorrespondingContractFailure(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures: []string{
			"persistent-failure: contract:scenario-contracts.yaml scenario-name has failed 2 consecutive cycles — may indicate a bad test specification",
		},
		Cycle: 2,
	})
	// Persistent failures section must exist even without corresponding contract failure
	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
		t.Fatal("persistent failures section must be present even without corresponding contract failure")
	}
	// The persistent failure hint must appear in the section
	if !strings.Contains(prompt, "persistent-failure: contract:scenario-contracts.yaml") {
		t.Fatal("persistent-failure hint must appear in audit section")
	}
}

func TestBuildFixPlanPrompt_EmptyFailures_NoPersistentSection(t *testing.T) {
	req := FixPlanRequest{
		Failures: nil,
	}
	prompt := buildFixPlanPrompt(req)
	if strings.Contains(prompt, "## Persistent Failures") {
		t.Error("empty failures must not render persistent failures section")
	}
}

func TestBuildFixPlanPrompt_MultiplePersistentFailures_AcrossDifferentContracts(t *testing.T) {
	// Test multiple persistent failures across different contracts
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures: []string{
			`contract:contract-alpha — file_contains failed: pattern "ExpectedType" not found in "types.go"`,
			`persistent-failure: contract:contract-alpha has failed 2 consecutive cycles — may indicate a bad test specification`,
			`contract:contract-beta — assertion failed: expected 5 assertions but got 3`,
			`persistent-failure: contract:contract-beta has failed 3 consecutive cycles — may indicate a bad test specification`,
			`contract:contract-gamma — timeout after 30s`,
		},
		Cycle: 2,
	})

	// Assert: persistent failures section exists
	if !strings.Contains(prompt, "## Persistent Failures — Possible Bad Contracts") {
		t.Fatal("persistent failures section must be present")
	}

	// Assert: both persistent failures appear in the persistent section
	if !strings.Contains(prompt, "persistent-failure: contract:contract-alpha has failed 2 consecutive cycles") {
		t.Fatal("first persistent failure must appear in persistent section")
	}
	if !strings.Contains(prompt, "persistent-failure: contract:contract-beta has failed 3 consecutive cycles") {
		t.Fatal("second persistent failure must appear in persistent section")
	}

	// Assert: validation failures section exists
	if !strings.Contains(prompt, "## Validation Failures to Fix") {
		t.Fatal("validation failures section must be present")
	}

	// Assert: all three contract failures appear in validation section
	validationIdx := strings.Index(prompt, "## Validation Failures to Fix")
	validationSection := prompt[validationIdx:]
	if !strings.Contains(validationSection, "contract:contract-alpha") {
		t.Fatal("first contract failure must appear in validation section")
	}
	if !strings.Contains(validationSection, "contract:contract-beta") {
		t.Fatal("second contract failure must appear in validation section")
	}
	if !strings.Contains(validationSection, "contract:contract-gamma") {
		t.Fatal("third contract failure must appear in validation section")
	}

	// Assert: non-persistent failure (contract-gamma timeout) does not appear in persistent section
	persistentIdx := strings.Index(prompt, "## Persistent Failures — Possible Bad Contracts")
	persistentSection := prompt[persistentIdx:validationIdx]
	if strings.Contains(persistentSection, "contract:contract-gamma") {
		t.Fatal("non-persistent failure must not appear in persistent section")
	}

	// Assert: persistent section appears before validation section
	if persistentIdx > validationIdx {
		t.Fatal("persistent failures section must appear before validation failures section")
	}
}

func TestBuildPlanPrompt_TaskGranularityConstraint(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if !strings.Contains(prompt, "at most 3-4 files") {
		t.Fatal("buildPlanPrompt must include file count constraint per task")
	}
	if !strings.Contains(prompt, "one task per scenario") {
		t.Fatal("buildPlanPrompt must instruct one task per scenario for scenario-driven work")
	}
}

func TestBuildFixPlanPrompt_TaskGranularityConstraint(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"test failure"},
		Cycle:        2,
	})
	if !strings.Contains(prompt, "at most 3-4 files") {
		t.Fatal("buildFixPlanPrompt must include file count constraint per task")
	}
	if !strings.Contains(prompt, "one task per scenario") {
		t.Fatal("buildFixPlanPrompt must instruct one task per scenario for scenario-driven work")
	}
}

func TestBuildPlanPrompt_ProofCheckQualityGuidelines(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	// Compilation mandatory
	if !strings.Contains(prompt, "go build ./...") {
		t.Fatal("buildPlanPrompt must require go build ./... for tasks modifying .go files")
	}
	// Behavioral over presence
	if !strings.Contains(prompt, "Behavioral over presence") {
		t.Fatal("buildPlanPrompt must include behavioral over presence guidance")
	}
	if !strings.Contains(prompt, "go test -run TestX") {
		t.Fatal("buildPlanPrompt must show go test -run as preferred over grep")
	}
	// Order verification
	if !strings.Contains(prompt, "Order and sequence verification") {
		t.Fatal("buildPlanPrompt must include order verification guidance")
	}
	if !strings.Contains(prompt, "awk") {
		t.Fatal("buildPlanPrompt must show awk example for order verification")
	}
	// Config flow
	if !strings.Contains(prompt, "Config flow verification") {
		t.Fatal("buildPlanPrompt must include config flow verification guidance")
	}
	// Integration wiring
	if !strings.Contains(prompt, "Integration wiring verification") {
		t.Fatal("buildPlanPrompt must include integration wiring verification guidance")
	}
	if !strings.Contains(prompt, "function CALL") {
		t.Fatal("buildPlanPrompt must emphasize verifying function calls not just imports")
	}
}

func TestBuildPlanPrompt_ContainsRuntimeOverSourceGrepRule(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if !strings.Contains(prompt, "Runtime over source-grep") {
		t.Fatal("buildPlanPrompt must include rule 7 about runtime over source-grep for behavioral properties")
	}
	if !strings.Contains(prompt, "Rule 7") {
		t.Fatal("buildPlanPrompt must explicitly mention Rule 7 in the runtime guidance")
	}
	if !strings.Contains(prompt, "--help") {
		t.Fatal("buildPlanPrompt rule 7 must include --help example for runtime verification")
	}
}

func TestBuildPlanPrompt_ContainsSuspectProofCheckInstruction(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if !strings.Contains(prompt, "[suspect-proof-check]") {
		t.Fatal("buildPlanPrompt must include suspect-proof-check guidance")
	}
	if !strings.Contains(prompt, "proof-check rewrite") {
		t.Fatal("buildPlanPrompt must explain proof-check rewrite tasks for suspect proof-check failures")
	}
}

func TestBuildFixPlanPrompt_ContainsRuntimeOverSourceGrepRule(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		Cycle:    2,
		Failures: []string{"some failure"},
	})
	if !strings.Contains(prompt, "Runtime over source-grep") {
		t.Fatal("buildFixPlanPrompt must contain 'Runtime over source-grep' rule")
	}
	if !strings.Contains(prompt, "Rule 7") {
		t.Fatal("buildFixPlanPrompt must explicitly mention Rule 7 in the runtime guidance")
	}
	if !strings.Contains(prompt, "--help") {
		t.Fatal("buildFixPlanPrompt must contain '--help' example in runtime-over-source-grep rule")
	}
}

func TestBuildFixPlanPrompt_ProofCheckQualityGuidelines(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"test failure"},
		Cycle:        2,
	})
	// Compilation mandatory
	if !strings.Contains(prompt, "go build ./...") {
		t.Fatal("buildFixPlanPrompt must require go build ./... for tasks modifying .go files")
	}
	// Behavioral over presence
	if !strings.Contains(prompt, "go test -run TestX") {
		t.Fatal("buildFixPlanPrompt must show go test -run as preferred over grep")
	}
	// Order verification with awk
	if !strings.Contains(prompt, "awk") {
		t.Fatal("buildFixPlanPrompt must show awk example for order verification")
	}
	// Config flow
	if !strings.Contains(prompt, "config") || !strings.Contains(prompt, "READ") {
		t.Fatal("buildFixPlanPrompt must include config flow verification (verify config is READ)")
	}
	// Integration wiring — verify function CALL not just import
	if !strings.Contains(prompt, "function CALL") {
		t.Fatal("buildFixPlanPrompt must emphasize verifying function calls not just imports")
	}
}

func TestBuildFixPlanPrompt_ContainsSuspectProofCheckInstruction(t *testing.T) {
	req := FixPlanRequest{Cycle: 2, Failures: []string{"[suspect-proof-check] grep failed"}}
	prompt := buildFixPlanPrompt(req)
	if !strings.Contains(prompt, "[suspect-proof-check]") {
		t.Error("expected BuildFixPlanPrompt to contain suspect-proof-check instruction")
	}
	if !strings.Contains(prompt, "proof-check rewrite") {
		t.Error("expected instruction to mention proof-check rewrite task")
	}
}

func TestBuildFixPlanPrompt_PlaybookHeuristics(t *testing.T) {
	// When PlaybookHeuristics is non-empty, section should be included
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan:       Plan{SpecID: "s1", Cycle: 1},
		Failures:           []string{"test failure"},
		Cycle:              2,
		PlaybookHeuristics: "Prioritize quick wins first\nAvoid deep refactoring in fix cycles",
	})
	if !strings.Contains(prompt, "## Playbook Heuristics") {
		t.Fatal("buildFixPlanPrompt must include Playbook Heuristics section when field is non-empty")
	}
	if !strings.Contains(prompt, "Prioritize quick wins first") {
		t.Fatal("buildFixPlanPrompt must include PlaybookHeuristics content")
	}

	// When PlaybookHeuristics is empty, section should be omitted
	prompt = buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"test failure"},
		Cycle:        2,
	})
	if strings.Contains(prompt, "## Playbook Heuristics") {
		t.Fatal("buildFixPlanPrompt must omit Playbook Heuristics section when field is empty")
	}
}

func TestBuildPlanPrompt_PlaybookHeuristics(t *testing.T) {
	// When PlaybookHeuristics is non-empty, section should be included
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket:         "build a thing",
		PlaybookHeuristics: "Use async patterns where applicable",
		Cycle:              1,
	})
	if !strings.Contains(prompt, "## Playbook Heuristics") {
		t.Fatal("buildPlanPrompt must include Playbook Heuristics section when field is non-empty")
	}
	if !strings.Contains(prompt, "Use async patterns where applicable") {
		t.Fatal("buildPlanPrompt must include PlaybookHeuristics content")
	}

	// When PlaybookHeuristics is empty, section should be omitted
	prompt = buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if strings.Contains(prompt, "## Playbook Heuristics") {
		t.Fatal("buildPlanPrompt must omit Playbook Heuristics section when field is empty")
	}
}

func TestBuildPlanPrompt_RefinementGuidance(t *testing.T) {
	// When RefinementGuidance is non-empty, section should be included
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket:         "build a thing",
		RefinementGuidance: "Focus on error handling improvements",
		Cycle:              1,
	})
	if !strings.Contains(prompt, "## Refinement Guidance") {
		t.Fatal("buildPlanPrompt must include Refinement Guidance section when field is non-empty")
	}
	if !strings.Contains(prompt, "Focus on error handling improvements") {
		t.Fatal("buildPlanPrompt must include RefinementGuidance content")
	}

	// When RefinementGuidance is empty, section should be omitted
	prompt = buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if strings.Contains(prompt, "## Refinement Guidance") {
		t.Fatal("buildPlanPrompt must omit Refinement Guidance section when field is empty")
	}
}

func TestBuildPlanPrompt_ContainsArchitectureDecisions(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if !strings.Contains(prompt, "Architecture Decisions") {
		t.Fatal("buildPlanPrompt must contain 'Architecture Decisions' section with think-step instructions")
	}
	archIdx := strings.Index(prompt, "## Architecture Decisions")
	outputIdx := strings.Index(prompt, "## Output Format")
	if archIdx == -1 || outputIdx == -1 || archIdx > outputIdx {
		t.Fatal("buildPlanPrompt: Architecture Decisions section must appear before Output Format section")
	}
}

func TestBuildPlanPrompt_ContainsArchitectureDecisionsInOutputFormat(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if !strings.Contains(prompt, "architecture_decisions") {
		t.Fatal("buildPlanPrompt output format must include 'architecture_decisions' field")
	}
	if !strings.Contains(prompt, "cross-cutting conventions this spec introduces") {
		t.Fatal("buildPlanPrompt output format must describe architecture_decisions as 'cross-cutting conventions this spec introduces'")
	}
}

func TestBuildPlanPrompt_ContainsEmptyArrayGuidance(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{
		SpecPacket: "build a thing",
		Cycle:      1,
	})
	if !strings.Contains(prompt, "If none exist, leave the array empty") {
		t.Fatal("buildPlanPrompt must contain guidance that if none exist, leave the array empty")
	}
}

func TestBuildFixPlanPrompt_ContainsArchitectureConventions_WhenConstraintsNonEmpty(t *testing.T) {
	constraints := []string{
		"Path semantics: always relative to project root",
		"Nil-field normalization: exported types use NormalizeNilFields()",
	}
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan:            Plan{SpecID: "s1", Cycle: 1},
		Failures:                []string{"lint error"},
		Cycle:                   2,
		ArchitectureConstraints: constraints,
	})
	if !strings.Contains(prompt, "Architecture Conventions") {
		t.Fatal("buildFixPlanPrompt must contain 'Architecture Conventions' section when ArchitectureConstraints is non-empty")
	}
	if !strings.Contains(prompt, "established in prior cycles") {
		t.Fatal("buildFixPlanPrompt must contain 'established in prior cycles' text")
	}
	if !strings.Contains(prompt, "architecture_drift finding") {
		t.Fatal("buildFixPlanPrompt must contain 'architecture_drift finding' text")
	}
	for _, constraint := range constraints {
		if !strings.Contains(prompt, constraint) {
			t.Fatalf("buildFixPlanPrompt must include constraint '%s' in output", constraint)
		}
	}
	archIdx := strings.Index(prompt, "## Architecture Conventions")
	heuristicsIdx := strings.Index(prompt, "## Playbook Heuristics")
	if heuristicsIdx != -1 && archIdx != -1 && archIdx > heuristicsIdx {
		t.Fatal("buildFixPlanPrompt: Architecture Conventions section must appear before Playbook Heuristics section")
	}
}

func TestBuildFixPlanPrompt_NoArchitectureConventions_WhenConstraintsEmpty(t *testing.T) {
	// Test with nil ArchitectureConstraints
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"lint error"},
		Cycle:        2,
	})
	if strings.Contains(prompt, "Architecture Conventions") {
		t.Fatal("buildFixPlanPrompt must NOT contain 'Architecture Conventions' when ArchitectureConstraints is nil")
	}

	// Test with empty ArchitectureConstraints slice
	prompt = buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan:            Plan{SpecID: "s1", Cycle: 1},
		Failures:                []string{"lint error"},
		Cycle:                   2,
		ArchitectureConstraints: []string{},
	})
	if strings.Contains(prompt, "Architecture Conventions") {
		t.Fatal("buildFixPlanPrompt must NOT contain 'Architecture Conventions' when ArchitectureConstraints is empty")
	}
}

func TestBuildFixPlanPrompt_ContainsArchitectureDecisionsInOutputFormat(t *testing.T) {
	prompt := buildFixPlanPrompt(FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		Failures:     []string{"lint error"},
		Cycle:        2,
	})
	if !strings.Contains(prompt, "architecture_decisions") {
		t.Fatal("buildFixPlanPrompt output format must include 'architecture_decisions' field")
	}
	if !strings.Contains(prompt, "new cross-cutting conventions resolved in this cycle") {
		t.Fatal("buildFixPlanPrompt output format must describe architecture_decisions as 'new cross-cutting conventions resolved in this cycle'")
	}
	if !strings.Contains(prompt, "architecture_drift finding") {
		t.Fatal("buildFixPlanPrompt output format must reference 'architecture_drift finding' for architecture_decisions")
	}
}
