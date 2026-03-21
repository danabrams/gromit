package planner

import (
	"context"
	"fmt"
	"strings"
)

// AgentResult aligns with internal/provider.Result pattern.
type AgentResult struct {
	Output    string  `json:"output"`
	TokensIn  int     `json:"tokens_in"`
	TokensOut int     `json:"tokens_out"`
	Cost      float64 `json:"cost"`
	Model     string  `json:"model"`
	Duration  int64   `json:"duration_ms,omitempty"`
}

// Agent is the interface for invoking an LLM agent.
type Agent interface {
	Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error)
}

// PlanRequest contains everything needed to generate a plan.
type PlanRequest struct {
	SpecPacket     string
	Cycle          int
	CompletedTasks []string
	Failures       []string
	CurrentDiff    string
}

// Planner orchestrates agent-driven plan generation.
type Planner struct {
	agent          Agent
	plannerTier    string
	MaxPlanRetries int // Total attempts = MaxPlanRetries + 1. Default 1 (2 total attempts).
}

// NewPlanner creates a Planner that uses the given agent at the specified tier.
func NewPlanner(agent Agent, plannerTier string) *Planner {
	return &Planner{agent: agent, plannerTier: plannerTier, MaxPlanRetries: 1}
}

// CreatePlan invokes the agent to produce a plan from the given request.
// On parse/validation failure, it retries up to MaxPlanRetries additional times.
func (p *Planner) CreatePlan(ctx context.Context, req PlanRequest) (Plan, error) {
	basePrompt := buildPlanPrompt(req)
	attempts := p.MaxPlanRetries + 1
	var lastErr error
	for i := 0; i < attempts; i++ {
		prompt := basePrompt
		if i > 0 && lastErr != nil {
			prompt = prompt + "\n\nYour previous output was invalid: " + lastErr.Error() + "\nPlease produce valid JSON output."
		}
		result, err := p.agent.Invoke(ctx, prompt, p.plannerTier)
		if err != nil {
			return Plan{}, fmt.Errorf("agent invocation failed: %w", err)
		}
		plan, err := ParsePlan(result.Output)
		if err != nil {
			lastErr = err
			continue
		}
		SanitizeWorktreePaths(&plan)
		return plan, nil
	}
	return Plan{}, fmt.Errorf("plan generation failed after %d attempts: %w", attempts, lastErr)
}

// CompletedTask summarizes a task that was executed in a prior cycle.
type CompletedTask struct {
	TaskID            string   `json:"task_id"`
	Attempts          int      `json:"attempts"`
	FilesChanged      []string `json:"files_changed"`
	ValidationOutcome string   `json:"validation_outcome"`
}

// FixPlanRequest contains everything needed to generate a fix plan.
type FixPlanRequest struct {
	OriginalPlan    Plan            `json:"original_plan"`
	CompletedTasks  []CompletedTask `json:"completed_tasks"`
	Failures        []string        `json:"failures"`
	CurrentDiff     string          `json:"current_diff"`
	Cycle           int             `json:"cycle"`
	PriorMaxTaskID  string          `json:"prior_max_task_id,omitempty"` // e.g. "t-004"; if set, fix plan task IDs must be greater
	SpecConstraints string          `json:"spec_constraints,omitempty"`  // Out-of-Scope + Architectural Constraints from spec.md
	SpecPacket      string          `json:"spec_packet,omitempty"`       // full spec packet for context (requirements, scope, acceptance criteria)
}

// CreateFixPlan invokes the agent to produce a fix plan addressing failures.
// On parse/validation failure, it retries up to MaxPlanRetries additional times.
// If PriorMaxTaskID is set, validates that all fix plan task IDs are greater.
func (p *Planner) CreateFixPlan(ctx context.Context, req FixPlanRequest) (Plan, error) {
	basePrompt := buildFixPlanPrompt(req)
	attempts := p.MaxPlanRetries + 1
	var lastErr error
	for i := 0; i < attempts; i++ {
		prompt := basePrompt
		if i > 0 && lastErr != nil {
			prompt = prompt + "\n\nYour previous output was invalid: " + lastErr.Error() + "\nPlease produce valid JSON output."
		}
		result, err := p.agent.Invoke(ctx, prompt, p.plannerTier)
		if err != nil {
			return Plan{}, fmt.Errorf("agent invocation failed: %w", err)
		}
		plan, err := ParsePlan(result.Output)
		if err != nil {
			lastErr = err
			continue
		}
		SanitizeWorktreePaths(&plan)
		if req.PriorMaxTaskID != "" {
			if err := ValidatePlanWithPrior(plan, req.PriorMaxTaskID); err != nil {
				lastErr = fmt.Errorf("prior-plan validation failed: %w", err)
				continue
			}
		}
		return plan, nil
	}
	return Plan{}, fmt.Errorf("fix plan generation failed after %d attempts: %w", attempts, lastErr)
}

