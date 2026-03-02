package runner

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/experiment"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/epilogue"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/pipeline/review"
	"github.com/danabrams/gromit/internal/pipeline/validate"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/specbranch"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/tracker"
)

// defaultTierToModelMap defines the default Claude model tier mapping.
// This is used for backward compatibility when no providers are configured.
// Deprecation marker: RunnerDeprecationMarkerLegacyClaudeTierModelMap
var defaultTierToModelMap = map[string]string{
	"high":   "opus",
	"medium": "sonnet",
	"low":    "haiku",
}

const (
	RunnerDeprecationMarkerLegacyClaudeTierModelMap     = "runner-deprecated-legacy-claude-tier-model-map"
	RunnerDeprecationMarkerLegacyTrackerBackendFallback = "runner-deprecated-legacy-tracker-backend-fallback"
)

func newRunnerImpl(cfg *config.Config, output io.Writer, labels []string) (*Orchestrator, error) {
	return newRunnerImplWithStageContext(cfg, output, labels, nil)
}

func newRunnerImplWithStageContext(cfg *config.Config, output io.Writer, labels []string, stageCtx *StageContext) (*Orchestrator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if output == nil {
		output = os.Stdout
	}
	emitStartupCompatibilityDeprecationWarning(cfg, output)
	gromitDir := filepath.Dir(cfg.Paths.Templates)

	iterationLogger, err := logger.NewLogger(cfg.Paths.Logs)
	if err != nil {
		_, _ = fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
	}
	var trendUpdater *logger.AsyncTrendUpdater
	if iterationLogger != nil {
		trendUpdater = logger.NewAsyncTrendUpdater(
			cfg.Paths.Logs,
			filepath.Join(gromitDir, "metrics"),
			30,
			func(err error) {
				if err != nil {
					_, _ = fmt.Fprintf(output, "Warning: could not refresh process trend metrics: %v\n", err)
				}
			},
		)
	}

	statusWriter, err := NewStatusWriter(gromitDir)
	if err != nil {
		_, _ = fmt.Fprintf(output, "Warning: could not create status writer: %v\n", err)
	}

	// Load experiments early so they can be injected into stages
	var experimentMgr *experiment.Manager
	if cfg.Experiment.Enabled {
		exps, err := experiment.LoadExperiments(cfg.Experiment.ExperimentsDir)
		if err != nil {
			_, _ = fmt.Fprintf(output, "Warning: could not load experiments: %v\n", err)
		} else {
			experimentMgr = experiment.NewManager(exps, filepath.Join(gromitDir, "experiment"))
		}
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
	renderer.SetDecomposeTarget(cfg.Decompose.Target)

	trackerClientInterface, err := newTrackerClient(resolveTrackerBackend(cfg))
	if err != nil {
		return nil, err
	}
	beadsClient := newTrackerBeadClient(trackerClientInterface)

	syncOut := newSyncWriter(output)

	router, learningsProvider, sf, costDefs, err := buildRouterAndLearningsProvider(cfg, gromitDir, output)
	if err != nil {
		return nil, err
	}

	wireLearningsFilter(renderer, learningsProvider)
	wireSiblingEnrichmentResolver(renderer, cfg.Paths.Logs)

	specMergeController := newSpecMergeController(
		cfg,
		trackerClientInterface,
		&specMergeRouterAdapter{router: router},
		&specMergeReviewRendererAdapter{renderer: renderer},
		stageCtx,
	)

	analyzerObj, err := analyzer.NewAnalyzer(learningsProvider, cfg.Models.Validation, renderer)
	if err != nil {
		return nil, err
	}
	streamLogger, err := logger.NewStreamLogger(cfg.Paths.Logs)
	if err != nil {
		_, _ = fmt.Fprintf(output, "Warning: could not create stream logger: %v\n", err)
	}
	buildPromptRegistry := newBuildPromptRegistry()
	buildCacheVersionKey := resolveBuildCacheVersionKey(cfg, gromitDir)

	// Stage 1: Gate (prepare.New with optional Prechecker, StuckDetector, Decomposer)
	decomposer := &decomposerAdapter{
		tracker:     trackerClientInterface,
		beads:       beadsClient,
		router:      router,
		maxSubBeads: cfg.Validation.RuntimeMaxSubBeadsValue(),
	}
	gateStage := prepare.New(syncOut)
	gateStage.WithDecomposer(decomposer)
	gateStage.WithReadinessAssessor(NewDeterministicReadinessAssessor())
	if cfg.Gate.EffectiveAutoGenerateCriteria() {
		if enricher := newGateCriteriaEnricher(cfg, router, trackerClientInterface); enricher != nil {
			gateStage.WithCriteriaEnricher(enricher)
		}
	}
	// Stage 2: Build (execute.New with Invoker and PromptRenderer)
	buildExecInvoker := newBuildExecutionInvoker(cfg, router, syncOut, streamLogger)
	buildStage := execute.New(
		&invokerAdapter{
			execInvoker:      buildExecInvoker,
			promptRegistry:   buildPromptRegistry,
			cacheVersionKey:  buildCacheVersionKey,
			providerCostDefs: costDefs,
		},
		&renderAdapter{
			r:              renderer,
			promptRegistry: buildPromptRegistry,
		},
		syncOut,
	)
	if beadsClient != nil {
		if runner := optionalTDDCycleRunner(cfg, renderer, router, syncOut, beadsClient, costDefs); runner != nil {
			buildStage.WithTDDCycleRunner(runner)
		}
	}
	if experimentMgr != nil {
		buildStage.WithExperimentManager(experimentMgr)
	}

	// Conditionally wrap Build stage with escalation handler for full
	// retry/analysis/escalation/decomposition behavior.
	var buildPipelineStage pipeline.Stage = buildStage
	if cfg.Escalation.Enabled {
		buildPipelineStage = newEscalationBuildStage(
			cfg, analyzerObj, beadsClient,
			decomposer.DecomposeToSubTasks, decomposer.CreateSubBeads,
			buildExecInvoker, renderer,
			buildStage, buildPromptRegistry, buildCacheVersionKey, costDefs, syncOut,
		)
	}

	// Stage 3: Validate (validate.New with CommandRunner)
	validateStage := validate.New(&cmdRunnerAdapter{runner: defaultCmdRunner}, syncOut)

	// Wrapper for getGitDiff to match review.GitDiffFn signature
	gitDiffFn := func(ctx context.Context) (string, error) {
		return getGitDiff(ctx, "")
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
		&beadLifecycleAdapter{tracker: trackerClientInterface},
		&statusWriterAdapter{sw: statusWriter},
		syncOut,
	)

	epilogueStage.WithCommandRunner(&epilogueCommandRunnerAdapter{runner: defaultCmdRunner})
	epilogueStage.WithFailureLearner(&failureLearnerAdapter{
		renderer: renderer,
		router:   router,
		analyzer: analyzerObj,
		logFn:    func(msg string, args ...interface{}) { _, _ = fmt.Fprintf(syncOut, msg+"\n", args...) },
	})

	if iterationLogger != nil {
		epilogueStage.WithIterationLogWriter(&iterationLogWriterAdapter{
			logger:       iterationLogger,
			trendUpdater: trendUpdater,
		})
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
		if beadsClient == nil {
			return nil, fmt.Errorf("bead client is not configured")
		}
		if len(labels) > 0 {
			return beadsClient.ReadyWithLabel(ctx, labels[0])
		}
		return beadsClient.Ready(ctx)
	}

	specProgressLabel := resolveSingleSpecProgressLabel(labels)
	if statusWriter != nil {
		statusWriter.SetScopeLabel(specProgressLabel)
	}

	coordinator, err := newIntegrationQueueCoordinator(cfg, gromitDir)
	if err != nil {
		return nil, err
	}
	orchCfg := OrchestratorConfig{
		Gate:     gateStage,
		Build:    buildPipelineStage,
		Validate: validateStage,
		Review:   reviewStage,
		Epilogue: epilogueStage,
		GetBead:  getBeadFn,
		GetBeadByID: func(ctx context.Context, beadID string) (*bead.Bead, error) {
			if beadsClient == nil {
				return nil, fmt.Errorf("bead client is not configured")
			}
			return beadsClient.Show(ctx, beadID)
		},
		Config:              cfg,
		SpecMergeController: specMergeController,
		GlobalStatsPath:     filepath.Join(gromitDir, "stats.json"),
		GetRunID:            getRunIDFn,
		LogsDir:             cfg.Paths.Logs,
		Output:              syncOut,
		TrendUpdater:        trendUpdater,
		AutoTriageService:   newAutoTriageService(cfg, gromitDir, trackerClientInterface, sf),
		ExperimentMgr:       experimentMgr,
		StatusWriter: func(iteration int, beadID, beadTitle string, dl time.Time) {
			if statusWriter != nil {
				if specProgressLabel != "" {
					total, err := estimateScopedIterationTotal(context.Background(), trackerClientInterface, specProgressLabel, iteration)
					if err == nil {
						statusWriter.SetIterationTotal(total)
					} else {
						statusWriter.SetIterationTotal(0)
					}
				} else {
					statusWriter.SetIterationTotal(cfg.Loop.MaxIterations)
				}

				timeBudgetMinutes := 0
				if !dl.IsZero() {
					statusWriter.SetTimeBudgetFromDeadline(dl)
					timeBudgetMinutes = statusWriter.TimeBudgetMinutes()
				}
				_ = statusWriter.Write(iteration, beadID, beadTitle, "", true, cfg.Loop.MaxIterations, timeBudgetMinutes)
			}
		},
		StatusFinalizer: func(iteration int, runErr error) {
			_ = runErr
			if statusWriter == nil {
				return
			}
			if err := statusWriter.WriteFinal(iteration); err != nil {
				_, _ = fmt.Fprintf(syncOut, "Warning: could not write final status: %v\n", err)
			}
		},
		StateSaver:       sf,
		ProviderCostDefs: costDefs,
		Coordinator:      coordinator,
		StageContext:     stageCtx,
	}

	if cfg.Methodology.Granularity == config.MethodologyGranularitySpec {
		repoDir := filepath.Dir(gromitDir)
		baseBranch := cfg.Git.BaseBranch
		orchCfg.BranchRouter = specbranch.NewRouter(baseBranch)
		orchCfg.GitCheckout = specbranch.NewGitOps(repoDir, baseBranch)
	}

	return NewOrchestrator(orchCfg), nil
}

func newBuildExecutionInvoker(cfg *config.Config, router *provider.Router, output io.Writer, streamLogger *logger.StreamLogger) *execution.Invoker {
	invoker := execution.NewInvoker(&executionRouterAdapter{router: router}, output, streamLogger)
	preserve := false
	if cfg != nil {
		preserve = cfg.Stream.PreserveProviderOutputEnabled()
	}
	return invoker.WithPreserveProviderTerminalStream(preserve)
}

func resolveBuildCacheVersionKey(cfg *config.Config, gromitDir string) string {
	if cfg == nil {
		return ""
	}
	paths := []string{
		filepath.Join(gromitDir, "RULES.md"),
		filepath.Join(cfg.Paths.Templates, "PROMPT_build.md"),
		filepath.Join(cfg.Paths.Templates, "PROMPT_tdd_build.md"),
		filepath.Join(cfg.Paths.Templates, "PROMPT_refactor_build.md"),
		cfg.Paths.ProjectClaudeMD,
	}
	sort.Strings(paths)

	hasher := sha1.New()
	wroteAny := false
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		data, err := os.ReadFile(trimmed)
		if err != nil {
			continue
		}
		if len(data) == 0 {
			continue
		}
		wroteAny = true
		_, _ = hasher.Write([]byte(trimmed))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(data)
		_, _ = hasher.Write([]byte{0})
	}
	if !wroteAny {
		return ""
	}
	return "build-rules-v1-" + hex.EncodeToString(hasher.Sum(nil))
}

