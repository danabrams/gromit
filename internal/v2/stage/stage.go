package stage

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
)

// Stage defines a single beam within the run loop.
type Stage interface {
	Name() string
	Run(context.Context, *Request) (*Result, error)
}

// Request captures metadata the loop passes to each stage during execution.
type Request struct {
	Bead         BeadInfo
	Model        string
	Iteration    int
	Config       *config.Config
	Worktree     string
	Remediation  bool
	RetryContext *RetryContext
	Telemetry    *LLMCostSummary
}

// Result reports the outcome of a stage invocation.
type Result struct {
	Decision  Decision
	Artifacts any
	Events    []events.Event
}

// DecomposeArtifacts packages the beads created by the decompose stage.
type DecomposeArtifacts struct {
	Beads []*bead.Bead
}

// Decision describes how the loop should proceed after a stage runs.
type Decision int

const (
	DecisionProceed Decision = iota
	DecisionSkip
	DecisionBlock
	DecisionFail
)

func (d Decision) String() string {
	switch d {
	case DecisionProceed:
		return "proceed"
	case DecisionSkip:
		return "skip"
	case DecisionBlock:
		return "block"
	case DecisionFail:
		return "fail"
	default:
		return "unknown"
	}
}

// BeadInfo captures identifying metadata about the bead under execution.
type BeadInfo struct {
	ID     string
	Labels []string
}

// RetryContext carries prior failure information for retry attempts.
type RetryContext struct {
	Attempt         int
	PriorFailures   []string
	EscalationLevel int
}

// RetryConfig defines retry behavior for a stage.
type RetryConfig struct {
	MaxRetries int
	RetryWith  []string
}

// LLMCostSummary captures telemetry about the LLM invocation that drove the iteration.
type LLMCostSummary struct {
	Model        string
	Duration     time.Duration
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}
