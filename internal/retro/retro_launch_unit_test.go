package retro

import (
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/logger"
)

func TestLaunchClaudeCode_PassesDirAndPromptToRunner(t *testing.T) {
	original := runInteractiveClaude
	t.Cleanup(func() { runInteractiveClaude = original })

	analysis := "unit-test analysis"
	var gotPrompt string
	var gotDir string
	runInteractiveClaude = func(promptText, dir string, stdin io.Reader, stdout, stderr io.Writer) error {
		gotPrompt = promptText
		gotDir = dir
		return nil
	}

	if err := LaunchClaudeCode(analysis, nil, nil, "/tmp/worktree"); err != nil {
		t.Fatalf("LaunchClaudeCode() error = %v", err)
	}

	if gotDir != "/tmp/worktree" {
		t.Fatalf("dir = %q, want %q", gotDir, "/tmp/worktree")
	}
	if !strings.Contains(gotPrompt, analysis) {
		t.Fatalf("prompt does not include analysis text: %q", gotPrompt)
	}
}

func TestBuildClaudeCodePrompt_IncludesEfficiencyAndExperimentSections(t *testing.T) {
	efficiency := &logger.EfficiencyReport{
		CurrentAvgCostPerBead:    1.5,
		HistoricalAvgCostPerBead: 2.0,
		CostDelta:                -0.5,
	}
	experiment := &Experiment{
		Name:       "Test Experiment",
		Hypothesis: "Test hypothesis",
		Change:     "Test change",
	}

	prompt := buildClaudeCodePrompt("analysis", efficiency, experiment)

	if !strings.Contains(prompt, "# Efficiency Analysis") {
		t.Fatalf("prompt missing efficiency section: %q", prompt)
	}
	if !strings.Contains(prompt, "# Active Experiment Evaluation") {
		t.Fatalf("prompt missing experiment section: %q", prompt)
	}
	if !strings.Contains(prompt, "Test Experiment") {
		t.Fatalf("prompt missing experiment name: %q", prompt)
	}
}
