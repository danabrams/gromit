package retro

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/state"
)

const (
	providerFamilyClaude = "claude"
	providerFamilyCodex  = "codex"
	providerFamilyMixed  = "mixed"
	retroAnalysisTier    = "high"
	sectionEfficiency    = "efficiency"
	sectionProcessTrend  = "process_trend"
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
	promptBudget   int
	diagnostics    *prompt.PromptDiagnostics
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
	return NewRetroWithProviderAndBudget(p, gromitDir, 0)
}

// NewRetroWithProviderAndBudget creates a new retrospective analyzer with a Provider
// and optional prompt budget for retro rules/learnings shaping.
func NewRetroWithProviderAndBudget(p ProviderRunner, gromitDir string, promptBudget int) (*Retro, error) {
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
		promptBudget:   promptBudget,
	}, nil
}

// SetPromptBudget configures optional max character shaping for retro prompt rules/learnings.
func (r *Retro) SetPromptBudget(maxChars int) {
	if r == nil {
		return
	}
	r.promptBudget = maxChars
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

	processTrend := r.loadProcessTrend()
	preShapeTokens := 0
	postShapeTokens := 0
	var shapeReport *prompt.ShapeReport
	if r.promptBudget > 0 {
		preShapeTokens = prompt.EstimateTokens(rules) + prompt.EstimateTokens(learnings)
		rules, learnings, shapeReport = prompt.ShapeRetroForBudget(rules, learnings, r.promptBudget)
		postShapeTokens = prompt.EstimateTokens(rules) + prompt.EstimateTokens(learnings)
	}

	sectionTokens := buildRetroSectionTokens(rules, learnings, runStats, beadStats, efficiency, processTrend)
	diagnostics := prompt.NewDiagnostics("retro", sectionTokens)
	if r.promptBudget > 0 {
		diagnostics.BudgetMaxChars = r.promptBudget
		diagnostics.PreShapeTokens = preShapeTokens
		diagnostics.PostShapeTokens = postShapeTokens
		if shapeReport != nil {
			diagnostics.ShapeActions = append([]string{}, shapeReport.TrimActions...)
		}
	}
	r.diagnostics = diagnostics

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
		ProcessTrend:      processTrend,
		Experiment:        experiment,
		ExperimentMetrics: selectExperimentMetrics(experiment, efficiency),
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, ctx); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return sb.String(), nil
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
