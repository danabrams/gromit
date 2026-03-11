package enrich

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnrichmentRun captures the full context and results of one enrichment pass.
type EnrichmentRun struct {
	RunID        string           `json:"run_id"`
	Timestamp    time.Time        `json:"timestamp"`
	Provider     string           `json:"provider"`
	Model        string           `json:"model"`
	Reasoning    string           `json:"reasoning"`
	Inputs       EnrichInput      `json:"inputs"`
	Request      RunRequest       `json:"request"`
	Results      []CategoryResult `json:"results"`
	CostUSD      float64          `json:"cost_usd"`
	InputTokens  int              `json:"input_tokens"`
	OutputTokens int              `json:"output_tokens"`
}

// NormalizeNilFields maps nil slices/maps to empty values.
// See CLAUDE.md nil-field normalization visibility convention.
func (r *EnrichmentRun) NormalizeNilFields() {
	if r.Results == nil {
		r.Results = []CategoryResult{}
	}
	if r.Request.Categories == nil {
		r.Request.Categories = []EnrichmentCategory{}
	}
	r.Inputs.NormalizeNilFields()
}

// NormalizeNilFields maps nil slices/maps to empty values.
// See CLAUDE.md nil-field normalization visibility convention.
func (e *EnrichInput) NormalizeNilFields() {
	if e.FileTree == nil {
		e.FileTree = []string{}
	}
}

// RunRequest specifies which categories to enrich.
type RunRequest struct {
	Categories []EnrichmentCategory `json:"categories"`
}

// CategoryResult captures the outcome of enriching one category.
type CategoryResult struct {
	Category     EnrichmentCategory `json:"category"`
	Success      bool               `json:"success"`
	FactCount    int                `json:"fact_count"`
	Error        string             `json:"error,omitempty"`
	CostUSD      float64            `json:"cost_usd"`
	InputTokens  int                `json:"input_tokens"`
	OutputTokens int                `json:"output_tokens"`
}

// RunStore manages enrichment run artifacts under inferred/runs/<run-id>/.
type RunStore struct{}

// NewRunStore creates a new RunStore.
func NewRunStore() *RunStore {
	return &RunStore{}
}

func (s *RunStore) runDir(cellPath, runID string) string {
	return filepath.Join(cellPath, "inferred", "runs", runID)
}

// SaveRun persists an enrichment run as a set of artifact files.
func (s *RunStore) SaveRun(cellPath string, run EnrichmentRun) error {
	run.NormalizeNilFields()

	dir := s.runDir(cellPath, run.RunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}

	// Write inputs.json
	if err := writeJSON(filepath.Join(dir, "inputs.json"), run.Inputs); err != nil {
		return fmt.Errorf("write inputs.json: %w", err)
	}

	// Write request.json
	if err := writeJSON(filepath.Join(dir, "request.json"), run.Request); err != nil {
		return fmt.Errorf("write request.json: %w", err)
	}

	// Write output.json (the full run)
	if err := writeJSON(filepath.Join(dir, "output.json"), run); err != nil {
		return fmt.Errorf("write output.json: %w", err)
	}

	// Write summary.md
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(renderSummary(run)), 0o644); err != nil {
		return fmt.Errorf("write summary.md: %w", err)
	}

	return nil
}

// LoadRun reads an enrichment run from its output.json artifact.
func (s *RunStore) LoadRun(cellPath, runID string) (EnrichmentRun, error) {
	data, err := os.ReadFile(filepath.Join(s.runDir(cellPath, runID), "output.json"))
	if err != nil {
		return EnrichmentRun{}, fmt.Errorf("read output.json: %w", err)
	}
	var run EnrichmentRun
	if err := json.Unmarshal(data, &run); err != nil {
		return EnrichmentRun{}, fmt.Errorf("unmarshal output.json: %w", err)
	}
	run.NormalizeNilFields()
	return run, nil
}

// ListRuns returns the run IDs present under inferred/runs/.
func (s *RunStore) ListRuns(cellPath string) ([]string, error) {
	runsDir := filepath.Join(cellPath, "inferred", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read runs dir: %w", err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func renderSummary(run EnrichmentRun) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Enrichment Run: %s\n\n", run.RunID)
	fmt.Fprintf(&b, "- **Timestamp:** %s\n", run.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Provider:** %s\n", run.Provider)
	fmt.Fprintf(&b, "- **Model:** %s\n", run.Model)
	fmt.Fprintf(&b, "- **Reasoning:** %s\n", run.Reasoning)
	fmt.Fprintf(&b, "- **Cost:** $%.4f\n", run.CostUSD)
	fmt.Fprintf(&b, "- **Tokens:** %d input / %d output\n\n", run.InputTokens, run.OutputTokens)

	if len(run.Results) > 0 {
		fmt.Fprintf(&b, "## Category Results\n\n")
		fmt.Fprintf(&b, "| Category | Status | Facts |\n")
		fmt.Fprintf(&b, "|----------|--------|-------|\n")
		for _, r := range run.Results {
			status := "pass"
			if !r.Success {
				status = fmt.Sprintf("FAIL: %s", r.Error)
			}
			fmt.Fprintf(&b, "| %s | %s | %d |\n", r.Category, status, r.FactCount)
		}
	}

	return b.String()
}
