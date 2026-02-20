package runtypes

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
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
	RetriesThisModel       int
	TotalRetriesThisBead   int
	MaxRetries             int
	MaxRetriesPerBead      int
	AttemptsThisBead       int
	MaxAttemptsPerBead     int
	CumulativeInputTokens  int
	CumulativeOutputTokens int

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

// IterationResult captures the outcome of one loop iteration.
type IterationResult struct {
	BeadID                  string
	BeadTitle               string
	SpecID                  string
	Model                   string
	Provider                string `json:"provider,omitempty"`
	FailurePhase            string `json:"failure_phase,omitempty"`
	FailureCategory         string `json:"failure_category,omitempty"`
	Success                 bool
	Validated               bool
	Duration                time.Duration
	Error                   error
	Escalated               bool
	EscalatedTo             string
	OriginalTier            string
	ActualTier              string
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
	ValidationDurationMs    int64  // time spent in validation in milliseconds
	CompilationErrors       bool   // true when pre-build compilation check found errors
	HardStopPendingApproval bool   // true when hard-stop action requires explicit human approval

	// Diagnostic fields for timeout investigation
	TimeoutType         string // "stall", "bead", "invocation", ""
	TimeoutPhase        string // phase active when timeout/cancel occurred (e.g. "red", "green", "refactor", "validation")
	TimeToFirstEventMs  int64
	ToolCallCount       int
	StallCount          int
	StallTier           string // "initial" or "active"
	RateLimitHits       int
	RateLimitRecoveryMs int64 // ms to recover from most recent rate limit

	AcceptanceFailureSummary  string // short summary for JSONL
	AcceptanceFailureOutput   string // captured validation output from failed acceptance verification
	AcceptanceFailureArtifact string // path to persisted failure artifact log
	AcceptanceFailureExitCode int    // exit code from failed acceptance validation

	// Reliability and Andon telemetry for iteration logs and status formatting.
	FailureClass     string
	AndonLevel       string
	TrimDecision     string
	AutonomyEligible bool
	AutonomySuccess  bool
	FirstPassSuccess bool
	FilesTouched     int
	TouchedPackages  []string
	MTTRProxyMs      int64
	EscalationClass  string
	RecurrenceCount  int

	// Coverage tracking fields
	CriteriaTotal      int
	CriteriaCovered    int
	CriteriaUntestable int
	UncoveredCriteria  []string

	// TDD phase metrics (nil when TDD methodology is inactive)
	PhaseMetrics []PhaseMetric `json:"phase_metrics,omitempty"`
	// Prompt diagnostics for token attribution and shaping decisions.
	PromptDiagnostics *prompt.PromptDiagnostics `json:"prompt_diagnostics,omitempty"`
}

// PhaseMetric captures per-phase metrics for TDD cycle tracking.
type PhaseMetric struct {
	Phase              string  `json:"phase"`
	CycleNumber        int     `json:"cycle_number"`
	BeadID             string  `json:"bead_id"`
	Model              string  `json:"model"`
	Tier               string  `json:"tier"`
	CostUSD            float64 `json:"cost_usd"`
	InputTokens        int     `json:"input_tokens"`
	OutputTokens       int     `json:"output_tokens"`
	DurationMs         int64   `json:"duration_ms"`
	Success            bool    `json:"success"`
	Escalated          bool    `json:"escalated"`
	EscalatedFrom      string  `json:"escalated_from,omitempty"`
	CriteriaTotal      int     `json:"criteria_total,omitempty"`
	CriteriaCovered    int     `json:"criteria_covered,omitempty"`
	CriteriaUntestable int     `json:"criteria_untestable,omitempty"`
}

// HardStopApprovalState captures explicit approval for hard-stop actions.
type HardStopApprovalState struct {
	Approved   bool
	ApprovedBy string
}

// InvocationResult captures the outcome of a single LLM invocation.
type InvocationResult struct {
	Result         *claude.Result
	Stats          *logger.StreamStats
	StallFired     bool
	ModelName      string
	ProviderName   string
	ProviderResult *provider.Result
	TimeoutType    string // "stall", "invocation", "bead", ""
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

// ArgvRunnerFn runs a program with explicit args and returns its output.
type ArgvRunnerFn func(ctx context.Context, program string, args []string, workDir string) (stdout string, stderr string, exitCode int, err error)

// AutoFixFn runs auto-fix tools (gofmt/goimports) on changed files since a commit.
type AutoFixFn func(startCommit string) error
