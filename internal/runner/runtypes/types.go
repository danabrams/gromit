package runtypes

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/andon"
)

// BeadContext holds the shared state for processing a single bead.
// Used by the runner facade and will be shared with sub-packages during extraction.
type BeadContext struct {
	Bead          *bead.Bead
	Parent        *bead.Bead
	Result        *IterationResult
	Model         string // concrete model name for display/logging
	Tier          string // abstract tier (high/medium/low) for router selection
	BuildProvider string // name of provider that performed the build
	PromptCtx     *prompt.Context
	BuildPrompt   string
	StartCommit   string
	Iteration     int

	// Retry tracking
	RetriesThisModel     int
	TotalRetriesThisBead int
	MaxRetries           int
	MaxRetriesPerBead    int
	AttemptsThisBead     int
	MaxAttemptsPerBead   int

	// Context management
	ParentCtx     context.Context
	BeadTimeout   time.Duration
	RunDeadline   time.Time
	BeadStartTime time.Time

	// Timeout policy tracking
	TimeoutEscalationsThisBead int
	StallRetryWithoutToolUsed  bool

	// Scope estimate (cached from scope gate)
	ScopeEstimate *prompt.ScopeEstimate

	// Package tracking (for learning extraction filtering)
	TouchedPackages []string

	// Hard-stop guardrail approval state for dangerous actions.
	HardStopApproval HardStopApprovalState
}

const (
	TimeoutTypePhase = "phase_timeout"
)

// IterationResult captures the outcome of one loop iteration.
type IterationResult struct {
	BeadID                  string
	BeadTitle               string
	Model                   string
	Success                 bool
	Validated               bool
	Duration                time.Duration
	Error                   error
	Escalated               bool
	EscalatedTo             string
	Decomposed              bool
	Output                  string
	CostUSD                 float64
	InputTokens             int
	OutputTokens            int
	ReviewBrokeValidation   bool   // true when review fixes broke previously-passing validation
	AlreadyDone             bool   // true when ATDD detected work was already complete
	ValidationRetried       bool   // true when validation recovery was attempted
	TrivialAutoFixed        bool   // true when auto-fix resolved validation without Claude
	UsageLimited            bool   // true when invocation failed due to usage/rate limit
	ValidationMode          string // "direct" when validation ran via shell commands
	CompilationErrors       bool   // true when pre-build compilation check found errors
	HardStopPendingApproval bool   // true when hard-stop action requires explicit human approval

	// Diagnostic fields for timeout investigation
	TimeoutType         string // "stall", "bead", "invocation", ""
	TimeToFirstEventMs  int64
	ToolCallCount       int
	StallCount          int
	StallTier           string // "initial" or "active"
	RateLimitHits       int
	RateLimitRecoveryMs int64 // ms to recover from most recent rate limit

	AcceptanceFailureSummary  string // short summary for JSONL
	AcceptanceFailureOutput   string // captured validation output from failed acceptance verification
	AcceptanceFailureArtifact string // path to persisted failure artifact log

	FailureClass     andon.FailureClass
	AndonLevel       andon.AndonLevel
	TrimDecision     string
	AutonomyEligible bool
	AutonomySuccess  bool
	FirstPassSuccess bool
	MTTRProxyMs      int64
	EscalationClass  andon.FailureClass
	RecurrenceCount  int
}

// HardStopApprovalState captures explicit approval for hard-stop actions.
type HardStopApprovalState struct {
	Approved   bool
	ApprovedBy string
}

// SubTask represents a single sub-task from task decomposition.
type SubTask struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	DependsOn          *int     `json:"depends_on"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

// NormalizeNilFields ensures nil slices are replaced with empty slices.
func (s *SubTask) NormalizeNilFields() {
	if s == nil {
		return
	}
	if s.AcceptanceCriteria == nil {
		s.AcceptanceCriteria = []string{}
	}
}

// GitDiffFn returns a git diff from a start commit.
type GitDiffFn func(startCommit string) (string, error)

// CmdRunnerFn runs a shell command and returns its output.
type CmdRunnerFn func(ctx context.Context, command string, workDir string) (stdout string, stderr string, exitCode int, err error)

// AutoFixFn runs auto-fix tools (gofmt/goimports) on changed files since a commit.
type AutoFixFn func(startCommit string) error
