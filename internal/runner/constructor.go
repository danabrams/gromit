package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/worktree"
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
	RunnerDeprecationMarkerLegacyClaudeTierModelMap    = "runner-deprecated-legacy-claude-tier-model-map"
	RunnerDeprecationMarkerLegacyTrackerBackendFallback = "runner-deprecated-legacy-tracker-backend-fallback"
)

func newRunnerImpl(cfg *config.Config, output io.Writer, labels []string) (*Orchestrator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if output == nil {
		output = os.Stdout
	}
	emitStartupCompatibilityDeprecationWarning(cfg, output)

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

	beadsClient, err := newTrackerClient(resolveTrackerBackend(cfg))
	if err != nil {
		return nil, err
	}

	syncOut := newSyncWriter(output)

	router, learningsProvider, sf, costDefs, err := buildRouterAndLearningsProvider(cfg, gromitDir, output)
	if err != nil {
		return nil, err
	}

	wireLearningsFilter(renderer, learningsProvider)
	wireSiblingEnrichmentResolver(renderer, cfg.Paths.Logs)

	analyzerObj, err := analyzer.NewAnalyzer(learningsProvider, cfg.Models.Validation, renderer)
	if err != nil {
		return nil, err
	}

	// Stage 1: Gate (prepare.New with optional Prechecker, StuckDetector, Decomposer)
	gateStage := prepare.New(syncOut)
	gateStage.WithDecomposer(&decomposerAdapter{
		beads:       beadsClient,
		router:      router,
		maxSubBeads: cfg.Validation.RuntimeMaxSubBeadsValue(),
	})

	// Stage 2: Build (execute.New with Invoker and PromptRenderer)
	buildStage := execute.New(&invokerAdapter{router: router, output: syncOut}, &renderAdapter{r: renderer}, syncOut)
	if runner := optionalTDDCycleRunner(cfg, renderer, router, syncOut); runner != nil {
		buildStage.WithTDDCycleRunner(runner)
	}

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
		analyzer: analyzerObj,
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

	specProgressLabel := resolveSingleSpecProgressLabel(labels)
	if statusWriter != nil {
		statusWriter.SetScopeLabel(specProgressLabel)
	}

	orchCfg := OrchestratorConfig{
		Gate:            gateStage,
		Build:           buildStage,
		Validate:        validateStage,
		Review:          reviewStage,
		Epilogue:        epilogueStage,
		GetBead:         getBeadFn,
		GetBeadByID: func(ctx context.Context, beadID string) (*bead.Bead, error) {
			return beadsClient.Show(beadID)
		},
		Config:          cfg,
		GlobalStatsPath: filepath.Join(gromitDir, "stats.json"),
		GetRunID:        getRunIDFn,
		LogsDir:         cfg.Paths.Logs,
		Output:          syncOut,
		StatusWriter: func(iteration int, beadID, beadTitle string, dl time.Time) {
			if statusWriter != nil {
				if specProgressLabel != "" {
					total, err := estimateScopedIterationTotal(beadsClient, specProgressLabel, iteration)
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
					timeBudgetMinutes = int(time.Until(dl).Minutes())
				}
				_ = statusWriter.Write(iteration, beadID, beadTitle, "", true, cfg.Loop.MaxIterations, timeBudgetMinutes)
			}
		},
		StateSaver:       sf,
		ProviderCostDefs: costDefs,
	}

	return NewOrchestrator(orchCfg), nil
}

func emitStartupCompatibilityDeprecationWarning(cfg *config.Config, output io.Writer) {
	if cfg == nil || output == nil {
		return
	}

	ctx := cfg.ResolveCompatibilityContext()
	markers := config.CompatibilityDeprecationMarkers(ctx)
	if marker := resolveTrackerBackendDeprecationMarker(cfg); marker != "" {
		markers = append(markers, marker)
	}
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

func estimateScopedIterationTotal(client *bead.Client, label string, iteration int) (int, error) {
	if client == nil || label == "" || iteration <= 0 {
		return 0, nil
	}

	beads, err := client.ListWithLabel(label)
	if err != nil {
		return 0, err
	}

	openNonEpicCount := 0
	for _, b := range beads {
		if b == nil {
			continue
		}
		if strings.EqualFold(b.Status, "closed") {
			continue
		}
		if strings.EqualFold(b.Type, "epic") {
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
		}

		router := provider.NewRouter(
			providers,
			cfg.Routing.PhasePreferences,
			cfg.Routing.Ratio,
			provider.ParseFallbackCooldown(cfg),
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

func resolveTrackerBackend(cfg *config.Config) string {
	if cfg == nil {
		return "bd"
	}
	return cfg.ResolveCompatibilityContext().TrackerBackend.Value
}

func resolveTrackerBackendDeprecationMarker(cfg *config.Config) string {
	if cfg == nil {
		return RunnerDeprecationMarkerLegacyTrackerBackendFallback
	}
	resolved := cfg.ResolveCompatibilityContext()
	if resolved.TrackerBackend.Source == config.CompatibilitySourceExplicit {
		return ""
	}
	return RunnerDeprecationMarkerLegacyTrackerBackendFallback
}

func newTrackerClient(backend string) (*bead.Client, error) {
	switch backend {
	case "bd":
		return bead.NewClient()
	default:
		return nil, fmt.Errorf("unsupported tracker backend: %s", backend)
	}
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
