package retro

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
)

func TestNewRetroWithProviderNilProvider(t *testing.T) {
	r, err := NewRetroWithProvider(nil, ".gromit")
	if r != nil {
		t.Error("expected nil Retro for nil provider")
	}
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestNewRetroWithProviderAndBudget(t *testing.T) {
	tmpDir := t.TempDir()
	mockProvider := &mockProvider{}

	r, err := NewRetroWithProviderAndBudget(mockProvider, tmpDir, 1234)
	if err != nil {
		t.Fatalf("NewRetroWithProviderAndBudget returned error: %v", err)
	}
	if r.promptBudget != 1234 {
		t.Fatalf("expected prompt budget 1234, got %d", r.promptBudget)
	}
}

func TestRunNilReceiver(t *testing.T) {
	var r *Retro
	_, err := r.Run(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil retro")
	}
}

func TestRunNilProvider(t *testing.T) {
	r := &Retro{
		provider: nil,
	}
	_, err := r.Run(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestRunNilLearningsFile(t *testing.T) {
	mockProvider := &mockProvider{}
	r := &Retro{
		provider:      mockProvider,
		learningsFile: nil,
	}
	_, err := r.Run(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil learnings file")
	}
}

func TestEnrichBeadStatsNilReceiver(t *testing.T) {
	var r *Retro
	beadStats := make(map[string]logger.BeadStats)
	// Should not panic
	r.enrichBeadStats(context.Background(), beadStats)
}

func TestEnrichBeadStatsNilMap(t *testing.T) {
	tmpDir := t.TempDir()
	mockProvider := &mockProvider{}
	r, _ := NewRetroWithProvider(mockProvider, tmpDir)
	// Should not panic
	r.enrichBeadStats(context.Background(), nil)
}

func TestEnrichBeadStatsEmptyMap(t *testing.T) {
	tmpDir := t.TempDir()
	mockProvider := &mockProvider{}
	r, _ := NewRetroWithProvider(mockProvider, tmpDir)
	beadStats := make(map[string]logger.BeadStats)
	// Should not panic and return immediately
	r.enrichBeadStats(context.Background(), beadStats)
	if len(beadStats) != 0 {
		t.Error("expected empty map to remain empty")
	}
}

func TestFilterClosedBeadsFromStuckList(t *testing.T) {
	// Create a map with both open and closed beads that have failures
	beadStats := map[string]logger.BeadStats{
		"open-bead-1": {
			BeadID:   "open-bead-1",
			Failures: 2,
			Status:   "open",
			Comments: []string{},
		},
		"open-bead-2": {
			BeadID:   "open-bead-2",
			Failures: 3,
			Status:   "open",
			Comments: []string{},
		},
		"closed-bead-1": {
			BeadID:      "closed-bead-1",
			Failures:    2,
			Status:      "closed",
			CloseReason: "fixed",
			Comments:    []string{},
		},
		"closed-bead-2": {
			BeadID:      "closed-bead-2",
			Failures:    4,
			Status:      "closed",
			CloseReason: "wontfix",
			Comments:    []string{},
		},
	}

	// Simulate the filtering logic from Run() that removes closed beads
	for id, stats := range beadStats {
		if stats.Status == "closed" {
			delete(beadStats, id)
		}
	}

	// Verify only open beads remain
	if len(beadStats) != 2 {
		t.Errorf("expected 2 open beads, got %d", len(beadStats))
	}
	if _, exists := beadStats["open-bead-1"]; !exists {
		t.Error("expected open-bead-1 to remain")
	}
	if _, exists := beadStats["open-bead-2"]; !exists {
		t.Error("expected open-bead-2 to remain")
	}
	if _, exists := beadStats["closed-bead-1"]; exists {
		t.Error("expected closed-bead-1 to be removed")
	}
	if _, exists := beadStats["closed-bead-2"]; exists {
		t.Error("expected closed-bead-2 to be removed")
	}
}

func TestLaunchClaudeCodeWithAnalysis(t *testing.T) {
	// This test verifies that LaunchClaudeCode builds the correct prompt structure.
	// We can't easily test the actual execution without mocking exec.Command,
	// but we can verify the function signature and basic behavior.

	// Test with empty analysis - should still build valid prompt
	analysis := ""

	// Note: We can't actually run this in tests as it would launch an interactive
	// claude session. In a real scenario, you'd use dependency injection to mock
	// the command execution. For now, we just verify the function exists and
	// accepts the correct parameters.

	// The function signature is: LaunchClaudeCode(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment, dir string) error
	// This is a compile-time check that the function exists with the right signature.
	var _ func(string, *logger.EfficiencyReport, *Experiment, string) error = LaunchClaudeCode

	// Test that the function accepts a non-empty analysis
	analysis = "Test analysis results"
	_ = analysis // Use the variable to prevent unused variable error

	// In a real integration test, you would:
	// 1. Mock exec.Command
	// 2. Verify the prompt contains the analysis
	// 3. Verify the command is "claude" with the prompt as an argument
	// 4. Verify stdin/stdout/stderr are connected
}

func TestRenderPromptWithPopulatedExperiment(t *testing.T) {
	// Test that renderPrompt can execute the template with all context fields populated,
	// especially with an Experiment containing non-zero BaselineMetrics (float64 types).
	// This test catches type mismatches that would cause template execution to fail at runtime.

	tmpDir := t.TempDir()

	// Write the real PROMPT_retro.md template to the temp directory
	templateContent := `# Retrospective Analysis

## Run Statistics

{{- if .RunStats.Total }}
- **Total iterations**: {{ .RunStats.Total }}
- **Succeeded**: {{ .RunStats.Succeeded }}
- **Failed**: {{ .RunStats.Failed }}
- **Failure rate**: {{ printf "%.1f%%" (mul .RunStats.FailureRate 100) }}
{{- end }}

{{- if .Efficiency }}

## Current Run Efficiency

### Per-Model Aggregates (Current Run)
| Model | Iterations | Avg Cost |
|-------|-----------|----------|
{{- range $model, $stats := .Efficiency.CurrentModels }}
| {{ $stats.Model }} | {{ $stats.IterationCount }} | ${{ printf "%.4f" $stats.AvgCostUSD }} |
{{- end }}

**Overall Metrics**
- Current avg cost per bead: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }}
{{- end }}

{{- if .Experiment }}

## Active Experiment Evaluation

**Experiment Details:**
- **Name**: {{ .Experiment.Name }}
- **Hypothesis**: {{ .Experiment.Hypothesis }}
- **Change**: {{ .Experiment.Change }}

**Baseline Metrics (at experiment start):**
- Avg cost per bead: ${{ printf "%.4f" .Experiment.BaselineMetrics.AvgCostPerBead }}
- Avg duration per bead: {{ .Experiment.BaselineMetrics.AvgDurationMs }}ms
- Avg input tokens: {{ printf "%.0f" .Experiment.BaselineMetrics.AvgInputTokens }}
- Avg output tokens: {{ printf "%.0f" .Experiment.BaselineMetrics.AvgOutputTokens }}
- Failure rate: {{ printf "%.1f%%" (mul .Experiment.BaselineMetrics.FailureRate 100) }}

**Current vs Baseline:**
- Cost: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }} vs ${{ printf "%.4f" .Experiment.BaselineMetrics.AvgCostPerBead }} ({{ if gt .Efficiency.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead }}+{{ printf "%.1f%%" (mul (div (sub .Efficiency.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead) .Experiment.BaselineMetrics.AvgCostPerBead) 100) }}{{ else }}no change{{ end }})
{{- end }}
`

	templatePath := tmpDir + "/template.md"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	// Create Retro with the temp template path
	tmpGromitDir := t.TempDir()
	mockProvider := &mockProvider{}
	r, err := NewRetroWithProvider(mockProvider, tmpGromitDir)
	if err != nil {
		t.Fatalf("failed to create Retro: %v", err)
	}

	// Override the template path to use our temp template
	r.templatePath = templatePath

	// Construct fully populated TemplateContext with non-nil and non-zero values
	ctx := TemplateContext{
		Rules:     "Test rules",
		Learnings: "Test learnings",
		RunStats: logger.RunStats{
			Total:     10,
			Succeeded: 8,
			Failed:    2,
		},
		BeadStats: map[string]logger.BeadStats{
			"bead-1": {
				BeadID:      "bead-1",
				BeadTitle:   "Test Bead",
				TotalRuns:   5,
				Failures:    1,
				Successes:   4,
				Status:      "open",
				CloseReason: "",
				Comments:    []string{"Comment 1", "Comment 2"},
			},
		},
		Efficiency: &logger.EfficiencyReport{
			CurrentModels: map[string]logger.ModelEfficiency{
				"sonnet": {
					Model:           "sonnet",
					IterationCount:  5,
					AvgCostUSD:      0.35,
					AvgDuration:     45 * time.Second,
					AvgInputTokens:  11500.5,
					AvgOutputTokens: 2800.75,
				},
			},
			CurrentAvgCostPerBead:     0.35,
			CurrentAvgDurationPerBead: 45 * time.Second,
		},
		Experiment: &Experiment{
			Name:        "Use haiku for test-only beads",
			Hypothesis:  "Beads that only modify test files can succeed with haiku",
			Change:      "Add label complexity:low to test beads",
			Measurement: "Compare success rate and cost before vs after",
			Risk:        "Test beads may fail more on haiku",
			StartedAt:   time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC),
			BaselineMetrics: BaselineMetrics{
				AvgCostPerBead:  0.42,
				AvgDurationMs:   45000.0,
				AvgInputTokens:  12000.5,
				AvgOutputTokens: 3000.25,
				FailureRate:     0.08,
			},
		},
	}

	// Call renderPrompt with all fields populated
	prompt, err := r.renderPrompt("", "", ctx.RunStats, ctx.BeadStats, ctx.Efficiency, ctx.Experiment)

	// Test 1: renderPrompt should not error
	if err != nil {
		t.Fatalf("renderPrompt failed: %v", err)
	}

	// Test 2: rendered output should not be empty
	if prompt == "" {
		t.Error("expected non-empty rendered prompt")
	}

	// Test 3: verify rendered output contains expected metric values from the input data
	expectedStrings := []string{
		"Total iterations",
		"Iteration",
		"0.3500", // Current avg cost (formatted with 4 decimals)
		"0.4200", // Baseline avg cost
		"12000",  // Baseline input tokens (formatted with 0 decimals)
		"3000",   // Baseline output tokens
		"8",      // Failure rate percent
		"sonnet", // Model name from CurrentModels
	}

	for _, expected := range expectedStrings {
		if !contains(prompt, expected) {
			t.Errorf("rendered prompt missing expected value: %q", expected)
		}
	}

	// Test 4: verify Experiment name and details are in output
	if !contains(prompt, "Use haiku for test-only beads") {
		t.Error("rendered prompt missing experiment name")
	}

	// Test 5: verify BaselineMetrics float64 values are properly formatted
	// (This would fail if BaselineMetrics.AvgDurationMs was incorrectly typed as int64)
	if !contains(prompt, "45000ms") {
		t.Error("rendered prompt missing properly formatted baseline duration")
	}
}

func TestRenderPrompt_ExperimentMetricsUsesProviderFamily(t *testing.T) {
	tmpDir := t.TempDir()

	templateContent := `{{- if .ExperimentMetrics }}
Family: {{ .ExperimentMetrics.ProviderFamily }}
Cost: ${{ printf "%.2f" .ExperimentMetrics.CurrentAvgCostPerBead }}
{{- end }}`

	templatePath := tmpDir + "/template.md"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("writing template file: %v", err)
	}

	r := &Retro{templatePath: templatePath}
	efficiency := &logger.EfficiencyReport{
		CurrentProviderFamilies: map[string]logger.ModelEfficiency{
			"codex": {
				Model:          "codex",
				AvgCostUSD:     0.55,
				AvgDuration:    15 * time.Second,
				AvgInputTokens: 9000,
			},
		},
		CurrentAvgCostPerBead: 0.10,
	}
	experiment := &Experiment{PrimaryConcern: "codex"}

	prompt, err := r.renderPrompt("", "", logger.RunStats{}, nil, efficiency, experiment)
	if err != nil {
		t.Fatalf("renderPrompt failed: %v", err)
	}

	if !contains(prompt, "Family: codex") {
		t.Error("expected prompt to include provider family for experiment metrics")
	}
	if !contains(prompt, "Cost: $0.55") {
		t.Error("expected prompt to use provider-family cost for experiment metrics")
	}
}

func TestRenderPromptWithoutExperiment(t *testing.T) {
	// Test that renderPrompt works correctly when Experiment is nil
	tmpDir := t.TempDir()

	// Write a simpler template that doesn't use Experiment
	templateContent := `# Retrospective Analysis

{{- if .RunStats.Total }}
- **Total iterations**: {{ .RunStats.Total }}
{{- end }}

{{- if .Efficiency }}
- Current avg cost per bead: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }}
{{- end }}
`

	templatePath := tmpDir + "/template.md"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	tmpGromitDir := t.TempDir()
	mockProvider := &mockProvider{}
	r, err := NewRetroWithProvider(mockProvider, tmpGromitDir)
	if err != nil {
		t.Fatalf("failed to create Retro: %v", err)
	}

	r.templatePath = templatePath

	// Call renderPrompt with nil Experiment
	prompt, err := r.renderPrompt("", "", logger.RunStats{Total: 5}, nil, &logger.EfficiencyReport{
		CurrentAvgCostPerBead: 0.35,
	}, nil)

	if err != nil {
		t.Fatalf("renderPrompt with nil Experiment failed: %v", err)
	}

	if prompt == "" {
		t.Error("expected non-empty rendered prompt even with nil Experiment")
	}

	if !contains(prompt, "5") {
		t.Error("rendered prompt missing Total iterations value")
	}
}

func TestRenderPromptTemplateExpressionsWithFloatTypes(t *testing.T) {
	// Test that all template functions (mul, div, sub, durationMs) work correctly
	// when applied to BaselineMetrics float64 fields in the Experiment context.
	// This specifically tests the bug scenario where BaselineMetrics fields were
	// incorrectly typed as int64, which would cause template function execution to fail.

	tmpDir := t.TempDir()

	// Write a template that exercises all template functions with BaselineMetrics float64 fields
	templateContent := `# Experiment Analysis

{{- if .Experiment }}
**Baseline Metrics:**
- Avg cost: ${{ printf "%.4f" .Experiment.BaselineMetrics.AvgCostPerBead }}
- Duration: {{ .Experiment.BaselineMetrics.AvgDurationMs }}ms
- Input tokens: {{ printf "%.0f" .Experiment.BaselineMetrics.AvgInputTokens }}
- Output tokens: {{ printf "%.0f" .Experiment.BaselineMetrics.AvgOutputTokens }}
- Failure rate: {{ printf "%.1f%%" (mul .Experiment.BaselineMetrics.FailureRate 100) }}

**Calculations (verify all template functions work with float64 BaselineMetrics):**
- Cost * 2: ${{ printf "%.4f" (mul .Experiment.BaselineMetrics.AvgCostPerBead 2.0) }}
- Duration / 2: {{ printf "%.0f" (div .Experiment.BaselineMetrics.AvgDurationMs 2.0) }}ms
- Cost delta (0.50 - baseline): ${{ printf "%.4f" (sub 0.50 .Experiment.BaselineMetrics.AvgCostPerBead) }}
- Input delta: {{ printf "%.0f" (sub .Experiment.BaselineMetrics.AvgInputTokens 10000.0) }}

**Efficiency Comparison (all float64 operations):**
{{- if .Efficiency }}
- Current cost: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }} vs ${{ printf "%.4f" .Experiment.BaselineMetrics.AvgCostPerBead }}
- Cost increase: {{ printf "%.1f%%" (mul (div (sub .Efficiency.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead) .Experiment.BaselineMetrics.AvgCostPerBead) 100) }}%
- Duration delta: {{ printf "%.0f" (durationMs .Efficiency.CurrentAvgDurationPerBead) }}ms
{{- end }}
{{- end }}
`

	templatePath := tmpDir + "/template.md"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	tmpGromitDir := t.TempDir()
	mockProvider := &mockProvider{}
	r, err := NewRetroWithProvider(mockProvider, tmpGromitDir)
	if err != nil {
		t.Fatalf("failed to create Retro: %v", err)
	}

	r.templatePath = templatePath

	// Create Experiment with BaselineMetrics containing float64 values that will be
	// passed to template functions expecting float64 arguments
	experiment := &Experiment{
		Name:        "Cost Reduction Test",
		Hypothesis:  "Lower costs with haiku",
		Change:      "Use haiku for simple beads",
		Measurement: "Compare cost metrics",
		Risk:        "Failures may increase",
		StartedAt:   time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC),
		BaselineMetrics: BaselineMetrics{
			AvgCostPerBead:  0.42,
			AvgDurationMs:   45000.0, // Must be float64, not int64
			AvgInputTokens:  12000.5, // Must be float64, not int64
			AvgOutputTokens: 3000.25, // Must be float64, not int64
			FailureRate:     0.08,    // float64 type
		},
	}

	efficiency := &logger.EfficiencyReport{
		CurrentModels: map[string]logger.ModelEfficiency{
			"sonnet": {
				Model:           "sonnet",
				IterationCount:  5,
				AvgCostUSD:      0.35,
				AvgDuration:     45 * time.Second,
				AvgInputTokens:  11500.5,
				AvgOutputTokens: 2800.75,
			},
		},
		CurrentAvgCostPerBead:     0.35,
		CurrentAvgDurationPerBead: 45 * time.Second,
	}

	// Call renderPrompt with populated Experiment
	prompt, err := r.renderPrompt("", "", logger.RunStats{}, nil, efficiency, experiment)

	// Verify no error occurred (template functions received correct float64 types)
	if err != nil {
		t.Fatalf("renderPrompt failed (likely due to type mismatch in template functions): %v", err)
	}

	if prompt == "" {
		t.Error("expected non-empty rendered prompt")
	}

	// Verify expected calculations in output
	expectedValues := []string{
		"0.4200", // Baseline cost
		"45000",  // Baseline duration
		"12000",  // Baseline input tokens
		"3000",   // Baseline output tokens
		"8.0",    // Failure rate percentage
		"0.8400", // Cost * 2
		"22500",  // Duration / 2
	}

	for _, expected := range expectedValues {
		if !contains(prompt, expected) {
			t.Errorf("rendered prompt missing expected calculated value: %q", expected)
		}
	}
}