func emitStartupCompatibilityDeprecationWarning(cfg *config.Config, output io.Writer) {
	if cfg == nil || output == nil {
		return
	}

	ctx := cfg.ResolveCompatibilityContext()
	markers := config.CompatibilityDeprecationMarkers(ctx)
	if len(markers) == 0 {
		return
	}

	_, _ = fmt.Fprintf(
		output,
		"Warning: %s active; set compatibility.strict_legacy_fallback: true now (strict-by-default planned after %s)\n",
		strings.Join(markers, ", "),
		config.CompatibilityStrictDefaultCutoverDate,
	)
}

func resolveSingleSpecProgressLabel(labels []string) string {
	if len(labels) != 1 {
		return ""
	}
	const specPrefix = "spec:"
	if labels[0] == "" || len(labels[0]) <= len(specPrefix) || labels[0][:len(specPrefix)] != specPrefix {
		return ""
	}
	return labels[0]
}

func estimateScopedIterationTotal(ctx context.Context, client tracker.Client, label string, iteration int) (int, error) {
	if client == nil || label == "" || iteration <= 0 {
		return 0, nil
	}

	items, err := client.ListWithLabel(ctx, label)
	if err != nil {
		return 0, err
	}

	openNonEpicCount := 0
	for _, item := range items {
		if strings.EqualFold(item.Status, tracker.StatusClosed) {
			continue
		}
		typ := strings.TrimSpace(item.Metadata["type"])
		if strings.EqualFold(typ, "epic") {
			continue
		}
		openNonEpicCount++
	}

	// iteration is 1-based and points to the currently active bead.
	// completed before this bead = iteration - 1.
	total := (iteration - 1) + openNonEpicCount
	if total < iteration {
		total = iteration
	}
	return total, nil
}

