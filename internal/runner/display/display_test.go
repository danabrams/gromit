package display

import (
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

func TestFormatRun_nilStatus(t *testing.T) {
	t.Parallel()

	got := FormatRun(nil)
	if !strings.Contains(got, "Run: not running") {
		t.Fatalf("FormatRun(nil) = %q, want substring %q", got, "Run: not running")
	}
}

func TestFormatRun_runningStatus(t *testing.T) {
	t.Parallel()

	status := &RunStatus{
		Running:   true,
		Iteration: 3,
		BeadID:    "beads-abc",
		BeadTitle: "Add feature X",
		Model:     "sonnet",
		ElapsedS:  120,
	}

	got := FormatRun(status)
	for _, substr := range []string{
		"Run: iteration 3",
		"2m elapsed",
		"beads-abc",
		"Add feature X",
		"Model:    sonnet",
	} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatRun() = %q, want substring %q", got, substr)
		}
	}
}

func TestFormatHealth_defaults(t *testing.T) {
	t.Parallel()

	got := FormatHealth(time.Time{}, 0)
	if !strings.Contains(got, "Health:") {
		t.Fatalf("FormatHealth() = %q, want substring %q", got, "Health:")
	}
}

func TestFormatPipeline_basic(t *testing.T) {
	t.Parallel()

	got := FormatPipeline(&pipeline.PipelineStatus{})
	if !strings.Contains(got, "Pipeline:") {
		t.Fatalf("FormatPipeline() = %q, want substring %q", got, "Pipeline:")
	}
}

func TestFormatRecommendation_hint(t *testing.T) {
	t.Parallel()

	got := FormatRecommendation("Refine backlog ideas")
	want := "Next action: Refine backlog ideas (gromit refine)"
	if got != want {
		t.Fatalf("FormatRecommendation() = %q, want %q", got, want)
	}
}

func TestFormatModelPerformance_singleModel(t *testing.T) {
	t.Parallel()

	stats := map[string]logger.ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   10,
			Successes:    9,
			Failures:     1,
			TotalCostUSD: 20.40,
		},
	}
	got := FormatModelPerformance(stats)
	for _, substr := range []string{"Model Performance", "opus", "90%", "(9/10)", "$2.04/iter"} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatModelPerformance() = %q, want substring %q", got, substr)
		}
	}
}
