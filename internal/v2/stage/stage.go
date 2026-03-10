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
	Remediation  bool
	GapAnalysis  string
	Findings     []Finding
	RetryContext *RetryContext
	Telemetry    *LLMCostSummary
	SpecFindings []SpecFinding
}

// StageResult reports the outcome of a stage invocation.
type StageResult struct {
	Decision  Decision
	Artifacts any
	Events    []event.TypedEvent
}

// Finding captures structured issues discovered during a stage run.
type Finding struct {
	Title         string
	Severity      Severity
	Category      Category
	Scope         Scope
	Description   string
	AffectedFiles []string
}

// NormalizeNilFields ensures slice fields are non-nil.
func (f *Finding) NormalizeNilFields() {
	if f == nil {
		return
	}
	if f.AffectedFiles == nil {
		f.AffectedFiles = []string{}
	}
}

// Severity describes the signal-level urgency of a finding.
type Severity string

const (
	SeverityCritical   Severity = "critical"
	SeverityWarning    Severity = "warning"
	SeveritySuggestion Severity = "suggestion"
)

// Category identifies the type of work a finding represents.
type Category string

const (
	CategoryBug          Category = "bug"
	CategoryAcceptance   Category = "acceptance"
	CategorySecurity     Category = "security"
	CategoryQuality      Category = "quality"
	CategoryTestGap      Category = "test_gap"
	CategoryArchitecture Category = "architecture"
)

// Scope bounds the surface area impacted by the finding.
type Scope string

const (
	ScopeSpec    Scope = "spec"
	ScopeGeneral Scope = "general"
)

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

// SpecFinding represents a finding about a spec surfaced by a stage.
type SpecFinding struct {
	Title       string
	Description string
	Severity    SpecFindingSeverity
	Category    SpecFindingCategory
	Scope       SpecFindingScope
	AffectedFiles []string
}

// NormalizeNilFields ensures slice fields are non-nil.
func (f *SpecFinding) NormalizeNilFields() {
	if f == nil {
		return
	}
	if f.AffectedFiles == nil {
		f.AffectedFiles = []string{}
	}
}

// SpecFindingSeverity classifies the urgency of a finding.
type SpecFindingSeverity string

const (
	SpecFindingSeverityCritical    SpecFindingSeverity = "critical"
	SpecFindingSeverityHigh        SpecFindingSeverity = "high"
	SpecFindingSeverityMedium      SpecFindingSeverity = "medium"
	SpecFindingSeverityLow         SpecFindingSeverity = "low"
	SpecFindingSeverityWarning     SpecFindingSeverity = "warning"
	SpecFindingSeveritySuggestion  SpecFindingSeverity = "suggestion"
)

// SpecFindingCategory groups findings by their domain.
type SpecFindingCategory string

const (
	SpecFindingCategoryAcceptance   SpecFindingCategory = "acceptance"
	SpecFindingCategoryScope        SpecFindingCategory = "scope"
	SpecFindingCategoryQuality      SpecFindingCategory = "quality"
	SpecFindingCategorySafety       SpecFindingCategory = "safety"
	SpecFindingCategorySecurity     SpecFindingCategory = "security"
	SpecFindingCategoryTestGap      SpecFindingCategory = "test_gap"
	SpecFindingCategoryArchitecture SpecFindingCategory = "architecture"
)

// SpecFindingScope identifies the artifact affected by a finding.
type SpecFindingScope string

const (
	SpecFindingScopeSpec  SpecFindingScope = "spec"
	SpecFindingScopeBead  SpecFindingScope = "bead"
	SpecFindingScopeStage SpecFindingScope = "stage"
)

// LLMCostSummary captures telemetry about the LLM invocation that drove the iteration.
type LLMCostSummary struct {
	Model        string
	Duration     time.Duration
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}
