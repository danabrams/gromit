package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/cli"
	"github.com/danabrams/gromit/internal/events/stream"
	"github.com/danabrams/gromit/internal/v2/adapter"
	gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
	llm "github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/presenter"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/dep"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/loop"
	"github.com/danabrams/gromit/internal/v2/routing"
	v2spec "github.com/danabrams/gromit/internal/v2/spec"
	"github.com/spf13/cobra"
)

var run2Cmd = &cobra.Command{
	Use:   "run2 [spec-file]",
	Short: "Run the v2 spec loop for a specific spec",
	Long:  "Run the fresh v2 spec loop for a single spec file with dedicated adapters and event streaming.",
	Args:  run2Args,
	RunE:  run2,
}

func init() {
	run2Cmd.Flags().String("epic", "", "Run specs scoped to the specified epic")
	run2Cmd.Flags().Bool("from-review", false, "Run beads that were created from spec review findings")
	run2Cmd.Flags().String("spec", "", "Limit the from-review beads to the specified spec")
}

var (
	newSpecLoopFn = func(adapters adapter.AdapterSet, cfg *config.Config, gate loop.DependencyGate, opts ...loop.SpecLoopOption) (specLoop, error) {
		loopInstance, err := loop.NewSpecLoop(adapters, cfg, gate, opts...)
		if err != nil {
			return nil, err
		}
		return loopInstance, nil
	}
	startRun2SubscribersFn = startRun2Subscribers
	newSpecLoopEmitterFn   = func(emitter *events.Emitter) loop.SpecLoopOption {
		return loop.WithEmitter(emitter)
	}
	newRun2LoopComponentsFn = loop.NewRun2LoopComponents
	newTaskTrackerAdapterFn = func(client *bead.Client) tasktracker.TaskTracker {
		return tasktracker.NewBDAdapter(client)
	}
	runBeadLoopFn = func(runLoop *loop.BeadLoop, ctx context.Context, beads []*bead.Bead, stopCh <-chan struct{}) (loop.BeadLoopResult, error) {
		if runLoop == nil {
			return loop.BeadLoopResult{}, fmt.Errorf("bead loop required")
		}
		return runLoop.Run(ctx, beads, stopCh)
	}
)

type specLoop interface {
	Run(ctx context.Context, specID string, stopCh <-chan struct{}) error
}

