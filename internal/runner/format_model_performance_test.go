package runner

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/logger"
)

func TestFormatModelPerformance_NilStats(t *testing.T) {
	got := formatModelPerformance(nil)

	// Should show a message indicating no data
	if !strings.Contains(got, "Model Performance") {
		t.Errorf("formatModelPerformance(nil) should contain header, got:\n%s", got)
	}
}

func TestFormatModelPerformance_EmptyStats(t *testing.T) {
	stats := make(map[string]logger.ModelStats)

	got := formatModelPerformance(stats)

	// Should show header but indicate no data
	if !strings.Contains(got, "Model Performance") {
		t.Errorf("formatModelPerformance(empty) should contain header, got:\n%s", got)
	}
}

func TestFormatModelPerformance_SingleModel(t *testing.T) {
	stats := map[string]logger.ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   10,
			Successes:    9,
			Failures:     1,
			TotalCostUSD: 20.40,
		},
	}

	got := formatModelPerformance(stats)

	want := []string{
		"Model Performance",
		"opus",
		"90%",        // success rate
		"(9/10)",     // success/total
		"$2.04/iter", // avg cost per iteration
	}

	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("formatModelPerformance() missing %q\ngot:\n%s", w, got)
		}
	}
}

func TestFormatModelPerformance_MultipleModels(t *testing.T) {
	stats := map[string]logger.ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   11,
			Successes:    10,
			Failures:     1,
			TotalCostUSD: 22.44,
		},
		"sonnet": {
			Model:        "sonnet",
			Iterations:   8,
			Successes:    3,
			Failures:     5,
			TotalCostUSD: 3.68,
		},
		"haiku": {
			Model:        "haiku",
			Iterations:   8,
			Successes:    6,
			Failures:     2,
			TotalCostUSD: 0.96,
		},
	}

	got := formatModelPerformance(stats)

	// Check that all models appear
	models := []string{"opus", "sonnet", "haiku"}
	for _, model := range models {
		if !strings.Contains(got, model) {
			t.Errorf("formatModelPerformance() missing model %q\ngot:\n%s", model, got)
		}
	}

	// Check opus stats (91% success rate)
	opusChecks := []string{
		"opus",
		"91%",
		"(10/11)",
		"$2.04/iter",
	}
	for _, check := range opusChecks {
		if !strings.Contains(got, check) {
			t.Errorf("formatModelPerformance() missing opus stat %q\ngot:\n%s", check, got)
		}
	}

	// Check sonnet stats (38% success rate)
	sonnetChecks := []string{
		"sonnet",
		"38%",
		"(3/8)",
		"$0.46/iter",
	}
	for _, check := range sonnetChecks {
		if !strings.Contains(got, check) {
			t.Errorf("formatModelPerformance() missing sonnet stat %q\ngot:\n%s", check, got)
		}
	}

	// Check haiku stats (75% success rate)
	haikuChecks := []string{
		"haiku",
		"75%",
		"(6/8)",
		"$0.12/iter",
	}
	for _, check := range haikuChecks {
		if !strings.Contains(got, check) {
			t.Errorf("formatModelPerformance() missing haiku stat %q\ngot:\n%s", check, got)
		}
	}
}

func TestFormatModelPerformance_ZeroIterations(t *testing.T) {
	stats := map[string]logger.ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   0,
			Successes:    0,
			Failures:     0,
			TotalCostUSD: 0.0,
		},
	}

	got := formatModelPerformance(stats)

	// Should handle zero iterations gracefully without division by zero
	if !strings.Contains(got, "opus") {
		t.Errorf("formatModelPerformance() should contain model name even with 0 iterations\ngot:\n%s", got)
	}
}

func TestFormatModelPerformance_PerfectSuccessRate(t *testing.T) {
	stats := map[string]logger.ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   5,
			Successes:    5,
			Failures:     0,
			TotalCostUSD: 10.00,
		},
	}

	got := formatModelPerformance(stats)

	want := []string{
		"opus",
		"100%",
		"(5/5)",
		"$2.00/iter",
	}

	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("formatModelPerformance() missing %q for perfect success\ngot:\n%s", w, got)
		}
	}
}

