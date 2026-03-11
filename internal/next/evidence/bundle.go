package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

func (b *Bundler) writeJSON(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.dir, name), data, 0o644)
}
