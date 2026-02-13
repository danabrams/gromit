package runtypes

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
)

// BeadContext holds the shared state for processing a single bead.
// Promoted from the unexported beadContext in runner/process.go.
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

	// Context management
	ParentCtx   context.Context
	BeadTimeout time.Duration
	RunDeadline time.Time

	// Scope estimate (cached from scope gate)
	ScopeEstimate *prompt.ScopeEstimate

	// Package tracking (for learning extraction filtering)
	TouchedPackages []string
}

// IterationResult captures the outcome of one loop iteration.
type IterationResult struct {
	BeadID                string
	BeadTitle             string
	Model                 string
	Success               bool
	Validated             bool
	Duration              time.Duration
	Error                 error
	Escalated             bool
	EscalatedTo           string
	Decomposed            bool
	Output                string
	CostUSD               float64
	InputTokens           int
	OutputTokens          int
	ReviewBrokeValidation bool   // true when review fixes broke previously-passing validation
	AlreadyDone           bool   // true when ATDD detected work was already complete
	ValidationRetried     bool   // true when validation recovery was attempted
	TrivialAutoFixed      bool   // true when auto-fix resolved validation without Claude
	UsageLimited          bool   // true when invocation failed due to usage/rate limit
	ValidationMode        string // "direct" when validation ran via shell commands

	// Diagnostic fields for timeout investigation
	TimeoutType         string // "stall", "bead", "invocation", ""
	TimeToFirstEventMs  int64
	ToolCallCount       int
	StallCount          int
	StallTier           string // "initial" or "active"
	RateLimitHits       int
	RateLimitRecoveryMs int64 // ms to recover from most recent rate limit
}

// SubTask represents a single sub-task from task decomposition.
type SubTask struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	DependsOn          *int     `json:"depends_on"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

// GitDiffFn returns a git diff from a start commit.
type GitDiffFn func(startCommit string) (string, error)

// CmdRunnerFn runs a shell command and returns its output.
type CmdRunnerFn func(ctx context.Context, command string, workDir string) (stdout string, stderr string, exitCode int, err error)

// AutoFixFn runs auto-fix tools (gofmt/goimports) on changed files since a commit.
type AutoFixFn func(startCommit string) error
