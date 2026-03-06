package main

import (
	"context"
	"fmt"
	"io"
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
	v2remediation "github.com/danabrams/gromit/internal/v2"
	"github.com/danabrams/gromit/internal/v2/adapter"
	gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
	llm "github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/presenter"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/dep"
	"github.com/danabrams/gromit/internal/v2/loop"
	v2spec "github.com/danabrams/gromit/internal/v2/spec"
	acceptstage "github.com/danabrams/gromit/internal/v2/stage/accept"
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
)

type specLoop interface {
	Run(ctx context.Context, specID string, stopCh <-chan struct{}) error
}

type remediationBeadRunner struct {
	loop *loop.BeadLoop
}

func (r remediationBeadRunner) Run(ctx context.Context, beads []*bead.Bead) error {
	if r.loop == nil {
		return fmt.Errorf("bead loop required")
	}
	return r.loop.Run(ctx, beads, nil)
}

func run2(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	specsDir := resolveSpecsDir(cfg)
	specFiles, err := resolveRun2Specs(cmd, args, specsDir)
	if err != nil {
		return err
	}

	gate, err := dep.NewSpecDependencyGate(specsDir)
	if err != nil {
		return fmt.Errorf("dependency gate: %w", err)
	}

	llmAdapter, err := llm.NewPlanLLMAdapter(cfg, specsDir)
	if err != nil {
		return fmt.Errorf("create plan adapter: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)
	worktreesDir := filepath.Join(gromitDir, "spec-worktrees")

	beadClient, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("create bd client: %w", err)
	}
	trackerAdapter := tasktracker.NewBDAdapter(beadClient)
	adapters := adapter.AdapterSet{
		Git:         gitadapter.NewExecGitAdapter(worktreesDir),
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
		emitter.Close()
		return fmt.Errorf("starting subscribers: %w", err)
	}
	defer func() {
		emitter.Close()
		wg.Wait()
	}()
	provider := newRun2LLMProvider(cfg)
	components, err := loop.NewRun2LoopComponents(cfg, adapters, trackerAdapter, provider, emitter, cmd.ErrOrStderr())
	if err != nil {
		return fmt.Errorf("preparing run loop components: %w", err)
	}
	defer components.Emitter.Close()

	acceptStage, err := acceptstage.New(cfg, adapters.Git, provider, "", "", "")
	if err != nil {
		return fmt.Errorf("create accept stage: %w", err)
	}
	remediationRunner := v2remediation.NewRemediationRunner(v2remediation.RemediationRunnerConfig{
		AcceptStage:    acceptStage,
		DecomposeStage: components.DecomposeStage,
		BeadRunner:     &remediationBeadRunner{loop: components.BeadLoop},
		GenerationCap:  3,
		Emitter:        emitter,
		Presenter:      adapters.Presenter,
	})

	baseOpts := []loop.SpecLoopOption{
		newSpecLoopEmitterFn(emitter),
		loop.WithPlanStage(components.PlanStage),
		loop.WithPresentStage(components.PresentStage, components.PresentSummaryContext),
		loop.WithDecomposeStage(components.DecomposeStage),
		loop.WithBeadLoop(components.BeadLoop),
		loop.WithAcceptStage(acceptStage),
		loop.WithRemediationRunner(remediationRunner),
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

func run2Args(cmd *cobra.Command, args []string) error {
	epicID, err := cmd.Flags().GetString("epic")
	if err != nil {
		return fmt.Errorf("reading epic flag: %w", err)
	}
	epicID = strings.TrimSpace(epicID)
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

func newRun2LLMProvider(cfg *config.Config) llm.LLMProvider {
	binary := "claude"
	if cfg != nil && strings.TrimSpace(cfg.Claude.Binary) != "" {
		binary = strings.TrimSpace(cfg.Claude.Binary)
	}
	flags := []string{}
	if cfg != nil && len(cfg.Claude.Flags) > 0 {
		flags = append(flags, cfg.Claude.Flags...)
	}
	timeout := 15 * time.Minute
	if cfg != nil && cfg.Claude.Timeout > 0 {
		timeout = time.Duration(cfg.Claude.Timeout) * time.Second
	}
	return llm.NewClaudeAdapter(binary, flags, timeout)
}
