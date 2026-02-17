package retro

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/state"
)

var runInteractiveClaude = func(promptText, dir string, stdin io.Reader, stdout, stderr io.Writer) error {
	// Launch claude binary in interactive mode.
	cmd := exec.Command("claude")
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Args = append(cmd.Args, promptText)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running claude: %w", err)
	}
	return nil
}

const (
	providerFamilyClaude = "claude"
	providerFamilyCodex  = "codex"
	providerFamilyMixed  = "mixed"
	retroAnalysisTier    = "high"
)

// ProviderRunner is an interface for running LLM prompts.
// It matches the subset of provider.Provider methods used by Retro.
type ProviderRunner interface {
	Run(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
		handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
}

// Retro manages retrospective analysis
type Retro struct {
	provider       ProviderRunner
	learningsFile  *learnings.File
	rulesPath      string
	templatePath   string
	experimentPath string
	gromitDir      string
}

// TemplateContext holds data for retro prompt template
type TemplateContext struct {
	Rules             string
	Learnings         string
	RunStats          logger.RunStats
	BeadStats         map[string]logger.BeadStats
	Efficiency        *logger.EfficiencyReport
	ProcessTrend      *logger.ProcessTrend
	Experiment        *Experiment
	ExperimentMetrics *ExperimentMetrics
}

// ExperimentMetrics holds current efficiency metrics for an experiment's provider family.
type ExperimentMetrics struct {
	ProviderFamily            string
	CurrentAvgCostPerBead     float64
	CurrentAvgDurationPerBead time.Duration
}

// Result represents the outcome of a retro analysis
type Result struct {
	Analysis      string
	ProposedRules string
	Success       bool
	Efficiency    *logger.EfficiencyReport
	Experiment    *Experiment
}

// NewRetroWithProvider creates a new retrospective analyzer with a Provider
func NewRetroWithProvider(p ProviderRunner, gromitDir string) (*Retro, error) {
	if p == nil {
		return nil, fmt.Errorf("provider is nil")
	}
	learningsFile, err := learnings.NewFile(gromitDir)
	if err != nil {
		return nil, err
	}
	return &Retro{
		provider:       p,
		learningsFile:  learningsFile,
		rulesPath:      filepath.Join(gromitDir, "RULES.md"),
		templatePath:   filepath.Join(gromitDir, "templates", "PROMPT_retro.md"),
		experimentPath: filepath.Join(gromitDir, "experiment.json"),
		gromitDir:      gromitDir,
	}, nil
}

// resultGetter is a common interface for extracting results from either provider or Claude
type resultGetter interface {
	GetSuccess() bool
	GetOutput() string
}

// Run executes the retrospective analysis.
// The beadFilter parameter is optional (nil or empty = all beads included).
// When non-empty, only logs for bead IDs in the filter map are included in the analysis.
func (r *Retro) Run(ctx context.Context, beadFilter map[string]bool) (*Result, error) {
	if r == nil {
		return nil, fmt.Errorf("retro is nil")
	}
	// Check for provider
	if r.provider == nil {
		return nil, fmt.Errorf("provider is nil")
	}
	// Load learnings
	if r.learningsFile == nil {
		return nil, fmt.Errorf("learnings file is nil")
	}
	if err := r.learningsFile.Load(); err != nil {
		return nil, fmt.Errorf("loading learnings: %w", err)
	}

	// Load state for filtered learning hashes
	stateFile, err := state.NewFile(r.gromitDir)
	if err != nil {
		return nil, fmt.Errorf("creating state file: %w", err)
	}
	if err := stateFile.Load(); err != nil {
		return nil, fmt.Errorf("loading state: %w", err)
	}

	// Run batch filter on provisional learnings
	alreadyFiltered := stateFile.GetFilteredHashes()
	runnerAdapter := r.createLearningsAdapter()
	llmFilter := learnings.NewLLMFilter(runnerAdapter, "gromit", learnings.ProjectDescriptions.Gromit)
	newlyEvaluatedHashes, err := r.learningsFile.FilterProvisional(llmFilter, alreadyFiltered)
	if err != nil {
		return nil, fmt.Errorf("filtering provisional learnings: %w", err)
	}

	// Add newly-evaluated hashes to state (but don't save yet)
	hasNewHashes := len(newlyEvaluatedHashes) > 0
	if hasNewHashes {
		stateFile.AddFilteredHashes(newlyEvaluatedHashes)
	}

	// Reconcile filtered hashes against current provisional learnings
	currentProvisionals := r.learningsFile.GetProvisional()
	currentHashes := make(map[string]bool, len(currentProvisionals))
	for _, l := range currentProvisionals {
		currentHashes[l.Hash] = true
	}
	hasPrunedHashes := stateFile.ReconcileFilteredHashes(currentHashes)

	// Save state once if either new hashes were added or stale hashes were pruned
	if hasNewHashes || hasPrunedHashes {
		if err := stateFile.Save(); err != nil {
			return nil, fmt.Errorf("saving state with filtered hashes: %w", err)
		}
	}

	// Load rules
	rules, err := r.loadRules()
	if err != nil {
		return nil, fmt.Errorf("loading rules: %w", err)
	}

	// Load run stats and per-bead stats (with optional filtering)
	logsDir := filepath.Join(filepath.Dir(r.rulesPath), "logs")
	runStats, _ := logger.ReadAllLogsFiltered(logsDir, beadFilter)
	allBeadStats, _ := logger.ReadPerBeadStatsFiltered(logsDir, beadFilter)

	// Load efficiency report (current run is empty string, all runs are historical)
	efficiencyReport, _ := logger.ReadEfficiencyReportFiltered(logsDir, "", beadFilter)

	// Load active experiment (if any)
	experiment, _ := LoadExperiment(r.experimentPath)
	r.captureExperimentLearning(experiment)

	// Format learnings for prompt (after optional experiment-learning capture)
	learningsText := r.formatLearnings()

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

	// Run analysis (use high tier for quality analysis)
	analysisResult, err := r.runAnalysis(ctx, prompt)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Analysis:   analysisResult.GetOutput(),
		Success:    analysisResult.GetSuccess(),
		Efficiency: efficiencyReport,
		Experiment: experiment,
	}

	return result, nil
}

