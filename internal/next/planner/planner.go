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
	agent       Agent
	plannerTier string
}

// NewPlanner creates a Planner that uses the given agent at the specified tier.
func NewPlanner(agent Agent, plannerTier string) *Planner {
	return &Planner{agent: agent, plannerTier: plannerTier}
}

// CreatePlan invokes the agent to produce a plan from the given request.
func (p *Planner) CreatePlan(ctx context.Context, req PlanRequest) (Plan, error) {
	prompt := buildPlanPrompt(req)
	result, err := p.agent.Invoke(ctx, prompt, p.plannerTier)
	if err != nil {
		return Plan{}, fmt.Errorf("agent invocation failed: %w", err)
	}
	return ParsePlan(result.Output)
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
}

// CreateFixPlan invokes the agent to produce a fix plan addressing failures.
func (p *Planner) CreateFixPlan(ctx context.Context, req FixPlanRequest) (Plan, error) {
	prompt := buildFixPlanPrompt(req)
	result, err := p.agent.Invoke(ctx, prompt, p.plannerTier)
	if err != nil {
		return Plan{}, fmt.Errorf("agent invocation failed: %w", err)
	}
	return ParsePlan(result.Output)
}

// buildFixPlanPrompt constructs the prompt for fix-plan generation.
func buildFixPlanPrompt(req FixPlanRequest) string {
	var b strings.Builder
	b.WriteString("You are a planning agent. Generate a FIX plan as JSON to address failures.\n\n")
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

	if len(req.Failures) > 0 {
		b.WriteString("## Failures to Address\n")
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

	b.WriteString("Respond with a JSON object with kind=\"fix\", parent_cycle, failures_addressed, and tasks.\n")
	b.WriteString("Each task needs: task_id, objective, expected_touched_area, proof_checks, parent_cycle, failures_addressed.\n")
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
	b.WriteString("Each task needs: task_id, objective, expected_touched_area, proof_checks.\n")
	return b.String()
}