func run2(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fromReview, err := cmd.Flags().GetBool("from-review")
	if err != nil {
		return fmt.Errorf("reading from-review flag: %w", err)
	}
	if fromReview {
		return run2FromReview(cmd, cfg)
	}

	specsDir := resolveSpecsDir(cfg)
	specFiles, err := resolveRun2Specs(cmd, args, specsDir)
	if err != nil {
		return fmt.Errorf("reading from-review flag: %w", err)
	}
	specFilter, err := cmd.Flags().GetString("spec")
	if err != nil {
		return fmt.Errorf("reading spec flag: %w", err)
	}
	specFilter = strings.TrimSpace(specFilter)

	specsDir := resolveSpecsDir(cfg)
	var specFiles []*v2spec.Spec
	if !fromReview {
		specFiles, err = resolveRun2Specs(cmd, args, specsDir)
		if err != nil {
			return err
		}
	}

	var gate loop.DependencyGate
	if !fromReview {
		gate, err = dep.NewSpecDependencyGate(specsDir)
		if err != nil {
			return fmt.Errorf("dependency gate: %w", err)
		}
	}

	planBinary := "claude"
	if strings.TrimSpace(cfg.Claude.Binary) != "" {
		planBinary = strings.TrimSpace(cfg.Claude.Binary)
	}
	planFlags := []string{}
	if len(cfg.Claude.Flags) > 0 {
		planFlags = append(planFlags, cfg.Claude.Flags...)
	}
	planTimeout := 15 * time.Minute
	if cfg.Claude.Timeout > 0 {
		planTimeout = time.Duration(cfg.Claude.Timeout) * time.Second
	}
	planProvider := llm.NewClaudeAdapter(planBinary, planFlags, planTimeout)
	llmAdapter, err := llm.NewPlanLLMAdapter(planProvider, specsDir)
	if err != nil {
		return fmt.Errorf("create plan adapter: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)
	worktreesDir := filepath.Join(gromitDir, "spec-worktrees")

	// Use an absolute repo root so the worktree paths returned by
	// Checkout() are always absolute, preventing git operations from
	// accidentally targeting the main repo when CWD changes.
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolving repo root: %w", err)
	}

	beadClient, err := newBeadClientFn()
	if err != nil {
		return fmt.Errorf("create bd client: %w", err)
	}
	// NOTE: The bd CLI (bead client) is an external binary that operates on
	// whatever repo its CWD points to. During spec runs, bd is NOT invoked
	// by gromit — any "bd: backup" commits on main are from manual or
	// scheduled bd invocations outside gromit's control. To prevent those
	// commits from landing on main during a spec run, the user should avoid
	// running bd backup while gromit run2 is executing.
	trackerAdapter, err := newTaskTrackerFn(beadClient)
	if err != nil {
		return fmt.Errorf("create task tracker: %w", err)
	}
	adapters := adapter.AdapterSet{
		Git:         gitadapter.NewExecGitAdapter(repoRoot, worktreesDir),
		LLM:         llmAdapter,
		TaskTracker: trackerAdapter,
		Presenter:   presenter.NewGitHubPresenter(nil),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, runSignalBufferSize)
	stopCh := make(chan struct{})
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go handleRunSignals(sigCh, stopCh, cancel, cmd.ErrOrStderr())

	emitter := events.NewEmitter()
	logsDir := resolveMainRepoLogsDirFn(gromitDir)
	wg, err := startRun2SubscribersFn(ctx, emitter, cmd.ErrOrStderr(), logsDir)
	if err != nil {
		return fmt.Errorf("starting subscribers: %w", err)
	}
	router, phaseModels := buildRouter(cfg)

	components, err := newRun2LoopComponentsFn(cfg, adapters, emitter, cmd.ErrOrStderr(), router, phaseModels)
	if err != nil {
		emitter.Close()
		wg.Wait()
		return fmt.Errorf("preparing run loop components: %w", err)
	}
	defer func() {
		// Close typed emitter first to stop subscriber goroutines from
		// producing new events, then close the legacy emitter.
		components.Emitter.Close()
		emitter.Close()
		wg.Wait()
	}()

	if fromReview {
		return runFromReview(ctx, cmd, cfg, adapters, components, stopCh, specFilter)
	}

	baseOpts := []loop.SpecLoopOption{
		newSpecLoopEmitterFn(emitter),
		loop.WithPlanStage(components.PlanStage),
		loop.WithPresentStage(components.PresentStage, components.PresentSummaryContext),
		loop.WithDecomposeStage(components.DecomposeStage),
		loop.WithBeadLoop(components.BeadLoop),
		loop.WithAcceptStage(components.AcceptStage),
		loop.WithSpecReviewStage(components.SpecReviewStage),
		loop.WithRemediationRunner(components.RemediationRunner),
		loop.WithSpecReviewStage(components.SpecReviewStage),
		loop.WithStageCommitter(components.StageCommitter),
		loop.WithTypedEmitter(components.TypedEmitter),
	)
	if router != nil {
		baseOpts = append(baseOpts, loop.WithRouter(router))
	}
	if len(phaseModels) > 0 {
		baseOpts = append(baseOpts, loop.WithPhaseModels(phaseModels))
	}
	for _, specFile := range specFiles {
		if err := specFile.CheckDependencies(specsDir); err != nil {
			return fmt.Errorf("checking dependencies for %s: %w", specFile.ID, err)
		}

		loopOpts := append([]loop.SpecLoopOption{
			loop.WithStageRecorder(newSpecLoopStageRecorder(emitter, specFile.ID)),
		}, baseOpts...)
		loopInstance, err := newSpecLoopFn(adapters, cfg, gate, loopOpts...)
		if err != nil {
			return fmt.Errorf("initializing spec loop: %w", err)
		}

		if err := loopInstance.Run(ctx, specFile.ID, stopCh); err != nil {
			return fmt.Errorf("running spec %s: %w", specFile.ID, err)
		}
	}

	return nil
}

func run2FromReview(cmd *cobra.Command, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config required")
	}

	parentCtx := cmd.Context()
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	specScope, err := cmd.Flags().GetString("spec")
	if err != nil {
		return fmt.Errorf("reading spec flag: %w", err)
	}
	specScope = strings.TrimSpace(specScope)

	specsDir := resolveSpecsDir(cfg)
	planBinary := "claude"
	if strings.TrimSpace(cfg.Claude.Binary) != "" {
		planBinary = strings.TrimSpace(cfg.Claude.Binary)
	}
	planFlags := []string{}
	if len(cfg.Claude.Flags) > 0 {
		planFlags = append(planFlags, cfg.Claude.Flags...)
	}
	planTimeout := 15 * time.Minute
	if cfg.Claude.Timeout > 0 {
		planTimeout = time.Duration(cfg.Claude.Timeout) * time.Second
	}
	planProvider := llm.NewClaudeAdapter(planBinary, planFlags, planTimeout)
	llmAdapter, err := llm.NewPlanLLMAdapter(planProvider, specsDir)
	if err != nil {
		return fmt.Errorf("create plan adapter: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)
	worktreesDir := filepath.Join(gromitDir, "spec-worktrees")
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolving repo root: %w", err)
	}

	beadClient, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("create bd client: %w", err)
	}
	trackerAdapter := newTaskTrackerAdapterFn(beadClient)
	adapters := adapter.AdapterSet{
		Git:         gitadapter.NewExecGitAdapter(repoRoot, worktreesDir),
		LLM:         llmAdapter,
		TaskTracker: trackerAdapter,
		Presenter:   presenter.NewGitHubPresenter(nil),
	}

	sigCh := make(chan os.Signal, runSignalBufferSize)
	stopCh := make(chan struct{})
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go handleRunSignals(sigCh, stopCh, cancel, cmd.ErrOrStderr())

	emitter := events.NewEmitter()
	logsDir := resolveMainRepoLogsDirFn(gromitDir)
	wg, err := startRun2SubscribersFn(ctx, emitter, cmd.ErrOrStderr(), logsDir)
	if err != nil {
		return fmt.Errorf("starting subscribers: %w", err)
	}

	router, phaseModels := buildRouter(cfg)

	components, err := newRun2LoopComponentsFn(cfg, adapters, emitter, cmd.ErrOrStderr(), router, phaseModels)
	if err != nil {
		emitter.Close()
		wg.Wait()
		return fmt.Errorf("preparing run loop components: %w", err)
	}
	defer func() {
		components.Emitter.Close()
		emitter.Close()
		wg.Wait()
	}()

	resp, err := adapters.TaskTracker.QueryBeads(ctx, tasktracker.TaskTrackerQueryBeadsRequest{
		Labels: fromReviewLabels(specScope),
		Status: "open",
	})
	if err != nil {
		return fmt.Errorf("query from-review beads: %w", err)
	}
	beads := trackerBeads(resp)
	beads = filterFromReviewBeads(beads, specScope)
	specSuffix := ""
	if specScope != "" {
		specSuffix = fmt.Sprintf(" for spec %q", specScope)
	}
	if len(beads) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No open from-review beads found.")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Found %d from-review bead(s)%s.\n", len(beads), specSuffix)

	if components.BeadLoop == nil {
		return fmt.Errorf("bead loop unavailable")
	}

	worktree := repoRoot
	if specScope != "" {
		worktree, err = adapters.Git.Checkout(ctx, specScope)
		if err != nil {
			return fmt.Errorf("checkout spec worktree: %w", err)
		}
	}
	components.BeadLoop.SetWorktree(worktree)
	_, err = runBeadLoopFn(components.BeadLoop, ctx, beads, stopCh)
	fmt.Fprintf(cmd.OutOrStdout(), "Executed %d from-review bead(s)%s through the bead loop.\n", len(beads), specSuffix)
	return err
}