func TestFormatModelPerformance_ZeroSuccessRate(t *testing.T) {
	stats := map[string]logger.ModelStats{
		"sonnet": {
			Model:        "sonnet",
			Iterations:   4,
			Successes:    0,
			Failures:     4,
			TotalCostUSD: 1.00,
		},
	}

	got := formatModelPerformance(stats)

	want := []string{
		"sonnet",
		"0%",
		"(0/4)",
		"$0.25/iter",
	}

	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("formatModelPerformance() missing %q for zero success\ngot:\n%s", w, got)
		}
	}
}

func TestFormatModelPerformance_FormattingConsistency(t *testing.T) {
	stats := map[string]logger.ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   10,
			Successes:    9,
			Failures:     1,
			TotalCostUSD: 20.40,
		},
	}

	got := formatModelPerformance(stats)

	// Check that output follows section-based formatting pattern (consistent with other formatters)
	if !strings.HasPrefix(strings.TrimSpace(got), "Model Performance") {
		t.Errorf("formatModelPerformance() should start with 'Model Performance' header, got:\n%s", got)
	}

	// Check for proper indentation (should use 2 spaces based on format.go patterns)
	lines := strings.Split(got, "\n")
	foundIndentedLine := false
	for _, line := range lines {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			foundIndentedLine = true
			break
		}
	}

	if !foundIndentedLine {
		t.Errorf("formatModelPerformance() should have lines with 2-space indentation, got:\n%s", got)
	}
}

func TestFormatModelPerformance_CostFormatting(t *testing.T) {
	tests := []struct {
		name         string
		totalCost    float64
		iterations   int
		expectedCost string
	}{
		{
			name:         "dollars and cents",
			totalCost:    20.40,
			iterations:   10,
			expectedCost: "$2.04/iter",
		},
		{
			name:         "cents only",
			totalCost:    0.96,
			iterations:   8,
			expectedCost: "$0.12/iter",
		},
		{
			name:         "large cost",
			totalCost:    100.00,
			iterations:   10,
			expectedCost: "$10.00/iter",
		},
		{
			name:         "fractional cents",
			totalCost:    1.00,
			iterations:   3,
			expectedCost: "$0.33/iter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := map[string]logger.ModelStats{
				"opus": {
					Model:        "opus",
					Iterations:   tt.iterations,
					Successes:    tt.iterations,
					Failures:     0,
					TotalCostUSD: tt.totalCost,
				},
			}

			got := formatModelPerformance(stats)

			if !strings.Contains(got, tt.expectedCost) {
				t.Errorf("formatModelPerformance() missing cost %q\ngot:\n%s", tt.expectedCost, got)
			}
		})
	}
}

func TestFormatModelPerformance_ModelOrdering(t *testing.T) {
	stats := map[string]logger.ModelStats{
		"haiku": {
			Model:        "haiku",
			Iterations:   5,
			Successes:    4,
			Failures:     1,
			TotalCostUSD: 0.50,
		},
		"opus": {
			Model:        "opus",
			Iterations:   5,
			Successes:    4,
			Failures:     1,
			TotalCostUSD: 10.00,
		},
		"sonnet": {
			Model:        "sonnet",
			Iterations:   5,
			Successes:    4,
			Failures:     1,
			TotalCostUSD: 2.50,
		},
	}

	got := formatModelPerformance(stats)

	// Models should appear in a consistent order (likely alphabetical or by tier: opus, sonnet, haiku)
	// Find positions of each model name
	opusPos := strings.Index(got, "opus")
	sonnetPos := strings.Index(got, "sonnet")
	haikuPos := strings.Index(got, "haiku")

	if opusPos == -1 || sonnetPos == -1 || haikuPos == -1 {
		t.Fatalf("formatModelPerformance() missing one or more models\ngot:\n%s", got)
	}

	// Verify consistent ordering (opus < sonnet < haiku by tier, or alphabetical haiku < opus < sonnet)
	// The spec doesn't mandate order, but consistency is important
	// We'll verify they appear in one of the two sensible orders
	tierOrder := opusPos < sonnetPos && sonnetPos < haikuPos
	alphaOrder := haikuPos < opusPos && opusPos < sonnetPos

	if !tierOrder && !alphaOrder {
		t.Errorf("formatModelPerformance() models should appear in consistent order (tier or alphabetical)\npositions: opus=%d sonnet=%d haiku=%d\ngot:\n%s",
			opusPos, sonnetPos, haikuPos, got)
	}
}
