package prompt

import (
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

// Context holds all data available to prompt templates
type Context struct {
	// Current task
	Bead       *bead.Bead
	ParentBead *bead.Bead
	Spec       string // Content of spec file if referenced
	SpecName   string // Name of spec file

	// Project context
	ClaudeMD string // Content of project's CLAUDE.md
	Rules    string // Content of RULES.md
	WorkDir  string // Working directory

	// Learnings
	ConfirmedLearnings []learnings.Learning
	RecentLearnings    []learnings.Learning

	// Validation history
	RecentValidationFailures []string // Summaries of recent validation failures from current run

	// Coverage tracking
	CoverageState   string // Summary of current criterion coverage state for TDD build prompts
	TargetCriterion string // Specific uncovered criterion to focus in the next TDD cycle

	// Scoped test command for build phase self-checks (e.g. "go test ./internal/runner/... ./internal/config/...")
	// When non-empty, templates should use this instead of the generic "./..." form.
	ScopedTestCommand string

	// SiblingTouchedPackages lists package paths touched by completed sibling beads.
	SiblingTouchedPackages []string

	// Iteration info
	Iteration      int
	Model          string
	IsRetry        bool
	PrevFailure    string // Output from previous failed attempt
	FailureContext string // Suggestion from failure analysis
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
func (c *Context) normalizeNilFields() {
	if c == nil {
		return
	}
	if c.ConfirmedLearnings == nil {
		c.ConfirmedLearnings = []learnings.Learning{}
	}
	if c.RecentLearnings == nil {
		c.RecentLearnings = []learnings.Learning{}
	}
	if c.RecentValidationFailures == nil {
		c.RecentValidationFailures = []string{}
	}
	if c.SiblingTouchedPackages == nil {
		c.SiblingTouchedPackages = []string{}
	}
}

// AnalyzeContext holds data for failure analysis prompt template
type AnalyzeContext struct {
	BeadID          string
	BeadTitle       string
	BeadDescription string
	FailureOutput   string
}

// LearnContext holds data for success learning extraction prompt template
type LearnContext struct {
	BeadID          string
	BeadTitle       string
	BeadDescription string
	Summary         string // Brief summary of work done
}

// SuccessLearning represents the result of extracting a learning from success
type SuccessLearning struct {
	Learning *string `json:"learning"` // nil if no learning extracted
	Category string  `json:"category"`
}

// DecomposeContext holds data for task decomposition prompt template
type DecomposeContext struct {
	Bead       *bead.Bead
	ParentBead *bead.Bead
	ATDDActive bool // Whether ATDD methodology is active for this bead
}

// ScopeContext holds data for scope estimation prompt template
type ScopeContext struct {
	Bead       *bead.Bead
	ParentBead *bead.Bead
}

// PrecheckContext holds data for precheck prompt template
type PrecheckContext struct {
	Bead       *bead.Bead
	ParentBead *bead.Bead
}

// SpecAcceptanceContext holds data for spec acceptance prompt template
type SpecAcceptanceContext struct {
	Spec  string
	Rules string
}

// SpecGateContext holds data for spec gate prompt template
type SpecGateContext struct {
	SpecCriteria       string
	FailureOutput      string
	TestOutput         string // Output from running acceptance tests
	CumulativeDiff     string // Cumulative git diff for the spec
	AcceptanceCriteria string // Acceptance criteria from the spec file
}

// DiagnosticContext holds data for the ATDD pass-before-build diagnostic template.
type DiagnosticContext struct {
	BeadTitle          string
	BeadDescription    string
	AcceptanceCriteria string
	TestDiff           string
	TestOutput         string
}

// TestFixContext holds data for test-fix prompt template
type TestFixContext struct {
	ClaudeMD          string
	Rules             string
	TestCommand       string
	TestFailureOutput string
}

// CoverageValidationContext holds data for test coverage validation prompt template.
type CoverageValidationContext struct {
	TestCode        string
	CriterionNumber int
	CriterionText   string
}

// TDDRedContext holds data for TDD red-phase prompt template.
type TDDRedContext struct {
	BeadID            string
	BeadTitle         string
	SpecExcerpt       string
	TestFileContents  map[string]string
	APISurface        string
	CycleSummary      string
	Rules             string
	WorkDir           string
	ScopedTestCommand string
	IsRetry           bool
	FailureContext    string
	PrevFailure       string
}

// TDDGreenContext holds data for TDD green-phase prompt template.
type TDDGreenContext struct {
	BeadID            string
	BeadTitle         string
	FailingTest       string
	TestFailureOutput string
	ImplFileContents  map[string]string
	Rules             string
	WorkDir           string
	ScopedTestCommand string
	IsRetry           bool
	FailureContext    string
	PrevFailure       string
}

// ScopeEstimate represents the result of scope estimation
type ScopeEstimate struct {
	Complexity                   string   `json:"complexity"`
	EstimatedIterations          int      `json:"estimated_iterations"`
	Rationale                    string   `json:"rationale"`
	CanCompleteInSingleIteration bool     `json:"can_complete_in_single_iteration"`
	Blockers                     []string `json:"blockers"`
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
func (s *ScopeEstimate) normalizeNilFields() {
	if s == nil {
		return
	}
	if s.Blockers == nil {
		s.Blockers = []string{}
	}
}

// ReviewContext holds data for light review prompt template
type ReviewContext struct {
	Bead               *bead.Bead
	ParentBead         *bead.Bead
	Spec               string
	Diff               string
	ClaudeMD           string
	Rules              string
	Model              string
	ValidationCommands []string
}

// CompletedBeadSummary holds summary information about a completed bead for thorough reviews
type CompletedBeadSummary struct {
	ID          string
	Title       string
	Description string
}

// ThoroughReviewContext holds data for thorough review prompt template
type ThoroughReviewContext struct {
	Diff           string
	CompletedBeads []CompletedBeadSummary
	ClaudeMD       string
	Rules          string
	Model          string
}