func run2Args(cmd *cobra.Command, args []string) error {
	fromReview, err := cmd.Flags().GetBool("from-review")
	if err != nil {
		return fmt.Errorf("reading from-review flag: %w", err)
	}

	specScope, err := cmd.Flags().GetString("spec")
	if err != nil {
		return fmt.Errorf("reading spec flag: %w", err)
	}

	epicID, err := cmd.Flags().GetString("epic")
	if err != nil {
		return fmt.Errorf("reading epic flag: %w", err)
	}
	epicID = strings.TrimSpace(epicID)

	if specScope = strings.TrimSpace(specScope); specScope != "" && !fromReview {
		return fmt.Errorf("the --spec flag requires --from-review")
	}

	if fromReview {
		if epicID != "" {
			return fmt.Errorf("--from-review cannot be combined with --epic")
		}
		if len(args) > 0 {
			return fmt.Errorf("--from-review cannot be combined with a spec file")
		}
		return nil
	}

	if epicID != "" {
		if len(args) > 0 {
			return fmt.Errorf("the --epic flag cannot be combined with a spec file")
		}
		return nil
	}

	if len(args) != 1 {
		return fmt.Errorf("spec file argument required")
	}
	return nil
}

func runFromReview(ctx context.Context, cmd *cobra.Command, cfg *config.Config, adapters adapter.AdapterSet, components *loop.Run2LoopComponents, stopCh <-chan struct{}, specFilter string) error {
	if adapters.TaskTracker == nil {
		return fmt.Errorf("task tracker adapter required for --from-review")
	}
	beads, err := loop.QueryFromReviewBeads(ctx, adapters.TaskTracker, specFilter)
	if err != nil {
		return err
	}
	if len(beads) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "No from-review beads found")
		return nil
	}
	return fmt.Errorf("runFromReview is not implemented yet")
}

