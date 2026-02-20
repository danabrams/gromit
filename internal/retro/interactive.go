package retro

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/danabrams/gromit/internal/logger"
)

const fileRefMessageFormat = "Read and follow instructions in %s"

var execCommand = exec.Command

var runInteractiveClaude = func(promptText, dir string, stdin io.Reader, stdout, stderr io.Writer) error {
	promptFile, err := os.CreateTemp("", "retro-prompt-*.md")
	if err != nil {
		return fmt.Errorf("creating temp prompt file: %w", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(promptText); err != nil {
		promptFile.Close()
		return fmt.Errorf("writing prompt file: %w", err)
	}
	if err := promptFile.Close(); err != nil {
		return fmt.Errorf("closing prompt file: %w", err)
	}

	// Launch claude binary in interactive mode.
	cmd := execCommand("claude", fmt.Sprintf(fileRefMessageFormat, promptPath))
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if dir != "" {
		cmd.Dir = dir
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running claude: %w", err)
	}
	return nil
}

// LaunchClaudeCode launches Claude Code in interactive mode with the analysis results.
// The prompt instructs Claude Code on what actions it can take:
// - Edit RULES.md
// - Edit LEARNINGS.md
// - Run bd commands
// - Create specs
// - Select and persist experiments
func LaunchClaudeCode(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment, dir string) error {
	promptText := BuildClaudeCodePrompt(analysis, efficiency, experiment)
	return runInteractiveClaude(promptText, dir, os.Stdin, os.Stdout, os.Stderr)
}

// BuildClaudeCodePrompt builds the prompt for the interactive retro review session.
func BuildClaudeCodePrompt(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment) string {
	// Build the prompt with analysis and instructions
	var prompt strings.Builder

	prompt.WriteString("# Retrospective Analysis Results\n\n")
	prompt.WriteString(analysis)
	prompt.WriteString("\n\n")

	// Add efficiency analysis section if available
	if efficiency != nil {
		prompt.WriteString("# Efficiency Analysis\n\n")
		prompt.WriteString("The retro analysis above includes detailed efficiency metrics comparing the current run against historical data. ")
		prompt.WriteString("When investigating efficiency anomalies, apply **Five Whys analysis** to trace surface symptoms ")
		prompt.WriteString("(e.g., \"this bead cost $3\") to root causes (e.g., \"the acceptance criteria were ambiguous, causing opus escalation\").\n\n")

		prompt.WriteString("**Key efficiency indicators:**\n")
		prompt.WriteString(fmt.Sprintf("- Current avg cost per bead: $%.4f\n", efficiency.CurrentAvgCostPerBead))
		prompt.WriteString(fmt.Sprintf("- Historical avg cost per bead: $%.4f\n", efficiency.HistoricalAvgCostPerBead))
		if efficiency.CostDelta != 0 && efficiency.HistoricalAvgCostPerBead != 0 {
			direction := "more expensive"
			if efficiency.CostDelta < 0 {
				direction = "cheaper"
			}
			pct := (efficiency.CostDelta / efficiency.HistoricalAvgCostPerBead) * 100
			prompt.WriteString(fmt.Sprintf("- Cost delta: $%.4f (%+.1f%% %s)\n", efficiency.CostDelta, pct, direction))
		}
		prompt.WriteString("\n")
	}

	// Add experiment evaluation section if an experiment is active
	if experiment != nil {
		prompt.WriteString("# Active Experiment Evaluation\n\n")
		prompt.WriteString(fmt.Sprintf("**Experiment:** %s\n\n", experiment.Name))
		prompt.WriteString(fmt.Sprintf("**Hypothesis:** %s\n\n", experiment.Hypothesis))
		prompt.WriteString(fmt.Sprintf("**Change:** %s\n\n", experiment.Change))
		prompt.WriteString(fmt.Sprintf("**Started:** %s\n\n", experiment.StartedAt.Format("2006-01-02")))
		if experiment.ID != "" {
			prompt.WriteString(fmt.Sprintf("**Experiment ID:** %s\n\n", experiment.ID))
		}

		prompt.WriteString("The retro analysis above includes a comparison of current metrics against the baseline. ")
		prompt.WriteString("Work with the user to decide whether to:\n\n")
		prompt.WriteString("1. **Keep** - The experiment was successful; integrate the change as standard practice (delete experiment.json)\n")
		prompt.WriteString("2. **Revert** - The experiment didn't work; undo the change and delete experiment.json\n")
		prompt.WriteString("3. **Extend** - Need more data; keep experiment.json for another cycle\n\n")
	}

	// Add instructions
	prompt.WriteString("# What You Can Do\n\n")
	prompt.WriteString("You are now in an interactive Claude Code session. Based on the analysis above, you can:\n\n")
	prompt.WriteString("1. **Edit RULES.md** - Update project rules based on learnings\n")
	prompt.WriteString("2. **Edit LEARNINGS.md** - Consolidate, archive, or promote learnings\n")
	prompt.WriteString("3. **Run bd commands** - Create new beads, update existing ones, or manage the backlog\n")
	prompt.WriteString("4. **Create specs** - Write detailed specifications in .gromit/specs/ for complex features\n")

	// Add experiment selection guidance
	if efficiency != nil || experiment != nil {
		prompt.WriteString("5. **Manage experiments**:\n")

		if experiment != nil {
			prompt.WriteString("   - An active experiment is being evaluated - decide whether to keep, revert, or extend it\n")
			prompt.WriteString("   - Get explicit user approval before setting/changing act_decision, and re-confirm on interactive retries\n")
			prompt.WriteString("   - Persist explicit PDSA fields: status, study_summary, act_decision, and act_date in .gromit/experiment.json\n")
		}

		if efficiency != nil && experiment == nil {
			prompt.WriteString("   - Select at most ONE experiment from the recommendations above (or none)\n")
			prompt.WriteString("   - Use retro.SaveExperiment() to persist your selection to .gromit/experiment.json\n")
			prompt.WriteString("   - Populate baseline metrics using retro.ComputeBaselineMetrics()\n")
		}

		prompt.WriteString("\n")
	}

	prompt.WriteString("\n**Important constraints:**\n")
	prompt.WriteString("- Only select ONE experiment at a time (never multiple)\n")
	prompt.WriteString("- Never set or change experiment decisions without explicit user approval in this session; if retried, re-confirm approval before finalizing\n")
	prompt.WriteString("- When selecting an experiment, populate baseline metrics using ComputeBaselineMetrics from the retro package\n")
	prompt.WriteString("- Experiment recommendations should focus on concrete, testable process changes\n")
	prompt.WriteString("- Apply Five Whys analysis when investigating efficiency anomalies to identify root causes\n")
	prompt.WriteString("\nPlease review the analysis and take appropriate actions.\n")
	return prompt.String()
}
