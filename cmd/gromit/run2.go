package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/cli"
	"github.com/danabrams/gromit/internal/events/stream"
	"github.com/danabrams/gromit/internal/v2/adapter"
	gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
	llmadapter "github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/presenter"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/dep"
	"github.com/danabrams/gromit/internal/v2/loop"
	v2spec "github.com/danabrams/gromit/internal/v2/spec"
	"github.com/spf13/cobra"
)

var run2Cmd = &cobra.Command{
	Use:   "run2 <spec-file>",
	Short: "Run the v2 spec loop for a specific spec",
	Long:  "Run the fresh v2 spec loop for a single spec file with dedicated adapters and event streaming.",
	Args:  cobra.ExactArgs(1),
	RunE:  run2,
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
	Run(ctx context.Context, specID string) error
}

func run2(cmd *cobra.Command, args []string) error {
	specPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolving spec path: %w", err)
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	specsDir := resolveSpecsDir(cfg)
	specFile, err := v2spec.Load(specPath)
	if err != nil {
		return fmt.Errorf("loading spec %s: %w", specPath, err)
	}
	if err := specFile.CheckDependencies(specsDir); err != nil {
		return fmt.Errorf("checking dependencies: %w", err)
	}

	gate, err := dep.NewSpecDependencyGate(specsDir)
	if err != nil {
		return fmt.Errorf("dependency gate: %w", err)
	}

	llmAdapter, err := llmadapter.NewPlanLLMAdapter(cfg, specsDir)
	if err != nil {
		return fmt.Errorf("create plan adapter: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)
	worktreesDir := filepath.Join(gromitDir, "spec-worktrees")

	adapters := adapter.AdapterSet{
		Git:         gitadapter.NewExecGitAdapter(worktreesDir),
		LLM:         llmAdapter,
		TaskTracker: tasktracker.NewBDAdapter(nil),
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

	loopInstance, err := newSpecLoopFn(adapters, cfg, gate,
		loop.WithStageRecorder(newSpecLoopStageRecorder(emitter, specFile.ID)),
		newSpecLoopEmitterFn(emitter),
	)
	if err != nil {
		return fmt.Errorf("initializing spec loop: %w", err)
	}

	if err := loopInstance.Run(ctx, specFile.ID); err != nil {
		return fmt.Errorf("running spec %s: %w", specFile.ID, err)
	}

	return nil
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
