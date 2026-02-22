package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline/epilogue"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/pipeline/review"
	"github.com/danabrams/gromit/internal/pipeline/validate"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/worktree"
)

// defaultTierToModelMap defines the default Claude model tier mapping.
// This is used for backward compatibility when no providers are configured.
var defaultTierToModelMap = map[string]string{
	"high":   "opus",
	"medium": "sonnet",
	"low":    "haiku",
}

// defaultCodexTierToModelMap defines the default Codex model tier mapping.
var defaultCodexTierToModelMap = map[string]string{
	"high":   "gpt-5.3-codex",
	"medium": "gpt-5.3-codex",
	"low":    "gpt-5.3-codex-spark",
}

type runnerPolicies struct {
	escalation  policy.EscalationPolicy
	methodology policy.MethodologyPolicy
	validation  policy.ValidationPolicy
	stuck       policy.StuckPolicy
}

func newRunnerPolicies(cfg *config.Config) runnerPolicies {
	return runnerPolicies{
		escalation:  policy.NewConfigEscalationPolicy(cfg),
		methodology: policy.NewConfigMethodologyPolicy(cfg),
		validation:  policy.NewConfigValidationPolicy(cfg),
		stuck:       policy.NewConfigStuckPolicy(cfg),
	}
}

// Adapter types for bridging existing infrastructure to pipeline stage interfaces.

// invokerAdapter wraps *provider.Router to satisfy execute.Invoker.
type invokerAdapter struct {
	router *provider.Router
	output io.Writer
}

func (a *invokerAdapter) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if a.router == nil {
		return nil, fmt.Errorf("router is nil")
	}
	p, _ := a.router.Select("build", tier)
	if p == nil {
		return nil, fmt.Errorf("no provider available for tier %s", tier)
	}
	return p.Run(ctx, prompt, tier)
}

func (a *invokerAdapter) StreamRun(ctx context.Context, prompt, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if a.router == nil {
		return nil, fmt.Errorf("router is nil")
	}
	p, _ := a.router.Select("build", tier)
	if p == nil {
		return nil, fmt.Errorf("no provider available for tier %s", tier)
	}
	return p.StreamRun(ctx, prompt, tier, w, handler, onToolCall)
}

// renderAdapter wraps prompt.Renderer to satisfy execute.PromptRenderer.
type renderAdapter struct {
	r *prompt.Renderer
}

func (a *renderAdapter) RenderBuild(title, description string, validationFailures []string) (string, error) {
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: validationFailures,
	}
	return a.r.RenderBuild(ctx)
}

func (a *renderAdapter) RenderTDDBuild(title, description string, validationFailures []string) (string, error) {
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: validationFailures,
	}
	return a.r.RenderTDDBuild(ctx)
}

func (a *renderAdapter) RenderRefactorBuild(title, description string, validationFailures []string) (string, error) {
	ctx := &prompt.Context{
		Bead: &bead.Bead{
			Title:       title,
			Description: description,
		},
		RecentValidationFailures: validationFailures,
	}
	return a.r.RenderBuild(ctx)
}

// cmdRunnerAdapter wraps a command runner function to satisfy validate.CommandRunner.
type cmdRunnerAdapter struct {
	runner func(ctx context.Context, command, workDir string) (string, string, int, error)
}

func (a *cmdRunnerAdapter) Run(ctx context.Context, command, workDir string) (string, string, int, error) {
	return a.runner(ctx, command, workDir)
}

// reviewInvokerAdapter wraps *provider.Router to satisfy review.Invoker.
type reviewInvokerAdapter struct {
	router  *provider.Router
	syncOut *syncWriter
}

func (a *reviewInvokerAdapter) StreamRun(ctx context.Context, prompt string, model string, w io.Writer) (string, error) {
	if a.router == nil {
		return "", fmt.Errorf("router is nil")
	}
	p, _ := a.router.Select("review", "high")
	if p == nil {
		return "", fmt.Errorf("no provider available for review")
	}
	result, err := p.Run(ctx, prompt, "high")
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("review invoker returned nil result")
	}
	return result.Output, nil
}

// beadCreatorAdapter wraps bead.Client to satisfy review.BeadCreator.
type beadCreatorAdapter struct {
	beads *bead.Client
}

func (a *beadCreatorAdapter) Create(title string, priority int, labels []string, outputs []string) (string, error) {
	b, err := a.beads.Create(title, priority, labels, outputs)
	if err != nil {
		return "", err
	}
	if b == nil {
		return "", fmt.Errorf("beads.Create returned nil")
	}
	return b.ID, nil
}

