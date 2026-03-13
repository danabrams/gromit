package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/validator"
)

// Bundler assembles evidence files into a directory that documents
// what happened during a spec execution run.
type Bundler struct {
	dir string
}

// NewBundler creates a Bundler that writes evidence files to dir.
func NewBundler(dir string) *Bundler {
	return &Bundler{dir: dir}
}

// Init creates the evidence directory.
func (b *Bundler) Init() error {
	return os.MkdirAll(b.dir, 0o755)
}

// WriteTaskResults writes task results to task-results.json.
func (b *Bundler) WriteTaskResults(tasks []runstore.Task) error {
	return b.writeJSON("task-results.json", tasks)
}

// WriteValidation writes final validation results to validation.json.
func (b *Bundler) WriteValidation(result validator.FinalResult) error {
	return b.writeJSON("validation.json", result)
}

// InvocationRecord captures a single LLM invocation's metadata.
type InvocationRecord struct {
	Phase      string  `json:"phase"`
	Tier       string  `json:"tier"`
	Model      string  `json:"model"`
	TokensIn   int     `json:"tokens_in"`
	TokensOut  int     `json:"tokens_out"`
	DurationMs int64   `json:"duration_ms"`
	CostUSD    float64 `json:"cost_usd"`
	Success    bool    `json:"success"`
}

// Metrics captures aggregate run metrics.
type Metrics struct {
	TotalTokens       int                `json:"total_tokens"`
	TotalCostUSD      float64            `json:"total_cost_usd"`
	TotalTasks        int                `json:"total_tasks"`
	PassedTasks       int                `json:"passed_tasks"`
	FailedTasks       int                `json:"failed_tasks"`
	DurationMs        int64              `json:"duration_ms"`
	Cycles            int                `json:"cycles"`
	TotalRetries      int                `json:"total_retries"`
	TotalReplans      int                `json:"total_replans"`
	HumanIntervention bool               `json:"human_intervention"`
	Invocations       []InvocationRecord `json:"invocations"`
}

// NormalizeNilFields maps nil slices to empty values for consistent JSON serialization.
func (m *Metrics) NormalizeNilFields() {
	if m.Invocations == nil {
		m.Invocations = []InvocationRecord{}
	}
}

// WriteMetrics writes aggregate metrics to metrics.json.
func (b *Bundler) WriteMetrics(m Metrics) error {
	m.NormalizeNilFields()
	return b.writeJSON("metrics.json", m)
}

// WriteDiffSummary writes the diff summary text to diff-summary.md.
func (b *Bundler) WriteDiffSummary(summary string) error {
	return os.WriteFile(filepath.Join(b.dir, "diff-summary.md"), []byte(summary), 0o644)
}

// SummaryInput provides data for generating the summary markdown.
type SummaryInput struct {
	SpecID    string `json:"spec_id"`
	Status    string `json:"status"`
	TaskCount int    `json:"task_count"`
	PassCount int    `json:"pass_count"`
	Cycles    int    `json:"cycles"`
}

// WriteSummary writes a summary markdown file.
func (b *Bundler) WriteSummary(s SummaryInput) error {
	md := fmt.Sprintf("# Execution Summary\n\n"+
		"- **Spec ID:** %s\n"+
		"- **Status:** %s\n"+
		"- **Tasks:** %d/%d passed\n"+
		"- **Cycles:** %d\n",
		s.SpecID, s.Status, s.PassCount, s.TaskCount, s.Cycles)
	return os.WriteFile(filepath.Join(b.dir, "summary.md"), []byte(md), 0o644)
}

// CycleRecord captures per-cycle task statistics.
type CycleRecord struct {
	Cycle     int `json:"cycle"`
	TaskCount int `json:"task_count"`
	PassCount int `json:"pass_count"`
}

// ReviewFindingSummary captures a per-facet summary for the review decision sheet.
type ReviewFindingSummary struct {
	Facet      string `json:"facet"`
	Count      int    `json:"count"`
	Severities string `json:"severities"`
}

// AcceptanceCriterionSummary captures a per-criterion acceptance result for the review decision sheet.
type AcceptanceCriterionSummary struct {
	Criterion string `json:"criterion"`
	Status    string `json:"status"`
	Rationale string `json:"rationale"`
}

