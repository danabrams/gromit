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
	OriginalPlan   Plan            `json:"original_plan"`
	CompletedTasks []CompletedTask `json:"completed_tasks"`
	Failures       []string        `json:"failures"`
	CurrentDiff    string          `json:"current_diff"`
	Cycle          int             `json:"cycle"`
	PriorMaxTaskID string          `json:"prior_max_task_id,omitempty"` // e.g. "t-004"; if set, fix plan task IDs must be greater
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

	if len(req.CompletedTasks) > 0 {
		b.WriteString("## Completed Tasks\n")
		for _, ct := range req.CompletedTasks {
			b.WriteString(fmt.Sprintf("- %s: %d attempts, outcome=%s, files=%v\n",
				ct.TaskID, ct.Attempts, ct.ValidationOutcome, ct.FilesChanged))
		}
		b.WriteString("\n")
	}

	// Separate review findings from other failures for clarity.
	var reviewFindings []string
	var otherFailures []string
	for _, f := range req.Failures {
		if strings.HasPrefix(f, "review:") {
			reviewFindings = append(reviewFindings, f)
		} else {
			otherFailures = append(otherFailures, f)
		}
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
	b.WriteString("- Only touch files that are relevant to the listed issues.\n\n")

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
	b.WriteString("  - proof_checks: array of strings (commands to verify the fix)\n")
	b.WriteString("  - parent_cycle: integer (the cycle being fixed)\n")
	b.WriteString("  - failures_addressed: array of strings (subset of failures this task fixes)\n")
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

	b.WriteString("Respond with a JSON object containing spec_id, cycle, kind, and tasks array.\n")
	b.WriteString("kind must be \"original\" (not \"implementation\" or any other value).\n")
	b.WriteString("task_id must use the format \"t-NNN\" (e.g. \"t-001\", \"t-002\").\n")
	b.WriteString("expected_touched_area must be an array of strings (e.g. [\"calc/calc.go\"]).\n")
	b.WriteString("Each task needs: task_id, objective, expected_touched_area, proof_checks.\n")
	return b.String()
}