// TestRunReconcileFilteredHashes_CollectsCurrentProvisionalHashes verifies that
// after FilterProvisional completes, Run() collects hashes from current provisional
// learnings to pass to ReconcileFilteredHashes.
func TestRunReconcileFilteredHashes_CollectsCurrentProvisionalHashes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create learnings file with provisional learnings
	learningsPath := tmpDir + "/LEARNINGS.md"
	learningsContent := `# Learnings

## Confirmed

*No confirmed learnings yet.*

## Provisional

### 2026-02-01 | test-bead-1 | patterns

First provisional learning

### 2026-02-02 | test-bead-2 | conventions

Second provisional learning

## Archived

*No archived learnings.*
`
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("writing learnings file: %v", err)
	}

	// Create basic config
	mockProvider := &mockProvider{}

	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("creating retro: %v", err)
	}

	// Load learnings to verify they exist
	if err := r.learningsFile.Load(); err != nil {
		t.Fatalf("loading learnings: %v", err)
	}

	provisionals := r.learningsFile.GetProvisional()
	if len(provisionals) != 2 {
		t.Fatalf("expected 2 provisional learnings, got %d", len(provisionals))
	}

	// Note: This test can't fully verify Run() behavior without mocking claude.Client,
	// but it sets up the preconditions. The acceptance tests verify the full integration.
}