// ReviewInput provides data for generating the review decision sheet.
type ReviewInput struct {
	TerminalState      string                       `json:"terminal_state"`
	BlockerSummary     string                       `json:"blocker_summary,omitempty"`
	WhatChanged        string                       `json:"what_changed"`
	CycleHistory       []CycleRecord                `json:"cycle_history"`
	ValidationResults  string                       `json:"validation_results"`
	KnownRisks         []string                     `json:"known_risks"`
	RecommendedAction  string                       `json:"recommended_action"`
	ReviewFindings     []ReviewFindingSummary        `json:"review_findings,omitempty"`
	AcceptanceCriteria []AcceptanceCriterionSummary  `json:"acceptance_criteria,omitempty"`
}

// NormalizeNilFields maps nil slices to empty values for consistent JSON serialization.
func (r *ReviewInput) NormalizeNilFields() {
	if r.CycleHistory == nil {
		r.CycleHistory = []CycleRecord{}
	}
	if r.KnownRisks == nil {
		r.KnownRisks = []string{}
	}
	if r.ReviewFindings == nil {
		r.ReviewFindings = []ReviewFindingSummary{}
	}
	if r.AcceptanceCriteria == nil {
		r.AcceptanceCriteria = []AcceptanceCriterionSummary{}
	}
}

// WriteReview writes a review decision sheet to review.md.
func (b *Bundler) WriteReview(r ReviewInput) error {
	md := fmt.Sprintf("# Review Decision Sheet\n\n"+
		"## Terminal State\n\n%s\n\n",
		r.TerminalState)

	if r.BlockerSummary != "" {
		md += fmt.Sprintf("## Blocker\n\n%s\n\n", r.BlockerSummary)
	}

	md += fmt.Sprintf("## What Changed\n\n%s\n\n"+
		"## Cycle History\n\n"+
		"| Cycle | Tasks | Passed |\n"+
		"|-------|-------|--------|\n",
		r.WhatChanged)

	for _, c := range r.CycleHistory {
		md += fmt.Sprintf("| %d | %d | %d |\n", c.Cycle, c.TaskCount, c.PassCount)
	}

	md += fmt.Sprintf("\n## Validation Results\n\n%s\n\n", r.ValidationResults)

	md += "## Known Risks\n\n"
	for _, risk := range r.KnownRisks {
		md += fmt.Sprintf("- %s\n", risk)
	}

	md += "\n## Review Findings\n\n"
	if len(r.ReviewFindings) > 0 {
		md += "| Facet | Count | Severities |\n"
		md += "|-------|-------|------------|\n"
		for _, f := range r.ReviewFindings {
			md += fmt.Sprintf("| %s | %d | %s |\n", f.Facet, f.Count, f.Severities)
		}
	} else {
		md += "No review findings.\n"
	}

	md += "\n## Acceptance Criteria\n\n"
	if len(r.AcceptanceCriteria) > 0 {
		md += "| Criterion | Status | Rationale |\n"
		md += "|-----------|--------|-----------|\n"
		for _, c := range r.AcceptanceCriteria {
			md += fmt.Sprintf("| %s | %s | %s |\n", c.Criterion, c.Status, c.Rationale)
		}
	} else {
		md += "No acceptance criteria evaluated.\n"
	}

	md += fmt.Sprintf("\n## Recommended Action\n\n%s\n", r.RecommendedAction)

	return os.WriteFile(filepath.Join(b.dir, "review.md"), []byte(md), 0o644)
}

// WriteReviewFindings writes facet-keyed review findings to review.json.
func (b *Bundler) WriteReviewFindings(findings map[string][]review.Finding) error {
	return b.writeJSON("review.json", findings)
}

// WriteAcceptanceResults writes structured acceptance results to acceptance.json.
func (b *Bundler) WriteAcceptanceResults(result acceptor.AcceptanceResult) error {
	result.NormalizeNilFields()
	return b.writeJSON("acceptance.json", result)
}

func (b *Bundler) writeJSON(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.dir, name), data, 0o644)
}