func resolveRun2Specs(cmd *cobra.Command, args []string, specsDir string) ([]*v2spec.Spec, error) {
	epicID, err := cmd.Flags().GetString("epic")
	if err != nil {
		return nil, fmt.Errorf("reading epic flag: %w", err)
	}
	epicID = strings.TrimSpace(epicID)
	if epicID != "" {
		specs, err := loadEpicSpecs(specsDir, epicID)
		if err != nil {
			return nil, fmt.Errorf("resolving epic %q: %w", epicID, err)
		}
		return specs, nil
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("spec file argument required")
	}
	specFile, err := loadSpecFromArg(args[0])
	if err != nil {
		return nil, err
	}
	return []*v2spec.Spec{specFile}, nil
}

func loadEpicSpecs(specsDir, epicID string) ([]*v2spec.Spec, error) {
	if specsDir == "" {
		return nil, fmt.Errorf("specs dir required")
	}
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, fmt.Errorf("reading specs directory: %w", err)
	}
	var matches []*v2spec.Spec
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			continue
		}
		path := filepath.Join(specsDir, entry.Name())
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolving spec path %s: %w", path, err)
		}
		specFile, err := v2spec.Load(absPath)
		if err != nil {
			return nil, fmt.Errorf("loading spec %s: %w", absPath, err)
		}
		if specFile.Epic == epicID {
			matches = append(matches, specFile)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no specs found for epic %q", epicID)
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ID < matches[j].ID
	})
	return matches, nil
}

