package retro

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
)

// Retro manages retrospective analysis
type Retro struct {
	cfg            *config.Config
	claude         *claude.Client
	learningsFile  *learnings.File
	rulesPath      string
	templatePath   string
	experimentPath string
}

// TemplateContext holds data for retro prompt template
type TemplateContext struct {
	Rules      string
	Learnings  string
	RunStats   logger.RunStats
	BeadStats  map[string]logger.BeadStats
	Efficiency *logger.EfficiencyReport
	Experiment *Experiment
}

// Result represents the outcome of a retro analysis
type Result struct {
	Analysis      string
	ProposedRules string
	Success       bool
	Efficiency    *logger.EfficiencyReport
	Experiment    *Experiment
}

// NewRetro creates a new retrospective analyzer
func NewRetro(cfg *config.Config, gromitDir string) (*Retro, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return nil, err
	}
	learningsFile, err := learnings.NewFile(gromitDir)
	if err != nil {
		return nil, err
	}
	return &Retro{
		cfg:            cfg,
		claude:         claudeClient,
		learningsFile:  learningsFile,
		rulesPath:      filepath.Join(gromitDir, "RULES.md"),
		templatePath:   filepath.Join(gromitDir, "templates", "PROMPT_retro.md"),
		experimentPath: filepath.Join(gromitDir, "experiment.json"),
	}, nil
}

// Run executes the retrospective analysis
func (r *Retro) Run(ctx context.Context) (*Result, error) {
	if r == nil {
		return nil, fmt.Errorf("retro is nil")
	}
	if r.claude == nil {
		return nil, fmt.Errorf("claude client is nil")
	}
	// Load learnings
	if r.learningsFile == nil {
		return nil, fmt.Errorf("learnings file is nil")
	}
	if err := r.learningsFile.Load(); err != nil {
		return nil, fmt.Errorf("loading learnings: %w", err)
	}

	// Load rules
	rules, err := r.loadRules()
	if err != nil {
		return nil, fmt.Errorf("loading rules: %w", err)
	}

	// Format learnings for prompt
	learningsText := r.formatLearnings()

	// Load run stats and per-bead stats
	logsDir := filepath.Join(filepath.Dir(r.rulesPath), "logs")
	runStats, _ := logger.ReadAllLogs(logsDir)
	allBeadStats, _ := logger.ReadPerBeadStats(logsDir)

	// Load efficiency report (current run is empty string, all runs are historical)
	efficiencyReport, _ := logger.ReadEfficiencyReport(logsDir, "")

	// Load active experiment (if any)
	experiment, _ := LoadExperiment(r.experimentPath)

	// Filter per-bead stats to only include beads with >= 2 failures
	filteredBeadStats := make(map[string]logger.BeadStats)
	for id, stats := range allBeadStats {
		if stats.Failures >= 2 {
			filteredBeadStats[id] = stats
		}
	}

	// Enrich bead stats with status, close reason, and comments from bd
	r.enrichBeadStats(ctx, filteredBeadStats)

	// Filter out closed beads from the stuck list
	for id, stats := range filteredBeadStats {
		if stats.Status == "closed" {
			delete(filteredBeadStats, id)
		}
	}

	// Render prompt
	prompt, err := r.renderPrompt(rules, learningsText, runStats, filteredBeadStats, efficiencyReport, experiment)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	// Run Claude analysis (use opus for quality analysis)
	model := "opus"
	claudeResult, err := r.claude.Run(ctx, prompt, model)
	if err != nil {
		return nil, fmt.Errorf("running Claude analysis: %w", err)
	}

	if claudeResult == nil {
		return &Result{Success: false}, nil
	}

	result := &Result{
		Analysis:   claudeResult.Output,
		Success:    claudeResult.Success,
		Efficiency: efficiencyReport,
		Experiment: experiment,
	}

	return result, nil
}

// loadRules reads the RULES.md file
func (r *Retro) loadRules() (string, error) {
	content, err := os.ReadFile(r.rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading rules file: %w", err)
	}
	return string(content), nil
}

