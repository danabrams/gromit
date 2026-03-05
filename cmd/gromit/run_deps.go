package main

import (
	"context"
	"io"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/retro"
	"github.com/danabrams/gromit/internal/runner"
	"github.com/danabrams/gromit/internal/worktree"
)

var _ retro.ProviderRunner = (*provider.ClaudeProvider)(nil)

type runDeps struct {
	runInDedicatedWorktree func(context.Context, string, func() error) error

	newRunnerWithStageContext func(*config.Config, io.Writer, *runner.StageContext, ...string) (*runner.Orchestrator, error)
	newBuildSpecStageContext  func(context.Context, *config.Config, string, string) (*runner.StageContext, error)
	newSpecBranchCreator      func(string, *config.Config) (runner.SpecBranchCreator, error)
	runHasOpenBeadsForLabel   func(string) (bool, error)

	retroResolveAgent         func(*config.Config, string, string, bool, io.Reader, io.Writer) (agent.Agent, error)
	retroSessionLauncher      func(context.Context, string, string, sessionConflictSettings, func(sessionDir string) error) (*worktree.SessionWorktree, error)
	retroRecordState          func(string) error
	retroClaudeFallbackRunner func(*config.Config) (retro.ProviderRunner, error)
	resolveMainRepoLogsDir    func(string) string
}

func newRunDeps() runDeps {
	return runDeps{
		runInDedicatedWorktree:    runInDedicatedWorktree,
		newRunnerWithStageContext: runner.NewRunnerWithStageContext,
		newBuildSpecStageContext:  runner.BuildSpecStageContext,
		newSpecBranchCreator:      runner.SpecBranchCreatorFactory,
		runHasOpenBeadsForLabel:   hasOpenBeadsForLabel,
		retroResolveAgent:         agent.Resolve,
		retroSessionLauncher:      runWithSessionWorktreeWithConflictSettings,
		retroRecordState:          recordRetroState,
		retroClaudeFallbackRunner: defaultRetroClaudeFallbackRunner,
		resolveMainRepoLogsDir:    resolveMainRepoLogsDir,
	}
}

func defaultRetroClaudeFallbackRunner(cfg *config.Config) (retro.ProviderRunner, error) {
	opusTimeout, _, _, _ := cfg.Claude.TimeoutsForModel("opus")
	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, opusTimeout)
	if err != nil {
		return nil, err
	}
	return provider.NewClaudeProvider(claudeClient, provider.DefaultTierToModelMap), nil
}
