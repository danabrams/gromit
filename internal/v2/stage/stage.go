package stage

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
)

// Stage defines a single beam within the run loop.
type Stage interface {
	Name() string
	Run(context.Context, *StageRequest) (*StageResult, error)
}

// StageRequest captures metadata the loop passes to each stage during execution.
type StageRequest struct {
	Bead         BeadInfo
	Model        string
	Tier         string // routing tier (e.g. "low", "medium", "high")
	Provider     llmtypes.LLMProvider
	Iteration    int
	Config       *config.Config
	Worktree     string
	Remediation              bool
	GapAnalysis              string
	CompletedBeadTitles      []string // titles of beads already closed
	FailedAcceptanceCriteria []string // specific criteria that failed acceptance
	RetryContext *RetryContext
	Telemetry    *LLMCostSummary
}

// StageResult reports the outcome of a stage invocation.
type StageResult struct {
	Decision  Decision
	Artifacts any
	Events    []event.TypedEvent
}

// Request is the legacy stage request type.
type Request = StageRequest

// Result is the legacy stage result type.
type Result = StageResult

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
	if s, ok := decisionStrings[d]; ok {
		return s
	}
	return "unknown"
}

var decisionStrings = map[Decision]string{
	DecisionProceed: "proceed",
	DecisionSkip:    "skip",
	DecisionBlock:   "block",
	DecisionFail:    "fail",
}

// BeadInfo captures identifying metadata about the bead under execution.
type BeadInfo struct {
	ID           string
	Title        string
	Description  string
	Priority     string
	Labels       []string
	Dependencies []string
}

// RetryContext carries prior failure information for retry attempts.
type RetryContext struct {
	Attempt         int
	PriorFailures   []string
	EscalationLevel int

	// Budget fields populated by Andon integration (not yet implemented)
	TimeBudgetRemaining *float64
	CostBudgetRemaining *float64
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
