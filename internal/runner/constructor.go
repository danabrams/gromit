package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/runner/validation"
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
	"low":    "gpt-5.3-codex",
}

func newRunnerImpl(cfg *config.Config, output io.Writer) (*Runner, *reviewpkg.Reviewer, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("config is nil")
	}
	if output == nil {
		output = os.Stdout
	}

	log, err := logger.NewLogger(cfg.Paths.Logs)
	if err != nil {
		_, _ = fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
	}

	gromitDir := filepath.Dir(cfg.Paths.Templates)
	mainDir := filepath.Dir(gromitDir)

	renderer, err := prompt.NewRenderer(
		cfg.Paths.Templates,
		cfg.Paths.Specs,
		cfg.Paths.ProjectClaudeMD,
		gromitDir,
	)
	if err != nil {
		return nil, nil, err
	}
	renderer.SetMaxLearningChars(cfg.Learnings.MaxLearningChars)
	renderer.SetBudgetConfig(cfg.Prompt.Budget.MaxChars, cfg.Prompt.Budget.LearningCapChars)

	beadsClient, err := bead.NewClient()
	if err != nil {
		return nil, nil, err
	}

	syncOut := newSyncWriter(output)

	router, learningsProvider, sf, err := buildRouterAndLearningsProvider(cfg, gromitDir, output)
	if err != nil {
		return nil, nil, err
	}

	wireLearningsFilter(renderer, learningsProvider)

	analyzerObj, err := analyzer.NewAnalyzer(learningsProvider, cfg.Models.Validation, renderer)
	if err != nil {
		return nil, nil, err
	}

	stallTimeoutFn := makeStallTimeoutFn(cfg)
	inv := buildInvoker(router, syncOut, stallTimeoutFn, cfg)

	r := &Runner{
		cfg:            cfg,
		beads:          beadsClient,
		router:         router,
		invoker:        inv,
		analyzer:       analyzerObj,
		renderer:       renderer,
		logger:         log,
		output:         syncOut,
		syncOut:        syncOut,
		gromitDir:      gromitDir,
		stateFile:      sf,
		gitDiffFn:      getGitDiff,
		gitHeadFn:      getGitHead,
		cmdRunnerFn:    defaultCmdRunner,
		processChecker: IsProcessAlive,
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return net.DefaultResolver.LookupHost(ctx, host)
		},
		lookPathFn: exec.LookPath,
	}
	if cfg.Worktree.IsEnabled() {
		manager, mgrErr := worktree.NewManager(mainDir)
		if mgrErr != nil {
			return nil, nil, mgrErr
		}
		r.worktreeManager = manager
	}

	r.escalationHandler = escalation.NewHandler(cfg, analyzerObj, beadsClient, r.DecomposeTask, r.CreateSubBeads, r.log, r.showPartialProgress)
	r.validationRunner = validation.NewRunner(cfg, defaultCmdRunner, r.autoFixFn, r.makeValidationExecuteFn())
	r.methodologyExec = r.makeMethodologyExec()
	if cfg.Methodology.Granularity == config.MethodologyGranularitySpec {
		r.specOrchestrator = newSpecOrchestrator(r)
	}
	if cfg.SpecGate.IsEnabled() {
		gate, err := r.buildSpecGate()
		if err != nil {
			return nil, nil, err
		}
		r.specGate = gate
	}

	reviewer := reviewpkg.NewReviewer(cfg, router, beadsClient, renderer, r.gitDiffFn, log)
	reviewer.SetLogFn(r.log)
	reviewer.SetValidateFn(r.makeReviewValidateFn())

	return r, reviewer, nil
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

func buildRouterAndLearningsProvider(cfg *config.Config, gromitDir string, output io.Writer) (*provider.Router, provider.Provider, *state.File, error) {
	if cfg.HasProviders() {
		cfg.SetDefaults()
		cfg.NormalizeNilFields()

		providers, err := buildProvidersFromConfig(cfg)
		if err != nil {
			return nil, nil, nil, err
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
		)

		learningsProvider := selectLearningsProvider(providers)
		return router, learningsProvider, sf, nil
	}

	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return nil, nil, nil, err
	}
	claudeProvider := provider.NewClaudeProvider(claudeClient, defaultTierToModelMap)
	return provider.NewSingleProviderRouter(claudeProvider), claudeProvider, nil, nil
}

func buildProvidersFromConfig(cfg *config.Config) (map[string]provider.Provider, error) {
	providers := make(map[string]provider.Provider)
	for name, def := range cfg.Providers {
		switch {
		case name == "claude" || def.Binary == "claude":
			tierMap := def.Models
			if len(tierMap) == 0 {
				tierMap = defaultTierToModelMap
			}
			client, err := claude.NewClient(def.Binary, def.Flags, cfg.Claude.Timeout)
			if err != nil {
				return nil, err
			}
			providers[name] = provider.NewClaudeProvider(client, tierMap)
		case name == "codex" || name == "openai" || def.Binary == "codex":
			tierMap := def.Models
			if len(tierMap) == 0 {
				tierMap = defaultCodexTierToModelMap
			}
			providers[name] = provider.NewCodexProvider(def.Binary, def.Flags, tierMap)
		default:
			return nil, fmt.Errorf("unrecognized provider %q: supported providers are \"claude\" and \"codex\"", name)
		}
	}
	return providers, nil
}

func parseFallbackCooldown(cfg *config.Config) time.Duration {
	if !cfg.Routing.Fallback.Enabled || cfg.Routing.Fallback.Cooldown == "" {
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
