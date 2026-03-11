package executor

import (
	"context"
	"time"
)

// Agent is the interface for invoking an AI agent. This mirrors the planner
// package's Agent interface; Go structural typing allows shared implementations.
type Agent interface {
	Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error)
}

// AgentResult holds the output from a single agent invocation.
type AgentResult struct {
	Output    string  `json:"output"`
	TokensIn  int     `json:"tokens_in"`
	TokensOut int     `json:"tokens_out"`
	Cost      float64 `json:"cost"`
	Model     string  `json:"model"`
	Duration  int64   `json:"duration_ms,omitempty"`
}

// Executor orchestrates agent invocations for task execution.
type Executor struct {
	agent Agent
}

// NewExecutor creates an Executor with the given agent.
func NewExecutor(agent Agent) *Executor {
	return &Executor{agent: agent}
}

// RunTaskInput holds the parameters for a single task execution.
type RunTaskInput struct {
	Packet                 string
	WorkDir                string
	ModelTier              string
	MaxTaskDurationSeconds int
}

// RunTaskResult holds the outcome of a single task execution.
type RunTaskResult struct {
	AgentOutput  string
	TokensUsed   int
	Cost         float64
	DurationMs   int64
	FilesChanged []string
	Model        string
	Tier         string
}

// RunTask invokes the agent with the given task packet and returns the result.
func (e *Executor) RunTask(ctx context.Context, input RunTaskInput) (RunTaskResult, error) {
	start := time.Now()

	if input.MaxTaskDurationSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(input.MaxTaskDurationSeconds)*time.Second)
		defer cancel()
	}

	result, err := e.agent.Invoke(ctx, input.Packet, input.ModelTier)
	if err != nil {
		return RunTaskResult{}, err
	}

	return RunTaskResult{
		AgentOutput: result.Output,
		TokensUsed:  result.TokensIn + result.TokensOut,
		Cost:        result.Cost,
		DurationMs:  time.Since(start).Milliseconds(),
		Model:       result.Model,
		Tier:        input.ModelTier,
	}, nil
}