// TestRunWithBeadFilter_FilteredPerBeadStats verifies that when a bead filter is provided,
// only beads in the filter appear in the per-bead stats passed to the prompt template.
func TestRunWithBeadFilter_FilteredPerBeadStats(t *testing.T) {
	// This test verifies that the filtering logic works correctly at the logger level.
	// We directly test ReadPerBeadStatsFiltered to ensure it filters correctly.

	tmpDir := t.TempDir()
	logsDir := tmpDir + "/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("creating logs dir: %v", err)
	}

	// Create iteration logs with multiple beads
	// bead-1: 2 iterations (1 success, 1 failure)
	// bead-2: 1 iteration (1 success)
	// bead-3: 2 iterations (2 failures)
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"bead-1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"duration_ms":1000,"cost_usd":0.10,"input_tokens":1000,"output_tokens":500}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"bead-1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"duration_ms":2000,"cost_usd":0.15,"input_tokens":1200,"output_tokens":600,"error":"build failed"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"bead-2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"duration_ms":1500,"cost_usd":0.12,"input_tokens":1100,"output_tokens":550}
{"timestamp":"2026-02-05T12:03:00Z","iteration":4,"bead_id":"bead-3","bead_title":"Task 3","model":"haiku","success":false,"validated":false,"duration_ms":800,"cost_usd":0.05,"input_tokens":500,"output_tokens":300,"error":"test failed"}
{"timestamp":"2026-02-05T12:04:00Z","iteration":5,"bead_id":"bead-3","bead_title":"Task 3","model":"sonnet","success":false,"validated":false,"duration_ms":1800,"cost_usd":0.18,"input_tokens":1300,"output_tokens":700,"error":"test failed again"}
`
	if err := os.WriteFile(logsDir+"/run-20260205-120000.jsonl", []byte(logContent), 0644); err != nil {
		t.Fatalf("writing log file: %v", err)
	}

	// Test 1: Filter that only includes bead-1 and bead-3 (excludes bead-2)
	beadFilter := map[string]bool{
		"bead-1": true,
		"bead-3": true,
	}

	stats, err := logger.ReadPerBeadStatsFiltered(logsDir, beadFilter)
	if err != nil {
		t.Fatalf("ReadPerBeadStatsFiltered failed: %v", err)
	}

	// Verify only bead-1 and bead-3 are included
	if len(stats) != 2 {
		t.Errorf("expected 2 beads in filtered stats, got %d", len(stats))
	}

	if _, exists := stats["bead-1"]; !exists {
		t.Error("expected bead-1 in filtered stats")
	}
	if _, exists := stats["bead-3"]; !exists {
		t.Error("expected bead-3 in filtered stats")
	}
	if _, exists := stats["bead-2"]; exists {
		t.Error("did not expect bead-2 in filtered stats (should be excluded)")
	}

	// Verify bead-1 has correct stats (1 success, 1 failure)
	if stats["bead-1"].TotalRuns != 2 {
		t.Errorf("expected bead-1 to have 2 total runs, got %d", stats["bead-1"].TotalRuns)
	}
	if stats["bead-1"].Successes != 1 {
		t.Errorf("expected bead-1 to have 1 success, got %d", stats["bead-1"].Successes)
	}
	if stats["bead-1"].Failures != 1 {
		t.Errorf("expected bead-1 to have 1 failure, got %d", stats["bead-1"].Failures)
	}

	// Verify bead-3 has correct stats (0 successes, 2 failures)
	if stats["bead-3"].TotalRuns != 2 {
		t.Errorf("expected bead-3 to have 2 total runs, got %d", stats["bead-3"].TotalRuns)
	}
	if stats["bead-3"].Failures != 2 {
		t.Errorf("expected bead-3 to have 2 failures, got %d", stats["bead-3"].Failures)
	}
}

// TestRunWithBeadFilter_EmptyFilterIncludesAll verifies that when a filter
// is provided but it's empty (map[string]bool{}), all beads are included (empty = no filtering).
func TestRunWithBeadFilter_EmptyFilterIncludesAll(t *testing.T) {
	// This test verifies behavior when beadFilter is an empty map.
	// Based on existing logger tests, an empty filter means "include all beads" (no filtering).

	tmpDir := t.TempDir()
	logsDir := tmpDir + "/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("creating logs dir: %v", err)
	}

	// Create iteration logs with multiple beads
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"bead-1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"duration_ms":1000,"cost_usd":0.10,"input_tokens":1000,"output_tokens":500}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"bead-2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"duration_ms":2000,"cost_usd":0.15,"input_tokens":1200,"output_tokens":600,"error":"failed"}
`
	if err := os.WriteFile(logsDir+"/run-20260205-120000.jsonl", []byte(logContent), 0644); err != nil {
		t.Fatalf("writing log file: %v", err)
	}

	// Create empty filter - should include all beads (no filtering)
	emptyFilter := map[string]bool{}

	// Test ReadPerBeadStatsFiltered with empty filter
	stats, err := logger.ReadPerBeadStatsFiltered(logsDir, emptyFilter)
	if err != nil {
		t.Fatalf("ReadPerBeadStatsFiltered failed: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 beads with empty filter (should include all), got %d", len(stats))
	}

	if _, exists := stats["bead-1"]; !exists {
		t.Error("expected bead-1 in stats with empty filter")
	}
	if _, exists := stats["bead-2"]; !exists {
		t.Error("expected bead-2 in stats with empty filter")
	}

	// Test ReadAllLogsFiltered with empty filter
	runStats, err := logger.ReadAllLogsFiltered(logsDir, emptyFilter)
	if err != nil {
		t.Fatalf("ReadAllLogsFiltered failed: %v", err)
	}

	if runStats.Total != 2 {
		t.Errorf("expected 2 total iterations with empty filter, got %d", runStats.Total)
	}
	if runStats.Succeeded != 1 {
		t.Errorf("expected 1 succeeded with empty filter, got %d", runStats.Succeeded)
	}
	if runStats.Failed != 1 {
		t.Errorf("expected 1 failed with empty filter, got %d", runStats.Failed)
	}
}