func loadSpecFromArg(arg string) (*v2spec.Spec, error) {
	specPath, err := filepath.Abs(arg)
	if err != nil {
		return nil, fmt.Errorf("resolving spec path: %w", err)
	}
	specFile, err := v2spec.Load(specPath)
	if err != nil {
		return nil, fmt.Errorf("loading spec %s: %w", specPath, err)
	}
	return specFile, nil
}

func fromReviewLabels(spec string) []string {
	labels := []string{"from-review"}
	spec = strings.TrimSpace(spec)
	if spec != "" {
		labels = append(labels, fmt.Sprintf("spec:%s", spec))
	}
	return labels
}

func filterFromReviewBeads(beads []*bead.Bead, spec string) []*bead.Bead {
	if len(beads) == 0 {
		return beads
	}
	specLabel := ""
	if trimmed := strings.TrimSpace(spec); trimmed != "" {
		specLabel = fmt.Sprintf("spec:%s", trimmed)
	}
	filtered := make([]*bead.Bead, 0, len(beads))
	for _, item := range beads {
		if item == nil {
			continue
		}
		if !bead.HasLabel(item.Labels, "from-review") {
			continue
		}
		if specLabel != "" && !bead.HasLabel(item.Labels, specLabel) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func trackerBeads(resp *tasktracker.TaskTrackerQueryBeadsResponse) []*bead.Bead {
	if resp == nil {
		return nil
	}
	result := make([]*bead.Bead, 0, len(resp.Beads))
	for _, item := range resp.Beads {
		beadCopy := &bead.Bead{
			ID:          item.ID,
			Title:       item.Title,
			Description: item.Description,
			Priority:    item.Priority,
			Labels:      append([]string(nil), item.Labels...),
			Status:      item.Status,
			DependsOn:   dependenciesFromStrings(item.DependsOn),
			BlockedBy:   dependenciesFromStrings(item.BlockedBy),
		}
		result = append(result, beadCopy)
	}
	return result
}

func dependenciesFromStrings(ids []string) []bead.Dependency {
	if len(ids) == 0 {
		return nil
	}
	deps := make([]bead.Dependency, 0, len(ids))
	for _, id := range ids {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			deps = append(deps, bead.Dependency{ID: trimmed})
		}
	}
	return deps
}

// buildRouter constructs a Router from the config's provider and routing
// settings. Returns (nil, phaseModels) when no providers are configured —
// the spec loop handles a nil Router gracefully, but phaseModels must
// always be extracted so TierForPhase can resolve configured tiers.
func buildRouter(cfg *config.Config) (*routing.Router, map[string]string) {
	if cfg == nil {
		return nil, nil
	}
	phaseModels := phaseModelsFromConfig(cfg.Methodology.PhaseModels)
	if len(cfg.Providers) == 0 {
		return nil, phaseModels
	}

	binary := "claude"
	if strings.TrimSpace(cfg.Claude.Binary) != "" {
		binary = strings.TrimSpace(cfg.Claude.Binary)
	}
	flags := []string{}
	if len(cfg.Claude.Flags) > 0 {
		flags = append(flags, cfg.Claude.Flags...)
	}
	timeout := 15 * time.Minute
	if cfg.Claude.Timeout > 0 {
		timeout = time.Duration(cfg.Claude.Timeout) * time.Second
	}

	providers := make(map[string]llmtypes.LLMProvider, len(cfg.Providers))
	models := make(map[string]map[string]string, len(cfg.Providers))
	for name, def := range cfg.Providers {
		provBinary := binary
		if strings.TrimSpace(def.Binary) != "" {
			provBinary = strings.TrimSpace(def.Binary)
		}
		provFlags := flags
		if len(def.Flags) > 0 {
			provFlags = append([]string(nil), def.Flags...)
		}
		if isCodexBinary(provBinary) {
			var opts []llm.CodexOption
			if len(def.ReasoningEffort) > 0 {
				opts = append(opts, llm.WithReasoningEffort(def.ReasoningEffort))
			}
			providers[name] = llm.NewCodexAdapter(provBinary, provFlags, timeout, opts...)
		} else {
			providers[name] = llm.NewClaudeAdapter(provBinary, provFlags, timeout)
		}
		if len(def.Models) > 0 {
			models[name] = def.Models
		}
	}

	var cooldown time.Duration
	if cd := strings.TrimSpace(cfg.Routing.Fallback.Cooldown); cd != "" {
		if parsed, err := time.ParseDuration(cd); err == nil {
			cooldown = parsed
		} else {
			log.Printf("WARNING: invalid routing.fallback.cooldown %q: %v; using default (0s)", cd, err)
		}
	}

	router := routing.NewRouter(routing.RouterConfig{
		Providers:        providers,
		PhasePreferences: cfg.Routing.PhasePreferences,
		Ratio:            cfg.Routing.Ratio,
		Cooldown:         cooldown,
		Models:           models,
	})

	return router, phaseModels
}

// isCodexBinary returns true if the binary name indicates a Codex CLI.
func isCodexBinary(binary string) bool {
	base := strings.ToLower(filepath.Base(binary))
	return strings.Contains(base, "codex")
}

// phaseModelsFromConfig converts the structured PhaseModelsConfig into a
// flat map[string]string suitable for routing.TierForPhase.
func phaseModelsFromConfig(pm config.PhaseModelsConfig) map[string]string {
	m := map[string]string{}
	if pm.Plan != "" {
		m["plan"] = pm.Plan
	}
	if pm.Decompose != "" {
		m["decompose"] = pm.Decompose
	}
	if pm.Build != "" {
		m["build"] = pm.Build
	}
	if pm.Red != "" {
		m["red"] = pm.Red
	}
	if pm.Green != "" {
		m["green"] = pm.Green
	}
	if pm.Refactor != "" {
		m["refactor"] = pm.Refactor
	}
	if pm.Validate != "" {
		m["validate"] = pm.Validate
	}
	if pm.Review != "" {
		m["review"] = pm.Review
	}
	if pm.Accept != "" {
		m["accept"] = pm.Accept
	}
	if pm.Triage != "" {
		m["triage"] = pm.Triage
	}
	if pm.Gate != "" {
		m["gate"] = pm.Gate
	}
	if pm.Epilogue != "" {
		m["epilogue"] = pm.Epilogue
	}
	return m
}

func startRun2Subscribers(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
	if emitter == nil {
		return nil, fmt.Errorf("emitter is nil")
	}
	if output == nil {
		output = io.Discard
	}

	wg := &sync.WaitGroup{}
	cliSubscriber := cli.NewCLISubscriber(cli.BasicWriter(output), emitter)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = cliSubscriber.Start(ctx)
	}()

	if strings.TrimSpace(logsDir) != "" {
		streamSubscriber, err := stream.NewFileSubscriber(logsDir, emitter)
		if err != nil {
			fmt.Fprintf(output, "Warning: could not start stream subscriber: %v\n", err)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = streamSubscriber.Start(ctx)
			}()
		}
	}

	return wg, nil
}

func newSpecLoopStageRecorder(emitter *events.Emitter, specID string) loop.StageRecorder {
	specID = strings.TrimSpace(specID)
	if emitter == nil {
		return nil
	}
	if specID == "" {
		specID = "spec"
	}
	return &specLoopStageRecorder{emitter: emitter, specID: specID}
}

type specLoopStageRecorder struct {
	emitter *events.Emitter
	specID  string
}

func (r *specLoopStageRecorder) RecordStage(name string) {
	if r == nil || r.emitter == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	r.emitter.Emit(&events.LogEvent{
		Level:   "info",
		Message: fmt.Sprintf("spec %s: stage %s", r.specID, name),
	})
}