// reviewRendererAdapter wraps prompt.Renderer to satisfy review.PromptRenderer.
type reviewRendererAdapter struct {
	r *prompt.Renderer
}

func (a *reviewRendererAdapter) RenderReview(beadTitle, diff string) (string, error) {
	ctx := &prompt.ReviewContext{
		Bead: &bead.Bead{
			Title: beadTitle,
		},
		Diff: diff,
	}
	return a.r.RenderReview(ctx)
}

// beadLifecycleAdapter wraps bead.Client to satisfy epilogue.BeadLifecycle.
type beadLifecycleAdapter struct {
	beads *bead.Client
}

func (a *beadLifecycleAdapter) Close(id string) error {
	return a.beads.Close(id)
}

func (a *beadLifecycleAdapter) Sync() error {
	return a.beads.Sync()
}

// statusWriterAdapter wraps runner.StatusWriter to satisfy epilogue.StatusWriter.
type statusWriterAdapter struct {
	sw *StatusWriter
}

func (a *statusWriterAdapter) Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error {
	if a.sw == nil {
		return nil
	}
	return a.sw.Write(iteration, beadID, beadTitle, model, true, maxIterations, timeBudgetMinutes)
}

// worktreeMergerAdapter wraps worktree.Manager to satisfy epilogue.WorktreeMerger.
type worktreeMergerAdapter struct {
	mgr *worktree.Manager
}

func (a *worktreeMergerAdapter) PendingBranches() ([]string, error) {
	return a.mgr.PendingBranches()
}

func (a *worktreeMergerAdapter) MergeBack(branch string) error {
	return a.mgr.MergeBack(branch)
}

// epilogueCommandRunnerAdapter wraps a command runner function to satisfy epilogue.CommandRunner.
type epilogueCommandRunnerAdapter struct {
	runner func(ctx context.Context, command, workDir string) (string, string, int, error)
}

func (a *epilogueCommandRunnerAdapter) Run(ctx context.Context, command string) (string, string, int, error) {
	return a.runner(ctx, command, "")
}

// iterationLogWriterAdapter wraps *logger.Logger to satisfy epilogue.IterationLogWriter.
type iterationLogWriterAdapter struct {
	logger *logger.Logger
}

func (a *iterationLogWriterAdapter) Write(log *logger.IterationLog) error {
	if a.logger == nil {
		return nil
	}
	return a.logger.LogIteration(log)
}

// failureLearnerAdapter wraps analyzer and related dependencies to satisfy epilogue.FailureLearner.
type failureLearnerAdapter struct {
	renderer *prompt.Renderer
	router   *provider.Router
	logFn    func(string, ...interface{})
}

func (a *failureLearnerAdapter) ExtractFailureLearning(ctx context.Context, beadID, beadTitle string) error {
	// Placeholder for failure learning extraction
	// In a full implementation, this would use the analyzer to extract learnings
	return nil
}