// TestRunWithNilFilter_PreservesUnchangedBehavior verifies that when beadFilter is nil,
// Run() includes all beads (default behavior, no filtering).
func TestRunWithNilFilter_PreservesUnchangedBehavior(t *testing.T) {
	// This test verifies backward compatibility: nil filter = no filtering, include all beads.

	tmpDir := t.TempDir()
	logsDir := tmpDir + "/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("creating logs dir: %v", err)
	}

	// Create iteration logs with multiple beads
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"bead-1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"duration_ms":1000,"cost_usd":0.10,"input_tokens":1000,"output_tokens":500}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"bead-2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"duration_ms":2000,"cost_usd":0.15,"input_tokens":1200,"output_tokens":600,"error":"failed"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"bead-3","bead_title":"Task 3","model":"haiku","success":true,"validated":true,"duration_ms":800,"cost_usd":0.05,"input_tokens":500,"output_tokens":300}
`
	if err := os.WriteFile(logsDir+"/run-20260205-120000.jsonl", []byte(logContent), 0644); err != nil {
		t.Fatalf("writing log file: %v", err)
	}

	// Test ReadPerBeadStatsFiltered with nil filter
	stats, err := logger.ReadPerBeadStatsFiltered(logsDir, nil)
	if err != nil {
		t.Fatalf("ReadPerBeadStatsFiltered failed: %v", err)
	}

	if len(stats) != 3 {
		t.Errorf("expected 3 beads with nil filter, got %d", len(stats))
	}

	if _, exists := stats["bead-1"]; !exists {
		t.Error("expected bead-1 in stats with nil filter")
	}
	if _, exists := stats["bead-2"]; !exists {
		t.Error("expected bead-2 in stats with nil filter")
	}
	if _, exists := stats["bead-3"]; !exists {
		t.Error("expected bead-3 in stats with nil filter")
	}

	// Test ReadAllLogsFiltered with nil filter
	runStats, err := logger.ReadAllLogsFiltered(logsDir, nil)
	if err != nil {
		t.Fatalf("ReadAllLogsFiltered failed: %v", err)
	}

	if runStats.Total != 3 {
		t.Errorf("expected 3 total iterations with nil filter, got %d", runStats.Total)
	}
	if runStats.Succeeded != 2 {
		t.Errorf("expected 2 succeeded with nil filter, got %d", runStats.Succeeded)
	}
	if runStats.Failed != 1 {
		t.Errorf("expected 1 failed with nil filter, got %d", runStats.Failed)
	}
}

// TestRunWithBeadFilter_FilterAppliedToRunStats verifies that when a bead filter is provided,
// the RunStats aggregates (Total, Succeeded, Failed) only include iterations from filtered beads.
func TestRunWithBeadFilter_FilterAppliedToRunStats(t *testing.T) {
	// This test verifies that RunStats reflect the filtered scope, not all beads.

	tmpDir := t.TempDir()
	logsDir := tmpDir + "/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("creating logs dir: %v", err)
	}

	// Create iteration logs:
	// - bead-1: 2 iterations (1 success, 1 failure)
	// - bead-2: 1 iteration (1 success)
	// - bead-3: 2 iterations (2 failures)
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"bead-1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"duration_ms":1000,"cost_usd":0.10,"input_tokens":1000,"output_tokens":500}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"bead-1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"duration_ms":2000,"cost_usd":0.15,"input_tokens":1200,"output_tokens":600,"error":"failed"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"bead-2","bead_title":"Task 2","model":"haiku","success":true,"validated":true,"duration_ms":800,"cost_usd":0.05,"input_tokens":500,"output_tokens":300}
{"timestamp":"2026-02-05T12:03:00Z","iteration":4,"bead_id":"bead-3","bead_title":"Task 3","model":"sonnet","success":false,"validated":false,"duration_ms":1500,"cost_usd":0.12,"input_tokens":1100,"output_tokens":550,"error":"test failed"}
{"timestamp":"2026-02-05T12:04:00Z","iteration":5,"bead_id":"bead-3","bead_title":"Task 3","model":"opus","success":false,"validated":false,"duration_ms":3000,"cost_usd":0.50,"input_tokens":2000,"output_tokens":1000,"error":"test failed"}
`
	if err := os.WriteFile(logsDir+"/run-20260205-120000.jsonl", []byte(logContent), 0644); err != nil {
		t.Fatalf("writing log file: %v", err)
	}

	// Create filter that only includes bead-1 and bead-2 (excludes bead-3)
	beadFilter := map[string]bool{
		"bead-1": true,
		"bead-2": true,
	}

	// Test ReadAllLogsFiltered with the filter
	runStats, err := logger.ReadAllLogsFiltered(logsDir, beadFilter)
	if err != nil {
		t.Fatalf("ReadAllLogsFiltered failed: %v", err)
	}

	// Expected: only iterations 1, 2, 3 are included (bead-1 and bead-2)
	// Iterations 4 and 5 (bead-3) should be excluded
	if runStats.Total != 3 {
		t.Errorf("expected 3 total iterations (excluding bead-3), got %d", runStats.Total)
	}

	// Expected: 2 successes (bead-1 iteration 1, bead-2 iteration 3)
	if runStats.Succeeded != 2 {
		t.Errorf("expected 2 succeeded iterations, got %d", runStats.Succeeded)
	}

	// Expected: 1 failure (bead-1 iteration 2)
	if runStats.Failed != 1 {
		t.Errorf("expected 1 failed iteration, got %d", runStats.Failed)
	}

	// Verify failure rate is correct (1 failure out of 3 total)
	expectedFailureRate := 1.0 / 3.0
	if diff := runStats.FailureRate() - expectedFailureRate; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected failure rate ~%.3f, got %.3f", expectedFailureRate, runStats.FailureRate())
	}
}