// formatLearnings formats learnings into a readable string
func (r *Retro) formatLearnings() string {
	var sb strings.Builder

	confirmed := r.learningsFile.GetConfirmed()
	provisional := r.learningsFile.GetProvisional()

	sb.WriteString("## Confirmed Learnings\n\n")
	if len(confirmed) == 0 {
		sb.WriteString("*No confirmed learnings yet.*\n\n")
	} else {
		for _, l := range confirmed {
			sb.WriteString(fmt.Sprintf("**%s | %s | %s | Hash: `%s`**\n",
				l.Date.Format("2006-01-02"),
				l.BeadID,
				l.Category,
				l.Hash,
			))
			if l.RelatedTo != "" {
				sb.WriteString(fmt.Sprintf("*Related to: %s*\n", l.RelatedTo))
			}
			sb.WriteString(l.Content)
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("## Provisional Learnings\n\n")
	if len(provisional) == 0 {
		sb.WriteString("*No provisional learnings.*\n\n")
	} else {
		for _, l := range provisional {
			sb.WriteString(fmt.Sprintf("**%s | %s | %s | Hash: `%s`**\n",
				l.Date.Format("2006-01-02"),
				l.BeadID,
				l.Category,
				l.Hash,
			))
			if l.RelatedTo != "" {
				sb.WriteString(fmt.Sprintf("*Related to: %s*\n", l.RelatedTo))
			}
			sb.WriteString(l.Content)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// renderPrompt renders the retro prompt template
func (r *Retro) renderPrompt(rules, learnings string, runStats logger.RunStats, beadStats map[string]logger.BeadStats, efficiency *logger.EfficiencyReport, experiment *Experiment) (string, error) {
	tmplContent, err := os.ReadFile(r.templatePath)
	if err != nil {
		return "", fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New("retro").Funcs(template.FuncMap{
		"mul": func(a, b float64) float64 { return a * b },
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"sub": func(a, b float64) float64 { return a - b },
		"durationMs": func(d time.Duration) float64 {
			return float64(d.Milliseconds())
		},
	}).Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	ctx := TemplateContext{
		Rules:      rules,
		Learnings:  learnings,
		RunStats:   runStats,
		BeadStats:  beadStats,
		Efficiency: efficiency,
		Experiment: experiment,
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, ctx); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return sb.String(), nil
}

// enrichBeadStats populates Status, CloseReason, and Comments fields on BeadStats
// by calling bd show for each bead. Errors are logged as warnings and do not stop enrichment.
func (r *Retro) enrichBeadStats(ctx context.Context, beadStats map[string]logger.BeadStats) {
	if r == nil || beadStats == nil {
		return
	}

	client, err := bead.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create bead client for enrichment: %v\n", err)
		return
	}

	for beadID, stats := range beadStats {
		// Get full bead details
		b, err := client.Show(beadID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get details for bead %s: %v\n", beadID, err)
			continue
		}

		// Populate status and close reason
		stats.Status = b.Status
		stats.CloseReason = b.CloseReason

		// Get comments
		comments, err := client.GetComments(beadID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get comments for bead %s: %v\n", beadID, err)
			stats.Comments = []string{}
		} else {
			// Extract comment text into a slice
			commentTexts := make([]string, len(comments))
			for i, c := range comments {
				commentTexts[i] = c.Text
			}
			stats.Comments = commentTexts
		}

		// Update the map entry
		beadStats[beadID] = stats
	}
}

// LaunchClaudeCode launches Claude Code in interactive mode with the analysis results.
// The prompt instructs Claude Code on what actions it can take:
// - Edit RULES.md
// - Edit LEARNINGS.md
// - Run bd commands
// - Create specs
// - Select and persist experiments
func LaunchClaudeCode(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment) error {
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
		if efficiency.CostDelta != 0 {
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

		prompt.WriteString("The retro analysis above includes a comparison of current metrics against the baseline. ")
		prompt.WriteString("You need to decide whether to:\n\n")
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
	prompt.WriteString("- When selecting an experiment, populate baseline metrics using ComputeBaselineMetrics from the retro package\n")
	prompt.WriteString("- Experiment recommendations should focus on concrete, testable process changes\n")
	prompt.WriteString("- Apply Five Whys analysis when investigating efficiency anomalies to identify root causes\n")
	prompt.WriteString("\nPlease review the analysis and take appropriate actions.\n")

	// Launch claude binary in interactive mode
	// Note: We don't use -p flag here since we want interactive mode
	cmd := exec.Command("claude")

	// Connect stdin/stdout/stderr to the parent process for full interactivity
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set the prompt as a positional argument
	cmd.Args = append(cmd.Args, prompt.String())

	// Run and wait for completion
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running claude: %w", err)
	}

	return nil
}
