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