func buildRouterAndLearningsProvider(cfg *config.Config, gromitDir string, output io.Writer) (*provider.Router, provider.Provider, *state.File, map[string]config.ProviderDef, error) {
	if cfg.HasProviders() {
		cfg.SetDefaults()
		cfg.NormalizeNilFields()

		providers, err := provider.BuildProvidersFromConfig(cfg)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		costDefs := make(map[string]config.ProviderDef)
		for name, def := range cfg.Providers {
			costDefs[providers[name].Name()] = def
		}

		var sf *state.File
		sf, err = state.NewFile(gromitDir)
		if err != nil {
			_, _ = fmt.Fprintf(output, "Warning: could not create state file: %v\n", err)
		} else if loadErr := sf.Load(); loadErr != nil {
			_, _ = fmt.Fprintf(output, "Warning: could not load state: %v\n", loadErr)
		} else {
			applyStateStalenessRecovery(sf, cfg, output)
		}

		circuitBreaker := provider.NewCircuitBreaker(&cfg.Routing.CircuitBreaker)

		router := provider.NewRouter(
			providers,
			cfg.Routing.PhasePreferences,
			cfg.Routing.Ratio,
			provider.ParseFallbackCooldown(cfg),
			sf,
			circuitBreaker,
		)

		learningsProvider := selectLearningsProvider(cfg.Learnings.Provider, providers)
		return router, learningsProvider, sf, costDefs, nil
	}

	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	claudeProvider := provider.NewClaudeProvider(claudeClient, defaultTierToModelMap)
	return provider.NewSingleProviderRouter(claudeProvider), claudeProvider, nil, nil, nil
}

func applyStateStalenessRecovery(sf *state.File, cfg *config.Config, output io.Writer) {
	if sf == nil {
		return
	}
	threshold := 60
	if cfg != nil && cfg.State.StaleThreshold > 0 {
		threshold = cfg.State.StaleThreshold
	}

	isStale, reason := sf.CheckStaleness(threshold)
	if !isStale {
		return
	}

	sf.AutoHeal()
	if output != nil {
		_, _ = fmt.Fprintf(output, "Warning: state.json staleness detected (%s); provider routing state reset\n", reason)
	}
	if err := sf.Save(); err != nil {
		if output != nil {
			_, _ = fmt.Fprintf(output, "Warning: could not save healed state: %v\n", err)
		}
	}
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