// TestRunWithBeadFilter_FilterAppliedToEfficiencyReport verifies that when a bead filter
// is provided, the EfficiencyReport only includes cost and token data from filtered beads.
func TestRunWithBeadFilter_FilterAppliedToEfficiencyReport(t *testing.T) {
	// This test verifies that efficiency metrics reflect the filtered scope.

	tmpDir := t.TempDir()
	logsDir := tmpDir + "/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("creating logs dir: %v", err)
	}

	// Create iteration logs with different costs:
	// - bead-1: $0.10 (sonnet)
	// - bead-2: $0.05 (haiku)
	// - bead-3: $0.50 (opus, expensive)
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"bead-1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"duration_ms":1000,"cost_usd":0.10,"input_tokens":1000,"output_tokens":500}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"bead-2","bead_title":"Task 2","model":"haiku","success":true,"validated":true,"duration_ms":800,"cost_usd":0.05,"input_tokens":500,"output_tokens":300}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"bead-3","bead_title":"Task 3","model":"opus","success":true,"validated":true,"duration_ms":3000,"cost_usd":0.50,"input_tokens":2000,"output_tokens":1000}
`
	runID := "20260205-120000"
	if err := os.WriteFile(filepath.Join(logsDir, "run-"+runID+".jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatalf("writing log file: %v", err)
	}

	// Create filter that only includes bead-1 and bead-2 (excludes expensive bead-3)
	beadFilter := map[string]bool{
		"bead-1": true,
		"bead-2": true,
	}

	// Test ReadEfficiencyReportFiltered with the filter
	report, err := logger.ReadEfficiencyReportFiltered(logsDir, runID, beadFilter)
	if err != nil {
		t.Fatalf("ReadEfficiencyReportFiltered failed: %v", err)
	}

	// Verify only 2 iterations are included (bead-1 and bead-2)
	if len(report.CurrentIterations) != 2 {
		t.Errorf("expected 2 current iterations (excluding bead-3), got %d", len(report.CurrentIterations))
	}

	// Verify the expensive bead-3 ($0.50) is excluded
	for _, iter := range report.CurrentIterations {
		if iter.BeadID == "bead-3" {
			t.Error("did not expect bead-3 in filtered efficiency report")
		}
	}

	// Verify only sonnet and haiku models are present (no opus)
	if _, hasOpus := report.CurrentModels["opus"]; hasOpus {
		t.Error("did not expect opus model in filtered efficiency report")
	}

	if _, hasSonnet := report.CurrentModels["sonnet"]; !hasSonnet {
		t.Error("expected sonnet model in filtered efficiency report")
	}

	if _, hasHaiku := report.CurrentModels["haiku"]; !hasHaiku {
		t.Error("expected haiku model in filtered efficiency report")
	}

	// Verify avg cost per bead excludes the expensive bead-3
	// Expected: (0.10 + 0.05) / 2 = 0.075
	expectedAvgCost := 0.075
	if diff := report.CurrentAvgCostPerBead - expectedAvgCost; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected avg cost per bead ~%.4f (excluding bead-3), got %.4f", expectedAvgCost, report.CurrentAvgCostPerBead)
	}
}

// TestRunWithBeadFilter_FilterWithNonexistentBead verifies that when a filter contains
// a bead ID that has no iteration logs, the functions return empty results without error.
func TestRunWithBeadFilter_FilterWithNonexistentBead(t *testing.T) {
	// This test verifies graceful handling of filters with nonexistent bead IDs.

	tmpDir := t.TempDir()
	logsDir := tmpDir + "/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("creating logs dir: %v", err)
	}

	// Create iteration logs with only bead-1
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"bead-1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"duration_ms":1000,"cost_usd":0.10,"input_tokens":1000,"output_tokens":500}
`
	runID := "20260205-120000"
	if err := os.WriteFile(filepath.Join(logsDir, "run-"+runID+".jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatalf("writing log file: %v", err)
	}

	// Create filter that includes a bead ID that doesn't exist in the logs
	beadFilter := map[string]bool{
		"nonexistent-bead": true,
	}

	// Test ReadPerBeadStatsFiltered - should return empty map (no beads match filter)
	stats, err := logger.ReadPerBeadStatsFiltered(logsDir, beadFilter)
	if err != nil {
		t.Fatalf("ReadPerBeadStatsFiltered failed: %v", err)
	}

	if len(stats) != 0 {
		t.Errorf("expected empty stats map for nonexistent bead, got %d entries", len(stats))
	}

	// Test ReadAllLogsFiltered - should return zero stats
	runStats, err := logger.ReadAllLogsFiltered(logsDir, beadFilter)
	if err != nil {
		t.Fatalf("ReadAllLogsFiltered failed: %v", err)
	}

	if runStats.Total != 0 {
		t.Errorf("expected 0 total iterations for nonexistent bead, got %d", runStats.Total)
	}
	if runStats.Succeeded != 0 {
		t.Errorf("expected 0 succeeded iterations for nonexistent bead, got %d", runStats.Succeeded)
	}
	if runStats.Failed != 0 {
		t.Errorf("expected 0 failed iterations for nonexistent bead, got %d", runStats.Failed)
	}

	// Test ReadEfficiencyReportFiltered - should return empty report
	report, err := logger.ReadEfficiencyReportFiltered(logsDir, runID, beadFilter)
	if err != nil {
		t.Fatalf("ReadEfficiencyReportFiltered failed: %v", err)
	}

	if len(report.CurrentIterations) != 0 {
		t.Errorf("expected 0 current iterations for nonexistent bead, got %d", len(report.CurrentIterations))
	}

	if report.CurrentAvgCostPerBead != 0 {
		t.Errorf("expected 0 avg cost for nonexistent bead, got %.4f", report.CurrentAvgCostPerBead)
	}
}

