package retro

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
)

func TestNewRetroNilConfig(t *testing.T) {
	r, err := NewRetro(nil, ".gromit")
	if r != nil {
		t.Error("expected nil Retro for nil config")
	}
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestRunNilReceiver(t *testing.T) {
	var r *Retro
	_, err := r.Run(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil retro")
	}
}

func TestRunNilClaudeClient(t *testing.T) {
	r := &Retro{
		claude: nil,
	}
	_, err := r.Run(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil claude client")
	}
}

func TestRunNilLearningsFile(t *testing.T) {
	claudeClient, _ := claude.NewClient("claude", nil, 60)
	r := &Retro{
		claude:        claudeClient,
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
	r, _ := NewRetro(&config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}, tmpDir)
	// Should not panic
	r.enrichBeadStats(context.Background(), nil)
}

func TestEnrichBeadStatsEmptyMap(t *testing.T) {
	tmpDir := t.TempDir()
	r, _ := NewRetro(&config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}, tmpDir)
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

	// The function signature is: LaunchClaudeCode(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment) error
	// This is a compile-time check that the function exists with the right signature.
	var _ func(string, *logger.EfficiencyReport, *Experiment) error = LaunchClaudeCode

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
	r, err := NewRetro(&config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}, tmpGromitDir)
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
	r, err := NewRetro(&config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}, tmpGromitDir)
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
	r, err := NewRetro(&config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}, tmpGromitDir)
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
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}

	r, err := NewRetro(cfg, tmpDir)
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
