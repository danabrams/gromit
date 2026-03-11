package enrich

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)
	store := NewRunStore()

	run := EnrichmentRun{
		RunID:        "run-001",
		Timestamp:    time.Now(),
		Provider:     "claude",
		Model:        "sonnet",
		Reasoning:    "medium",
		Inputs:       EnrichInput{ProjectName: "test-project", FileTree: []string{"main.go", "go.mod"}},
		Request:      RunRequest{Categories: AllCategories()},
		Results:      []CategoryResult{},
		CostUSD:      0.05,
		InputTokens:  1000,
		OutputTokens: 500,
	}

	if err := store.SaveRun(dir, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	loaded, err := store.LoadRun(dir, "run-001")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if loaded.RunID != "run-001" {
		t.Errorf("RunID = %q, want run-001", loaded.RunID)
	}
	if loaded.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want 0.05", loaded.CostUSD)
	}

	// Verify inputs.json was written
	inputsPath := filepath.Join(dir, "inferred", "runs", "run-001", "inputs.json")
	inputsData, err := os.ReadFile(inputsPath)
	if err != nil {
		t.Fatalf("inputs.json not written: %v", err)
	}
	if len(inputsData) == 0 {
		t.Error("inputs.json should not be empty")
	}
}

func TestRunStore_ListRuns(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)
	store := NewRunStore()

	for _, id := range []string{"run-001", "run-002"} {
		store.SaveRun(dir, EnrichmentRun{RunID: id, Timestamp: time.Now()})
	}

	runs, err := store.ListRuns(dir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}
}

func TestRenderSummary_EscapesPipeInErrorAndCategory(t *testing.T) {
	run := EnrichmentRun{
		RunID:     "run-pipe",
		Timestamp: time.Now(),
		Provider:  "claude",
		Model:     "sonnet",
		Results: []CategoryResult{
			{
				Category:  CategoryEntrypoint,
				Success:   false,
				Error:     "unexpected | in output | twice",
				FactCount: 0,
			},
		},
	}

	summary := renderSummary(run)

	// The error pipes must be escaped so the table row has exactly 4 pipe delimiters.
	if strings.Contains(summary, "unexpected | in") {
		t.Errorf("pipe characters in error were not escaped:\n%s", summary)
	}
	if !strings.Contains(summary, `unexpected \| in output \| twice`) {
		t.Errorf("expected escaped pipes in summary, got:\n%s", summary)
	}

	// Verify the table row has the correct number of unescaped pipe delimiters.
	for _, line := range strings.Split(summary, "\n") {
		if strings.HasPrefix(line, "| ") && strings.Contains(line, "FAIL") {
			// Count unescaped pipes by removing escaped ones first.
			stripped := strings.ReplaceAll(line, `\|`, "")
			count := strings.Count(stripped, "|")
			// A well-formed 3-column row has 4 unescaped pipes: | a | b | c |
			if count != 4 {
				t.Errorf("malformed table row (expected 4 unescaped pipes, got %d): %s", count, line)
			}
		}
	}
}

func TestRunStore_SavesSummary(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)
	store := NewRunStore()

	run := EnrichmentRun{
		RunID:        "run-001",
		Timestamp:    time.Now(),
		Provider:     "claude",
		Model:        "sonnet",
		CostUSD:      0.12,
		InputTokens:  5000,
		OutputTokens: 2000,
		Results: []CategoryResult{
			{Category: CategoryEntrypoint, Success: true, FactCount: 3},
			{Category: CategoryRiskyArea, Success: false, Error: "timeout"},
		},
	}

	store.SaveRun(dir, run)

	summaryPath := filepath.Join(dir, "inferred", "runs", "run-001", "summary.md")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("summary.md not written: %v", err)
	}
	summary := string(data)
	if len(summary) == 0 {
		t.Error("summary.md should not be empty")
	}
	if !strings.Contains(summary, "Cost") {
		t.Errorf("summary.md should contain 'Cost', got: %s", summary)
	}
}