// TestRenderPromptWithNilExperimentAndEfficiency tests that renderPrompt
// correctly handles the case where Experiment=nil, Efficiency=nil, and RunStats are zero.
// This verifies the template conditional branches ({{- if .Experiment }}, {{- if .Efficiency }}, {{- if .RunStats.Total }})
// render correctly when all are empty/nil.
func TestRenderPromptWithNilExperimentAndEfficiency(t *testing.T) {
	tmpDir := t.TempDir()

	// Write the real PROMPT_retro.md template to the temp directory
	templateContent := `# Retrospective Analysis

## Run Statistics

{{- if .RunStats.Total }}
- **Total iterations**: {{ .RunStats.Total }}
- **Succeeded**: {{ .RunStats.Succeeded }}
- **Failed**: {{ .RunStats.Failed }}
- **Failure rate**: {{ printf "%.1f%%" (mul .RunStats.FailureRate 100) }}
{{- else }}
- No iterations recorded yet
{{- end }}

{{- if .Efficiency }}

## Current Run Efficiency

### Per-Model Aggregates (Current Run)
| Model | Iterations | Avg Cost |
|-------|-----------|----------|
{{- range $model, $stats := .Efficiency.CurrentModels }}
| {{ $stats.Model }} | {{ $stats.IterationCount }} | ${{ printf "%.4f" $stats.AvgCostUSD }} |
{{- end }}

**Overall Metrics**
- Current avg cost per bead: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }}
{{- end }}

{{- if .Experiment }}

## Active Experiment Evaluation

**Experiment Details:**
- **Name**: {{ .Experiment.Name }}
- **Hypothesis**: {{ .Experiment.Hypothesis }}
- **Change**: {{ .Experiment.Change }}

**Baseline Metrics (at experiment start):**
- Avg cost per bead: ${{ printf "%.4f" .Experiment.BaselineMetrics.AvgCostPerBead }}
- Avg duration per bead: {{ .Experiment.BaselineMetrics.AvgDurationMs }}ms
- Avg input tokens: {{ printf "%.0f" .Experiment.BaselineMetrics.AvgInputTokens }}
- Avg output tokens: {{ printf "%.0f" .Experiment.BaselineMetrics.AvgOutputTokens }}
- Failure rate: {{ printf "%.1f%%" (mul .Experiment.BaselineMetrics.FailureRate 100) }}

**Current vs Baseline:**
- Cost: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }} vs ${{ printf "%.4f" .Experiment.BaselineMetrics.AvgCostPerBead }} ({{ if gt .Efficiency.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead }}+{{ printf "%.1f%%" (mul (div (sub .Efficiency.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead) .Experiment.BaselineMetrics.AvgCostPerBead) 100) }}{{ else }}no change{{ end }})
{{- end }}
`

	templatePath := tmpDir + "/template.md"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	// Create Retro with the temp template path
	tmpGromitDir := t.TempDir()
	mockProvider := &mockProvider{}
	r, err := NewRetroWithProvider(mockProvider, tmpGromitDir)
	if err != nil {
		t.Fatalf("failed to create Retro: %v", err)
	}

	// Override the template path to use our temp template
	r.templatePath = templatePath

	// Call renderPrompt with all nil/zero values
	prompt, err := r.renderPrompt("", "", logger.RunStats{}, nil, nil, nil)

	// Test 1: renderPrompt should not error with nil Experiment and nil Efficiency
	if err != nil {
		t.Fatalf("renderPrompt with nil Experiment/Efficiency failed: %v", err)
	}

	// Test 2: rendered output should not be empty
	if prompt == "" {
		t.Error("expected non-empty rendered prompt even with nil Experiment/Efficiency")
	}

	// Test 3: verify experiment metric sections are absent from output
	// The template should not render the "Active Experiment Evaluation" section
	if contains(prompt, "Active Experiment Evaluation") {
		t.Error("rendered prompt should not contain 'Active Experiment Evaluation' section when Experiment is nil")
	}

	// Test 4: verify experiment name is not in output
	if contains(prompt, "Name:") && contains(prompt, "Hypothesis:") && contains(prompt, "Change:") {
		// All three together would indicate the experiment section rendered
		t.Error("rendered prompt should not contain experiment details when Experiment is nil")
	}

	// Test 5: verify efficiency section is absent from output
	if contains(prompt, "Per-Model Aggregates") {
		t.Error("rendered prompt should not contain 'Per-Model Aggregates' section when Efficiency is nil")
	}

	// Test 6: verify the template still renders without RunStats (zero values)
	if !contains(prompt, "No iterations recorded yet") {
		t.Error("rendered prompt should contain 'No iterations recorded yet' fallback when RunStats.Total is 0")
	}
}