func newRunnerImpl(cfg *config.Config, output io.Writer, labels []string) (*Orchestrator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if output == nil {
		output = os.Stdout
	}

	iterationLogger, err := logger.NewLogger(cfg.Paths.Logs)
	if err != nil {
		_, _ = fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
	}

	gromitDir := filepath.Dir(cfg.Paths.Templates)

	statusWriter, err := NewStatusWriter(gromitDir)
	if err != nil {
		_, _ = fmt.Fprintf(output, "Warning: could not create status writer: %v\n", err)
	}

	renderer, err := prompt.NewRenderer(
		cfg.Paths.Templates,
		cfg.Paths.Specs,
		cfg.Paths.ProjectClaudeMD,
		gromitDir,
	)
	if err != nil {
		return nil, err
	}
	renderer.SetMaxLearningChars(cfg.Learnings.MaxLearningChars)
	renderer.SetSkipBuildLearnings(cfg.Learnings.SkipBuildLearnings)
	renderer.SetBudgetConfig(cfg.Prompt.Budget.MaxChars, cfg.Prompt.Budget.LearningCapChars)

	beadsClient, err := bead.NewClient()
	if err != nil {
		return nil, err
	}

	syncOut := newSyncWriter(output)

	router, learningsProvider, _, _, err := buildRouterAndLearningsProvider(cfg, gromitDir, output)
	if err != nil {
		return nil, err
	}

	wireLearningsFilter(renderer, learningsProvider)
	wireSiblingEnrichmentResolver(renderer, cfg.Paths.Logs)

	_, err = analyzer.NewAnalyzer(learningsProvider, cfg.Models.Validation, renderer)
	if err != nil {
		return nil, err
	}

	// Stage 1: Gate (prepare.New with optional Prechecker, StuckDetector, Decomposer)
	gateStage := prepare.New(syncOut)

	// Stage 2: Build (execute.New with Invoker and PromptRenderer)
	buildStage := execute.New(&invokerAdapter{router: router, output: syncOut}, &renderAdapter{r: renderer}, syncOut)

	// Stage 3: Validate (validate.New with CommandRunner)
	validateStage := validate.New(&cmdRunnerAdapter{runner: defaultCmdRunner}, syncOut)

	// Wrapper for getGitDiff to match review.GitDiffFn signature
	gitDiffFn := func() (string, error) {
		return getGitDiff("")
	}

	// Stage 4: Review (review.New with Invoker, BeadCreator, PromptRenderer, GitDiffFn)
	reviewStage := review.New(
		&reviewInvokerAdapter{router: router, syncOut: syncOut},
		&beadCreatorAdapter{beads: beadsClient},
		&reviewRendererAdapter{r: renderer},
		gitDiffFn,
		syncOut,
	)

	// Stage 5: Epilogue (epilogue.New with BeadLifecycle and StatusWriter)
	epilogueStage := epilogue.New(
		&beadLifecycleAdapter{beads: beadsClient},
		&statusWriterAdapter{sw: statusWriter},
		syncOut,
	)

	// Wire optional Epilogue dependencies
	if cfg.Worktree.IsEnabled() {
		mainDir := filepath.Dir(gromitDir)
		manager, mgrErr := worktree.NewManager(mainDir)
		if mgrErr == nil {
			epilogueStage.WithWorktree(&worktreeMergerAdapter{mgr: manager})
		}
	}

	epilogueStage.WithCommandRunner(&epilogueCommandRunnerAdapter{runner: defaultCmdRunner})
	epilogueStage.WithFailureLearner(&failureLearnerAdapter{
		renderer: renderer,
		router:   router,
		logFn:    func(msg string, args ...interface{}) { _, _ = fmt.Fprintf(syncOut, msg+"\n", args...) },
	})

	if iterationLogger != nil {
		epilogueStage.WithIterationLogWriter(&iterationLogWriterAdapter{logger: iterationLogger})
	}

	// Create OrchestratorConfig
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	getRunIDFn := func() string {
		if iterationLogger != nil {
			return iterationLogger.RunID()
		}
		return ""
	}

	getBeadFn := func(ctx context.Context) (*bead.Bead, error) {
		if len(labels) > 0 {
			return beadsClient.ReadyWithLabel(labels[0])
		}
		return beadsClient.Ready()
	}

	orchCfg := OrchestratorConfig{
		Gate:            gateStage,
		Build:           buildStage,
		Validate:        validateStage,
		Review:          reviewStage,
		Epilogue:        epilogueStage,
		GetBead:         getBeadFn,
		Config:          cfg,
		GlobalStatsPath: filepath.Join(gromitDir, "stats.json"),
		GetRunID:        getRunIDFn,
		LogsDir:         cfg.Paths.Logs,
		Output:          syncOut,
		StatusWriter: func(iteration int, beadID, beadTitle string) {
			if statusWriter != nil {
				_ = statusWriter.Write(iteration, beadID, beadTitle, "", true, cfg.Loop.MaxIterations, 0)
			}
		},
	}

	return NewOrchestrator(orchCfg), nil
}

func buildInvoker(router *provider.Router, output *syncWriter, stallTimeoutFn execution.StallTimeoutFunc, cfg *config.Config) *execution.Invoker {
	var execRouter execution.Router
	if router != nil {
		execRouter = &routerAdapter{r: router}
	}
	return execution.NewInvoker(execRouter, output, nil).
		WithHeartbeat(output, stallTimeoutFn).
		WithPreserveProviderTerminalStream(cfg.Stream.PreserveProviderOutputEnabled())
}