// buildFixPlanPrompt constructs the prompt for fix-plan generation.
func buildFixPlanPrompt(req FixPlanRequest) string {
	var b strings.Builder
	b.WriteString("You are a planning agent. Generate a SURGICAL FIX plan as JSON.\n")
	b.WriteString("Your goal is to create targeted tasks that address ONLY the specific failures and review findings listed below.\n\n")
	b.WriteString(fmt.Sprintf("## Original Plan (Cycle %d, Spec %s)\n", req.OriginalPlan.Cycle, req.OriginalPlan.SpecID))
	b.WriteString(fmt.Sprintf("Tasks in original plan: %d\n\n", len(req.OriginalPlan.Tasks)))

	b.WriteString(fmt.Sprintf("## Fix Cycle: %d\n\n", req.Cycle))

	if req.SpecPacket != "" {
		b.WriteString("## Spec (Original Requirements)\n")
		b.WriteString("Fix tasks MUST comply with these requirements. Do NOT produce changes that violate the In-Scope or Acceptance Criteria.\n\n")
		b.WriteString(req.SpecPacket)
		b.WriteString("\n\n")
	}

	if req.SpecConstraints != "" {
		b.WriteString("## HARD REQUIREMENTS — Spec Constraints\n")
		b.WriteString("These constraints are ABSOLUTE and cannot be overridden by any failure or review finding.\n")
		b.WriteString("'Modify' includes editing, deleting, renaming, or moving a file.\n")
		b.WriteString("CRITICAL: If the ONLY way to fix a failure is by violating a constraint (e.g., modifying a forbidden test file),\n")
		b.WriteString("then do NOT create a fix task for that failure at all. Leave it unfixed.\n")
		b.WriteString("It is BETTER to exhaust cycles and hand off to a human than to violate a spec constraint.\n\n")
		b.WriteString(req.SpecConstraints)
		b.WriteString("\n\n")
	}

	if len(req.CompletedTasks) > 0 {
		b.WriteString("## Completed Tasks\n")
		for _, ct := range req.CompletedTasks {
			b.WriteString(fmt.Sprintf("- %s: %d attempts, outcome=%s, files=%v\n",
				ct.TaskID, ct.Attempts, ct.ValidationOutcome, ct.FilesChanged))
		}
		b.WriteString("\n")
	}

	// Separate persistent failures, review findings, and other failures for clarity.
	var persistentFailures []string
	var reviewFindings []string
	var otherFailures []string
	for _, f := range req.Failures {
		if strings.HasPrefix(f, "persistent-failure:") {
			persistentFailures = append(persistentFailures, f)
			// Also add to otherFailures so it appears in the validation section
			otherFailures = append(otherFailures, f)
		} else if strings.HasPrefix(f, "review:") {
			reviewFindings = append(reviewFindings, f)
		} else {
			otherFailures = append(otherFailures, f)
		}
	}

	if len(persistentFailures) > 0 {
		b.WriteString("## Persistent Failures — Possible Bad Contracts\n")
		b.WriteString("The following failures have repeated across multiple consecutive cycles.\n")
		b.WriteString("This strongly suggests the contract assertion itself is wrong, not the implementation.\n")
		b.WriteString("\n")
		b.WriteString("BEFORE creating any implementation fix task for these failures:\n")
		b.WriteString("1. Find the assertion in scenario-contracts.yaml that corresponds to this failure\n")
		b.WriteString("2. Verify the pattern actually appears in the target file (run grep manually in your head)\n")
		b.WriteString("3. If the pattern looks like a regex (contains .*  \\w+  \\[  etc.) but the file uses\n")
		b.WriteString("   literal Go syntax, the pattern may need to be a literal substring instead\n")
		b.WriteString("4. Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you\n")
		b.WriteString("   have high confidence the implementation is wrong\n")
		b.WriteString("\n")
		b.WriteString("Persistent failures:\n")
		for _, f := range persistentFailures {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(reviewFindings) > 0 {
		b.WriteString("## Review Findings to Fix\n")
		b.WriteString("The following review warnings were raised against the current code. Each fix task you create MUST directly address one or more of these findings.\n")
		for _, f := range reviewFindings {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(otherFailures) > 0 {
		b.WriteString("## Validation Failures to Fix\n")
		b.WriteString("The following validation failures occurred. Each fix task you create MUST directly address one or more of these failures.\n")
		for _, f := range otherFailures {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if req.CurrentDiff != "" {
		b.WriteString("## Current Diff\n```\n")
		b.WriteString(req.CurrentDiff)
		b.WriteString("\n```\n\n")
	}

	b.WriteString("## Instructions\n")
	b.WriteString("- Do NOT replan or recreate original tasks — do not re-include work that already completed successfully.\n")
	b.WriteString("- Create ONLY surgical fix tasks that address the specific failures and review findings listed above.\n")
	b.WriteString("- Each fix task objective must reference which failure(s) or review finding(s) it addresses.\n")
	b.WriteString("- Only touch files that are relevant to the listed issues.\n")
	b.WriteString("- NEVER create tasks that touch files prohibited by spec constraints (e.g., existing test files if the spec says not to modify them).\n")
	b.WriteString("- If a failure can ONLY be fixed by modifying a prohibited file, skip that failure entirely — do not create a task for it.\n\n")

	b.WriteString("## Task Granularity\n")
	b.WriteString("Each task should touch at most 3-4 files in expected_touched_area. If a fix requires more files, split it into multiple tasks.\n")
	b.WriteString("Scenario-driven work (e.g. updating multiple test files or fixing multiple independent scenarios) MUST be decomposed as one task per scenario or test file, not bundled into a single aggregate task.\n\n")
	b.WriteString("## Output Format\n")
	b.WriteString("Respond with a JSON object:\n")
	b.WriteString("- kind: must be \"fix\"\n")
	b.WriteString("- spec_id, cycle, parent_cycle, failures_addressed (array of strings from the failures above)\n")
	b.WriteString("- tasks: array where each task has:\n")
	b.WriteString("  - task_id: use \"t-NNN\" format")
	if req.PriorMaxTaskID != "" {
		b.WriteString(fmt.Sprintf(" (IDs must be greater than %s)", req.PriorMaxTaskID))
	}
	b.WriteString("\n")
	b.WriteString("  - objective: string describing the surgical fix\n")
	b.WriteString("  - expected_touched_area: array of strings (file paths or directories)\n")
	b.WriteString("  - proof_checks: array of EXECUTABLE SHELL COMMANDS to verify the fix. Must be runnable via `sh -c`. No prose descriptions.\n")
	b.WriteString("    **CRITICAL: All file paths in proof_checks and expected_touched_area must be relative to the project root (e.g. `internal/pkg/foo.go`). NEVER use `.gromit-next/worktrees/...` prefixes.**\n")
	b.WriteString("    **Proof check quality rules** (same as original plan):\n")
	b.WriteString("    - Every task modifying `.go` files MUST include `go build ./...`.\n")
	b.WriteString("    - Prefer `go test -run TestX -v ./path/to/pkg/` over `grep -q 'func X'` — verify behavior, not presence.\n")
	b.WriteString("    - For operation ordering, use awk to compare line numbers (e.g. `awk '/stepA/{ a=NR } /stepB/{ b=NR } END{ if(a>=b||a==0||b==0) exit 1 }' file.go`).\n")
	b.WriteString("    - For config flow, verify the config field is READ where it is used (e.g. `grep -q 'cfg\\.Field' consumer.go`).\n")
	b.WriteString("    - For integration wiring, verify the function CALL, not just the import (e.g. `grep -q 'svc\\.Run(' caller.go`).\n")
	b.WriteString("    - For `*_test.go` in `expected_touched_area`, include a proof check verifying new test content exists. Do NOT rely solely on `go test ./...`.\n")
	b.WriteString("  - parent_cycle: integer (the cycle being fixed)\n")
	b.WriteString("  - failures_addressed: array of strings (subset of failures this task fixes)\n")
	b.WriteString("  - fixes: string (optional) the task_id of the failed task this fix task addresses (e.g. \"t-001\"). Include this when your fix directly addresses a specific prior task's failure.\n")
	return b.String()
}

// buildPlanPrompt constructs the prompt for plan generation.
func buildPlanPrompt(req PlanRequest) string {
	var b strings.Builder
	b.WriteString("You are a planning agent. Generate an execution plan as JSON.\n\n")
	b.WriteString("## Spec Packet\n")
	b.WriteString(req.SpecPacket)
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("## Cycle: %d\n\n", req.Cycle))

	if len(req.CompletedTasks) > 0 {
		b.WriteString("## Completed Tasks\n")
		for _, t := range req.CompletedTasks {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(req.Failures) > 0 {
		b.WriteString("## Failures\n")
		for _, f := range req.Failures {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if req.CurrentDiff != "" {
		b.WriteString("## Current Diff\n```\n")
		b.WriteString(req.CurrentDiff)
		b.WriteString("\n```\n\n")
	}

	b.WriteString("## Task Granularity\n")
	b.WriteString("Each task should touch at most 3-4 files in expected_touched_area. If a logical unit requires more files, split it into multiple tasks.\n")
	b.WriteString("Scenario-driven work (e.g. updating multiple test files or implementing multiple independent scenarios) MUST be decomposed as one task per scenario or test file, not bundled into a single aggregate task. Bundling defers failures to later fix cycles.\n\n")
	b.WriteString("## Output Format\n")
	b.WriteString("Respond with a JSON object containing spec_id, cycle, kind, and tasks array.\n")
	b.WriteString("kind must be \"original\" (not \"implementation\" or any other value).\n")
	b.WriteString("task_id must use the format \"t-NNN\" (e.g. \"t-001\", \"t-002\").\n")
	b.WriteString("expected_touched_area must be an array of strings (e.g. [\"calc/calc.go\"]).\n")
	b.WriteString("Each task needs: task_id, objective, expected_touched_area, proof_checks.\n")
	b.WriteString("proof_checks must be EXECUTABLE SHELL COMMANDS only (run via `sh -c`). No prose descriptions — only runnable commands.\n")
	b.WriteString("**CRITICAL: All file paths in proof_checks and expected_touched_area must be relative to the project root (e.g. `internal/pkg/foo.go`). NEVER use `.gromit-next/worktrees/...` prefixes.**\n\n")

	b.WriteString("## Proof Check Quality Guidelines\n")
	b.WriteString("Proof checks must verify BEHAVIOR, not just PRESENCE of code. Follow these rules in priority order:\n\n")
	b.WriteString("1. **Compilation is mandatory**: Every task that creates or modifies `.go` files MUST include `go build ./...` as a proof check. Code that doesn't compile is never acceptable.\n\n")
	b.WriteString("2. **Behavioral over presence**: Prefer `go test -run TestX -v ./path/to/pkg/` over `grep -q 'func X'`. A grep proves a function name exists; a test proves it works. Use grep only when no test exists yet AND you are not creating one in this task.\n\n")
	b.WriteString("3. **Order and sequence verification**: When the spec requires operations in a specific order (e.g., validate before save, lock before read), verify order with a command like: `awk '/validateInput/{ v=NR } /saveRecord/{ s=NR } END{ if(v>=s||v==0||s==0) exit 1 }' path/to/file.go` — this proves validateInput appears before saveRecord.\n\n")
	b.WriteString("4. **Config flow verification**: When a config value should influence behavior, verify it is READ in the function that uses it, not just defined somewhere. Example: `grep -q 'cfg\\.MaxRetries' internal/runner/retry.go` proves the retry function actually reads the config, not just that MaxRetries exists in a struct.\n\n")
	b.WriteString("5. **Integration wiring verification**: For tasks that wire components together, verify the actual function CALL exists, not just the import. Example: `grep -q 'validator\\.Validate(' internal/runner/pipeline.go` proves the pipeline calls the validator, not just that it imports the validator package.\n\n")
	b.WriteString("6. **Test file proof checks**: For each `*_test.go` file listed in `expected_touched_area`, include at least one proof check that verifies new content exists in that test file — for example `grep -q 'TestFoo_Bar' path/to/foo_test.go`. Do NOT rely solely on `go test ./...`; it passes even when no new tests were added.\n")
	return b.String()
}