// createLearningsAdapter creates the appropriate adapter for learnings filtering
func (r *Retro) createLearningsAdapter() interface {
	Run(ctx context.Context, prompt string, model string) (*learnings.Result, error)
} {
	return learnings.NewProviderRunnerAdapter(r.provider)
}

// runAnalysis executes the LLM analysis using provider with streaming output
func (r *Retro) runAnalysis(ctx context.Context, prompt string) (resultGetter, error) {
	result, err := r.provider.StreamRun(ctx, prompt, retroAnalysisTier, os.Stderr, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("running provider analysis: %w", err)
	}
	if result == nil {
		fallback, fallbackErr := r.provider.Run(ctx, prompt, retroAnalysisTier)
		if fallbackErr != nil {
			return nil, fmt.Errorf("provider returned nil stream result and fallback failed: %w", fallbackErr)
		}
		if fallback != nil {
			if strings.TrimSpace(fallback.Output) == "" {
				return nil, fmt.Errorf("provider returned empty analysis output")
			}
			return &providerResultAdapter{fallback}, nil
		}
		return nil, fmt.Errorf("provider returned nil analysis result")
	}

	// Some provider/CLI stream modes can exit successfully but yield empty output.
	// Fall back to non-stream run so retro still gets analyzable text.
	if strings.TrimSpace(result.Output) == "" {
		fallback, fallbackErr := r.provider.Run(ctx, prompt, retroAnalysisTier)
		if fallbackErr == nil && fallback != nil && strings.TrimSpace(fallback.Output) != "" {
			return &providerResultAdapter{fallback}, nil
		}
		return nil, fmt.Errorf("provider returned empty analysis output")
	}
	return &providerResultAdapter{result}, nil
}

// providerResultAdapter adapts provider.Result to resultGetter interface
type providerResultAdapter struct {
	result *provider.Result
}

func (a *providerResultAdapter) GetSuccess() bool {
	return a.result.Success
}

func (a *providerResultAdapter) GetOutput() string {
	return a.result.Output
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
		Rules:             rules,
		Learnings:         learnings,
		RunStats:          runStats,
		BeadStats:         beadStats,
		Efficiency:        efficiency,
		ProcessTrend:      r.loadProcessTrend(),
		Experiment:        experiment,
		ExperimentMetrics: selectExperimentMetrics(experiment, efficiency),
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, ctx); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return sb.String(), nil
}