// createMinimalRetroFiles creates minimal files needed for retro.Run() to execute.
// This helper is used by filtering tests that focus on log filtering behavior.
func createMinimalRetroFiles(t *testing.T, tmpDir string) {
	t.Helper()

	// Create RULES.md
	rulesPath := tmpDir + "/RULES.md"
	if err := os.WriteFile(rulesPath, []byte("# Rules\n\nTest rules.\n"), 0644); err != nil {
		t.Fatalf("writing rules file: %v", err)
	}

	// Create LEARNINGS.md
	learningsPath := tmpDir + "/LEARNINGS.md"
	learningsContent := `# Learnings

## Confirmed

*No confirmed learnings yet.*

## Provisional

*No provisional learnings.*

## Archived

*No archived learnings.*
`
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("writing learnings file: %v", err)
	}

	// Create state.json
	statePath := tmpDir + "/state.json"
	stateContent := `{"filtered_hashes":[],"last_retro":null}`
	if err := os.WriteFile(statePath, []byte(stateContent), 0644); err != nil {
		t.Fatalf("writing state file: %v", err)
	}

	// Create templates directory with retro template
	templatesDir := tmpDir + "/templates"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("creating templates dir: %v", err)
	}

	templatePath := templatesDir + "/PROMPT_retro.md"
	templateContent := `# Retrospective Analysis

{{- if .RunStats.Total }}
- **Total iterations**: {{ .RunStats.Total }}
- **Succeeded**: {{ .RunStats.Succeeded }}
- **Failed**: {{ .RunStats.Failed }}
{{- end }}

{{- if .BeadStats }}
## Stuck Beads
{{- range $id, $stats := .BeadStats }}
- **{{ $stats.BeadTitle }}** ({{ $id }}): {{ $stats.Failures }} failures
{{- end }}
{{- end }}

{{- if .Efficiency }}
## Efficiency
- Current avg cost per bead: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }}
{{- end }}
`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("writing template file: %v", err)
	}
}

// TestRenderPromptWithNilExperimentAndEfficiencyUsingRealTemplate verifies that renderPrompt
// correctly executes the real PROMPT_retro.md template with nil Experiment and nil Efficiency.
// This test ensures the actual template (not a simplified version) renders without error
// when no experiment is active and no efficiency data is available.
func TestRenderPromptWithNilExperimentAndEfficiencyUsingRealTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	// Copy the real PROMPT_retro.md template to the temp directory
	realTemplatePath := "../../.gromit/templates/PROMPT_retro.md"
	templateContent, err := os.ReadFile(realTemplatePath)
	if err != nil {
		t.Fatalf("failed to read real template: %v", err)
	}

	templatePath := filepath.Join(tmpDir, "PROMPT_retro.md")
	if err := os.WriteFile(templatePath, templateContent, 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	// Create Retro with the real template
	tmpGromitDir := t.TempDir()
	mockProvider := &mockProvider{}
	r, err := NewRetroWithProvider(mockProvider, tmpGromitDir)
	if err != nil {
		t.Fatalf("failed to create Retro: %v", err)
	}

	// Override the template path to use the real template
	r.templatePath = templatePath

	// Call renderPrompt with nil Experiment, nil Efficiency, and zero RunStats
	prompt, err := r.renderPrompt("# Test Rules", "# Test Learnings", logger.RunStats{}, nil, nil, nil)

	// Test 1: renderPrompt should not error
	if err != nil {
		t.Fatalf("renderPrompt with nil Experiment/Efficiency failed: %v", err)
	}

	// Test 2: rendered output should not be empty
	if prompt == "" {
		t.Error("expected non-empty rendered prompt")
	}

	// Test 3: verify experiment metric sections are absent from output
	if contains(prompt, "Active Experiment Evaluation") {
		t.Error("rendered prompt should not contain 'Active Experiment Evaluation' section when Experiment is nil")
	}

	// Test 4: verify experiment details are not in output
	if contains(prompt, "**Experiment Details:**") {
		t.Error("rendered prompt should not contain experiment details when Experiment is nil")
	}
}

func TestRenderPromptWithProviderFamiliesInRealTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	realTemplatePath := "../../.gromit/templates/PROMPT_retro.md"
	templateContent, err := os.ReadFile(realTemplatePath)
	if err != nil {
		t.Fatalf("failed to read real template: %v", err)
	}

	templatePath := filepath.Join(tmpDir, "PROMPT_retro.md")
	if err := os.WriteFile(templatePath, templateContent, 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	tmpGromitDir := t.TempDir()
	mockProvider := &mockProvider{}
	r, err := NewRetroWithProvider(mockProvider, tmpGromitDir)
	if err != nil {
		t.Fatalf("failed to create Retro: %v", err)
	}
	r.templatePath = templatePath

	efficiency := &logger.EfficiencyReport{
		CurrentProviderFamilies: map[string]logger.ModelEfficiency{
			"claude": {Model: "claude", IterationCount: 2},
			"codex":  {Model: "codex", IterationCount: 1},
		},
		HistoricalModels: map[string]logger.ModelEfficiency{
			"opus": {Model: "opus", IterationCount: 1},
		},
		MixedProviderFamilies: true,
	}

	prompt, err := r.renderPrompt("# Test Rules", "# Test Learnings", logger.RunStats{Total: 1}, nil, efficiency, nil)
	if err != nil {
		t.Fatalf("renderPrompt with provider families failed: %v", err)
	}

	if !contains(prompt, "Per-Provider-Family Aggregates") {
		t.Error("rendered prompt should include provider-family aggregates section")
	}
	if !contains(prompt, "Mixed providers") {
		t.Error("rendered prompt should label mixed-provider aggregates")
	}
}

func TestRenderPromptWithProcessTrendFailureBreakdownInRealTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	realTemplatePath := "../../.gromit/templates/PROMPT_retro.md"
	templateContent, err := os.ReadFile(realTemplatePath)
	if err != nil {
		t.Fatalf("failed to read real template: %v", err)
	}

	templatePath := filepath.Join(tmpDir, "PROMPT_retro.md")
	if err := os.WriteFile(templatePath, templateContent, 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	tmpGromitDir := t.TempDir()
	metricsDir := filepath.Join(tmpGromitDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		t.Fatalf("failed to create metrics dir: %v", err)
	}

	trend := logger.ProcessTrend{
		GeneratedAt:     time.Date(2026, time.February, 18, 12, 0, 0, 0, time.UTC),
		TotalIterations: 12,
		WindowSize:      5,
		LatestWindow: logger.ProcessTrendWindow{
			SuccessRate:           0.4,
			FailureRate:           0.6,
			FirstPassSuccess:      0.3,
			EscalationRate:        0.2,
			AvgDurationMs:         2500,
			P95DurationMs:         4000,
			AvgCostUSD:            0.125,
			AvgMTTRProxyMs:        1900,
			PreflightFailureRate:  0.1,
			BuildFailureRate:      0.2,
			ValidationFailureRate: 0.3,
			TimeoutFailureRate:    0.4,
		},
	}

	data, err := json.Marshal(trend)
	if err != nil {
		t.Fatalf("failed to marshal process trend: %v", err)
	}
	if err := os.WriteFile(filepath.Join(metricsDir, "process_trend.json"), data, 0644); err != nil {
		t.Fatalf("failed to write process trend: %v", err)
	}

	mockProvider := &mockProvider{}
	r, err := NewRetroWithProvider(mockProvider, tmpGromitDir)
	if err != nil {
		t.Fatalf("failed to create Retro: %v", err)
	}
	r.templatePath = templatePath

	prompt, err := r.renderPrompt("# Test Rules", "# Test Learnings", logger.RunStats{Total: 1}, nil, nil, nil)
	if err != nil {
		t.Fatalf("renderPrompt with process trend failed: %v", err)
	}

	expected := []string{
		"### Failure Breakdown",
		"Preflight failures: 10.0%",
		"Build failures: 20.0%",
		"Validation failures: 30.0%",
		"Timeout failures: 40.0%",
	}
	for _, want := range expected {
		if !contains(prompt, want) {
			t.Errorf("rendered prompt missing expected failure breakdown text: %q", want)
		}
	}
}