func buildRouterAndLearningsProvider(cfg *config.Config, gromitDir string, output io.Writer) (*provider.Router, provider.Provider, *state.File, map[string]config.ProviderDef, error) {
	if cfg.HasProviders() {
		cfg.SetDefaults()
		cfg.NormalizeNilFields()

		providers, costDefs, err := buildProvidersFromConfig(cfg)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		var sf *state.File
		sf, err = state.NewFile(gromitDir)
		if err != nil {
			_, _ = fmt.Fprintf(output, "Warning: could not create state file: %v\n", err)
		} else if loadErr := sf.Load(); loadErr != nil {
			_, _ = fmt.Fprintf(output, "Warning: could not load state: %v\n", loadErr)
		}

		router := provider.NewRouter(
			providers,
			cfg.Routing.PhasePreferences,
			cfg.Routing.Ratio,
			parseFallbackCooldown(cfg),
			sf,
			nil,
		)

		learningsProvider := selectLearningsProvider(providers)
		return router, learningsProvider, sf, costDefs, nil
	}

	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	claudeProvider := provider.NewClaudeProvider(claudeClient, defaultTierToModelMap)
	return provider.NewSingleProviderRouter(claudeProvider), claudeProvider, nil, nil, nil
}

func initializeSpecOrchestration(cfg *config.Config, r *Runner) error {
	if cfg == nil || r == nil {
		return nil
	}
	if cfg.Methodology.Granularity != config.MethodologyGranularitySpec {
		return nil
	}

	r.specOrchestrator = newSpecOrchestrator(r)
	gate, err := newSpecGate(r)
	if err != nil {
		return err
	}
	r.specGate = gate
	return nil
}

func buildProvidersFromConfig(cfg *config.Config) (map[string]provider.Provider, map[string]config.ProviderDef, error) {
	providers := make(map[string]provider.Provider)
	costDefs := make(map[string]config.ProviderDef)
	for name, def := range cfg.Providers {
		switch {
		case name == "claude" || def.Binary == "claude":
			tierMap := def.Models
			if len(tierMap) == 0 {
				tierMap = defaultTierToModelMap
			}
			client, err := claude.NewClient(def.Binary, def.Flags, cfg.Claude.Timeout)
			if err != nil {
				return nil, nil, err
			}
			providers[name] = provider.NewClaudeProvider(client, tierMap)
		case name == "codex" || name == "openai" || def.Binary == "codex":
			tierMap := def.Models
			if len(tierMap) == 0 {
				tierMap = defaultCodexTierToModelMap
			}
			codexProvider := provider.NewCodexProvider(def.Binary, def.Flags, tierMap)
			codexProvider.SetReasoningEffort(def.ReasoningEffort)
			providers[name] = codexProvider
		default:
			return nil, nil, fmt.Errorf("unrecognized provider %q: supported providers are \"claude\" and \"codex\"", name)
		}
		costDefs[providers[name].Name()] = def
	}
	return providers, costDefs, nil
}

func parseFallbackCooldown(cfg *config.Config) time.Duration {
	if !cfg.Routing.Fallback.EnabledOrDefault(len(cfg.Providers) > 1) || cfg.Routing.Fallback.Cooldown == "" {
		return 0
	}
	cooldown, err := time.ParseDuration(cfg.Routing.Fallback.Cooldown)
	if err != nil {
		return 30 * time.Minute
	}
	return cooldown
}

func selectLearningsProvider(providers map[string]provider.Provider) provider.Provider {
	if cp, ok := providers["claude"]; ok {
		return cp
	}
	for _, p := range providers {
		return p
	}
	return nil
}

func wireLearningsFilter(renderer PromptRenderer, learningsProvider provider.Provider) {
	if renderer == nil || learningsProvider == nil {
		return
	}
	lf := renderer.GetLearningsFile()
	if lf == nil {
		return
	}

	providerRunnerAdapter := learnings.NewProviderRunnerAdapter(learningsProvider)
	lf.SetFilter(learnings.NewLLMFilter(providerRunnerAdapter, "gromit", learnings.ProjectDescriptions.Gromit))
}

func wireSiblingEnrichmentResolver(renderer PromptRenderer, logsDir string) {
	if renderer == nil {
		return
	}
	renderer.SetSiblingTouchedPackagesResolver(func(current *bead.Bead, parent *bead.Bead) ([]string, error) {
		if current == nil {
			return []string{}, nil
		}
		specID := specLabelFromCurrentOrParent(current, parent)
		return logger.ReadSiblingTouchedPackagesBySpec(logsDir, current.ID, specID)
	})
}

func specLabelFromCurrentOrParent(current, parent *bead.Bead) string {
	if current == nil {
		return ""
	}
	specID := bead.FindSpecLabel(current.Labels)
	if specID == "" && parent != nil {
		specID = bead.FindSpecLabel(parent.Labels)
	}
	return specID
}