func selectExperimentMetrics(exp *Experiment, efficiency *logger.EfficiencyReport) *ExperimentMetrics {
	if exp == nil || efficiency == nil {
		return nil
	}

	family := normalizeProviderFamily(exp.PrimaryConcern)
	if family != "" {
		if stats, ok := efficiency.CurrentProviderFamilies[family]; ok {
			return experimentMetricsFromStats(family, stats)
		}
	}

	return &ExperimentMetrics{
		ProviderFamily:            providerFamilyMixed,
		CurrentAvgCostPerBead:     efficiency.CurrentAvgCostPerBead,
		CurrentAvgDurationPerBead: efficiency.CurrentAvgDurationPerBead,
	}
}

func experimentMetricsFromStats(family string, stats logger.ModelEfficiency) *ExperimentMetrics {
	return &ExperimentMetrics{
		ProviderFamily:            family,
		CurrentAvgCostPerBead:     stats.AvgCostUSD,
		CurrentAvgDurationPerBead: stats.AvgDuration,
	}
}

func normalizeProviderFamily(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case providerFamilyClaude:
		return providerFamilyClaude
	case providerFamilyCodex:
		return providerFamilyCodex
	default:
		return ""
	}
}

func (r *Retro) loadProcessTrend() *logger.ProcessTrend {
	if r == nil || r.gromitDir == "" {
		return nil
	}
	path := filepath.Join(r.gromitDir, "metrics", "process_trend.json")
	trend, err := logger.ReadProcessTrend(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load process trend: %v\n", err)
		return nil
	}
	return trend
}

func (r *Retro) captureExperimentLearning(exp *Experiment) {
	if r == nil || exp == nil || r.learningsFile == nil {
		return
	}

	decision := strings.ToLower(strings.TrimSpace(exp.ActDecision))
	if exp.LearningsCaptured {
		return
	}
	if strings.TrimSpace(exp.StudySummary) == "" {
		return
	}
	if decision != "keep" && decision != "revert" {
		return
	}

	expID := strings.TrimSpace(exp.ID)
	if expID == "" {
		expID = strings.TrimSpace(exp.Name)
	}
	content := fmt.Sprintf(
		"PDSA experiment `%s` completed with decision `%s`.\n\nHypothesis: %s\nChange: %s\nMeasurement: %s\nStudy: %s",
		expID,
		decision,
		strings.TrimSpace(exp.Hypothesis),
		strings.TrimSpace(exp.Change),
		strings.TrimSpace(exp.Measurement),
		strings.TrimSpace(exp.StudySummary),
	)

	if _, err := r.learningsFile.Add("retro", content, learnings.CategoryPatterns); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to capture experiment learning: %v\n", err)
		return
	}

	exp.LearningsCaptured = true
	if strings.TrimSpace(exp.Status) == "" || strings.EqualFold(exp.Status, "act") {
		exp.Status = "completed"
	}
	if exp.ActDate == nil {
		now := time.Now().UTC()
		exp.ActDate = &now
	}
	if err := SaveExperiment(r.experimentPath, exp); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to persist experiment learning state: %v\n", err)
	}
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

	var skipped int
	for beadID, stats := range beadStats {
		// Get full bead details (may fail for beads deleted since the log was written)
		b, err := client.Show(beadID)
		if err != nil {
			skipped++
			continue
		}

		// Populate status and close reason
		stats.Status = b.Status
		stats.CloseReason = b.CloseReason

		// Get comments
		comments, err := client.GetComments(beadID)
		if err != nil {
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
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "Note: skipped %d bead(s) no longer in bd\n", skipped)
	}
}

// LaunchClaudeCode launches Claude Code in interactive mode with the analysis results.
// The prompt instructs Claude Code on what actions it can take:
// - Edit RULES.md
// - Edit LEARNINGS.md
// - Run bd commands
// - Create specs
// - Select and persist experiments
func LaunchClaudeCode(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment, dir string) error {
	promptText := buildClaudeCodePrompt(analysis, efficiency, experiment)
	return runInteractiveClaude(promptText, dir, os.Stdin, os.Stdout, os.Stderr)
}

func buildClaudeCodePrompt(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment) string {
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
		if experiment.ID != "" {
			prompt.WriteString(fmt.Sprintf("**Experiment ID:** %s\n\n", experiment.ID))
		}

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
	prompt.WriteString("- When selecting an experiment, populate baseline metrics using ComputeBaselineMetrics from the retro package\n")
	prompt.WriteString("- Experiment recommendations should focus on concrete, testable process changes\n")
	prompt.WriteString("- Apply Five Whys analysis when investigating efficiency anomalies to identify root causes\n")
	prompt.WriteString("\nPlease review the analysis and take appropriate actions.\n")
	return prompt.String()
}