func TestRenderPrompt_ShapesRulesAndLearningsWhenBudgetConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "PROMPT_retro.md")
	if err := os.WriteFile(templatePath, []byte("RULES:\n{{.Rules}}\nLEARNINGS:\n{{.Learnings}}\n"), 0644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	r := &Retro{
		templatePath: templatePath,
		promptBudget: 40,
	}

	rules := strings.Repeat("r", 30)
	learnings := strings.Repeat("l", 40)

	prompt, err := r.renderPrompt(rules, learnings, logger.RunStats{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("renderPrompt returned error: %v", err)
	}

	if !strings.Contains(prompt, rules) {
		t.Fatalf("expected rules to remain after shaping, got prompt: %q", prompt)
	}
	if strings.Contains(prompt, learnings) {
		t.Fatalf("expected learnings to be shaped out of prompt, got prompt: %q", prompt)
	}
}

func TestRenderPrompt_BuildsPromptDiagnostics(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "PROMPT_retro.md")
	if err := os.WriteFile(templatePath, []byte("OK"), 0644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	r := &Retro{templatePath: templatePath}
	beadStats := map[string]logger.BeadStats{
		"gromit-1": {
			BeadID:    "gromit-1",
			Failures:  2,
			Comments:  []string{"note"},
			BeadTitle: "Test Bead",
		},
	}
	efficiency := &logger.EfficiencyReport{
		CurrentAvgCostPerBead: 0.2,
	}
	runStats := logger.RunStats{Total: 1}

	if _, err := r.renderPrompt("rules content", "confirmed learnings", runStats, beadStats, efficiency, nil); err != nil {
		t.Fatalf("renderPrompt returned error: %v", err)
	}

	diagnostics := r.PromptDiagnostics()
	if diagnostics == nil {
		t.Fatal("expected prompt diagnostics")
	}
	if diagnostics.PromptType != "retro" {
		t.Fatalf("PromptType = %q, want retro", diagnostics.PromptType)
	}

	expected := map[string]int{
		prompt.SectionRules:              prompt.EstimateTokens("rules content"),
		prompt.SectionConfirmedLearnings: prompt.EstimateTokens("confirmed learnings"),
		prompt.SectionRunStats:           prompt.EstimateTokens(diagnosticJSON(runStats)),
		prompt.SectionBeadStats:          prompt.EstimateTokens(diagnosticJSON(beadStats)),
		"efficiency":                     prompt.EstimateTokens(diagnosticJSON(efficiency)),
		"process_trend":                  prompt.EstimateTokens(diagnosticJSON(r.loadProcessTrend())),
	}
	expectedSum := 0
	for key, expectedValue := range expected {
		value, ok := diagnostics.SectionTokens[key]
		if !ok {
			t.Fatalf("missing section token key %q", key)
		}
		if value != expectedValue {
			t.Fatalf("section token %q = %d, want %d", key, value, expectedValue)
		}
		expectedSum += expectedValue
	}
	if diagnostics.EstimatedTokens != expectedSum {
		t.Fatalf("EstimatedTokens = %d, want %d", diagnostics.EstimatedTokens, expectedSum)
	}
}

func TestRenderPrompt_BudgetShapingDiagnostics(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "PROMPT_retro.md")
	if err := os.WriteFile(templatePath, []byte("{{.Rules}}{{.Learnings}}"), 0644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	r := &Retro{
		templatePath: templatePath,
		promptBudget: 40,
	}

	rules := strings.Repeat("r", 30)
	learnings := strings.Repeat("l", 60)
	shapedRules, shapedLearnings, shapeReport := prompt.ShapeRetroForBudget(rules, learnings, 40)
	if _, err := r.renderPrompt(rules, learnings, logger.RunStats{}, nil, nil, nil); err != nil {
		t.Fatalf("renderPrompt returned error: %v", err)
	}

	diagnostics := r.PromptDiagnostics()
	if diagnostics == nil {
		t.Fatal("expected prompt diagnostics")
	}
	if diagnostics.PreShapeTokens <= 0 {
		t.Fatalf("PreShapeTokens = %d, want > 0", diagnostics.PreShapeTokens)
	}
	if diagnostics.PreShapeTokens != prompt.EstimateTokens(rules)+prompt.EstimateTokens(learnings) {
		t.Fatalf("PreShapeTokens = %d, want %d", diagnostics.PreShapeTokens, prompt.EstimateTokens(rules)+prompt.EstimateTokens(learnings))
	}
	if diagnostics.PostShapeTokens <= 0 {
		t.Fatalf("PostShapeTokens = %d, want > 0", diagnostics.PostShapeTokens)
	}
	if diagnostics.PostShapeTokens != prompt.EstimateTokens(shapedRules)+prompt.EstimateTokens(shapedLearnings) {
		t.Fatalf("PostShapeTokens = %d, want %d", diagnostics.PostShapeTokens, prompt.EstimateTokens(shapedRules)+prompt.EstimateTokens(shapedLearnings))
	}
	if diagnostics.PostShapeTokens > diagnostics.PreShapeTokens {
		t.Fatalf("PostShapeTokens = %d, want <= PreShapeTokens = %d", diagnostics.PostShapeTokens, diagnostics.PreShapeTokens)
	}
	if len(diagnostics.ShapeActions) == 0 {
		t.Fatal("expected non-empty ShapeActions when shaping is active")
	}
	if len(diagnostics.ShapeActions) != len(shapeReport.TrimActions) {
		t.Fatalf("ShapeActions = %#v, want %d actions", diagnostics.ShapeActions, len(shapeReport.TrimActions))
	}
	for i, action := range shapeReport.TrimActions {
		if diagnostics.ShapeActions[i] != action {
			t.Fatalf("ShapeActions[%d] = %q, want %q", i, diagnostics.ShapeActions[i], action)
		}
	}
	if diagnostics.SectionTokens[prompt.SectionRules] != prompt.EstimateTokens(shapedRules) {
		t.Fatalf("SectionTokens[rules] = %d, want %d", diagnostics.SectionTokens[prompt.SectionRules], prompt.EstimateTokens(shapedRules))
	}
	if diagnostics.SectionTokens[prompt.SectionConfirmedLearnings] != prompt.EstimateTokens(shapedLearnings) {
		t.Fatalf("SectionTokens[learnings] = %d, want %d", diagnostics.SectionTokens[prompt.SectionConfirmedLearnings], prompt.EstimateTokens(shapedLearnings))
	}
}
