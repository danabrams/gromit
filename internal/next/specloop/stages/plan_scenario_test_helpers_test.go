package stages

import (
	"context"

	"github.com/danabrams/gromit/internal/next/planner"
)

// capturingPlannerAgent satisfies planner.Agent and captures the prompt sent to the agent.
type capturingPlannerAgent struct {
	capturedPrompt string
	response       string
}

func (a *capturingPlannerAgent) Invoke(_ context.Context, prompt string, _ string) (planner.AgentResult, error) {
	a.capturedPrompt = prompt
	return planner.AgentResult{Output: a.response}, nil
}
