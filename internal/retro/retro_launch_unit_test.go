package retro

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func TestRunInteractiveClaude_UsesPromptFile(t *testing.T) {
	tmpDir := t.TempDir()
	claudePath := filepath.Join(tmpDir, "claude")
	argPath := filepath.Join(tmpDir, "arg.txt")
	promptPath := filepath.Join(tmpDir, "prompt.txt")

	script := `#!/bin/sh
set -e
printf '%s' "$1" > "$CAPTURE_ARG"
case "$1" in
  "Read and follow instructions in "*)
    path="${1#Read and follow instructions in }"
    cat "$path" > "$CAPTURE_PROMPT"
    ;;
  *)
    : > "$CAPTURE_PROMPT"
    ;;
esac
`
	if err := os.WriteFile(claudePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write claude script: %v", err)
	}

	t.Setenv("PATH", fmt.Sprintf("%s:%s", tmpDir, os.Getenv("PATH")))
	t.Setenv("CAPTURE_ARG", argPath)
	t.Setenv("CAPTURE_PROMPT", promptPath)

	prompt := "retro prompt text"
	if err := runInteractiveClaude(prompt, "", strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatalf("runInteractiveClaude error: %v", err)
	}

	arg, err := os.ReadFile(argPath)
	if err != nil {
		t.Fatalf("read captured arg: %v", err)
	}
	gotArg := string(arg)
	const prefix = "Read and follow instructions in "
	if !strings.HasPrefix(gotArg, prefix) {
		t.Fatalf("arg = %q, want prefix %q", gotArg, prefix)
	}

	gotPrompt, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read captured prompt: %v", err)
	}
	if string(gotPrompt) != prompt {
		t.Fatalf("prompt = %q, want %q", string(gotPrompt), prompt)
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

	prompt := BuildClaudeCodePrompt("analysis", efficiency, experiment)

	if !strings.Contains(prompt, "# Efficiency Analysis") {
		t.Fatalf("prompt missing efficiency section: %q", prompt)
	}
	if !strings.Contains(prompt, "# Active Experiment Evaluation") {
		t.Fatalf("prompt missing experiment section: %q", prompt)
	}
	if !strings.Contains(prompt, "Test Experiment") {
		t.Fatalf("prompt missing experiment name: %q", prompt)
	}
	if !strings.Contains(prompt, "Never set or change experiment decisions without explicit user approval") {
		t.Fatalf("prompt missing explicit-approval guardrail for experiment decisions: %q", prompt)
	}
}

func TestBuildClaudeCodePrompt_SkipsCostDeltaPercentWhenHistoricalCostIsZero(t *testing.T) {
	efficiency := &logger.EfficiencyReport{
		CurrentAvgCostPerBead:    1.5,
		HistoricalAvgCostPerBead: 0,
		CostDelta:                0.5,
	}

	prompt := BuildClaudeCodePrompt("analysis", efficiency, nil)

	if !strings.Contains(prompt, "- Historical avg cost per bead: $0.0000") {
		t.Fatalf("prompt missing historical avg cost line: %q", prompt)
	}
	if strings.Contains(prompt, "- Cost delta:") {
		t.Fatalf("prompt should not include cost delta percentage when historical avg is zero: %q", prompt)
	}
}
