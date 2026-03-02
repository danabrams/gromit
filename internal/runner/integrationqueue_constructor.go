package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/procutil"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

const integrationQueueGitCommandProcessCapacityWait = 1500 * time.Millisecond

var errIntegrationQueueStoreInit = errors.New("integration queue store initialization failed")

var (
	newRunnerIntegrationQueueStoreFn = func(gromitDir string) (*integrationqueue.Store, error) {
		return integrationqueue.NewStore(gromitDir)
	}
	newIntegrationQueueGitOpsAdapterFn = func(repoDir string, cfg *config.Config) (*integrationQueueGitOpsAdapter, error) {
		repoDir = strings.TrimSpace(repoDir)
		if repoDir == "" {
			return nil, fmt.Errorf("repo directory is empty")
		}
		info, err := os.Stat(repoDir)
		if err != nil {
			return nil, fmt.Errorf("accessing repo directory %s: %w", repoDir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("repo directory %s is not a directory", repoDir)
		}

		baseBranch := ""
		pushTimeout := time.Duration(0)
		if cfg != nil {
			baseBranch = cfg.Git.BaseBranch
			pushTimeout = cfg.Git.PushTimeoutDuration()
		}

		return &integrationQueueGitOpsAdapter{
			repoDir:       repoDir,
			baseBranch:    baseBranch,
			pushTimeout:   pushTimeout,
			runGitCommand: runIntegrationQueueGitCommand,
		}, nil
	}
	newIntegrationQueueScopedGateAdapterFn = func(cfg *config.Config, repoDir string) (*integrationQueueScopedGateAdapter, error) {
		return &integrationQueueScopedGateAdapter{
			evaluator: newIntegrationQueueScopedGateEvaluator(cfg, repoDir, nil),
		}, nil
	}
	newIntegrationQueueCoordinatorFn = integrationqueue.NewCoordinator
)

func newIntegrationQueueScopedGateEvaluator(cfg *config.Config, repoDir string, cmdRunner runtypes.CmdRunnerFn) integrationQueueScopedGateEvaluator {
	if cfg == nil || !cfg.Validation.Enabled {
		return nil
	}

	commands := cfg.Validation.FastCommandsOrDefault()
	if len(commands) == 0 {
		return nil
	}

	if cmdRunner == nil {
		cmdRunner = defaultCmdRunner
	}

	dir := strings.TrimSpace(repoDir)
	if dir == "" {
		dir = "."
	}

	return func(ctx context.Context, entry integrationqueue.Entry) error {
		if strings.TrimSpace(entry.Branch) == "" {
			return nil
		}

		runner := validation.NewRunner(cfg, cmdRunner, nil, nil)
		result, err := runner.RunDirect(ctx, commands, dir)
		if err != nil {
			return fmt.Errorf("validation invocation: %w", err)
		}
		if result == nil {
			return fmt.Errorf("validation returned no result")
		}
		if !result.Success {
			output := strings.TrimSpace(result.Output)
			if output == "" {
				output = "validation failed"
			}
			return fmt.Errorf("validation failed: %s", output)
		}
		return nil
	}
}

func newIntegrationQueueCoordinator(cfg *config.Config, gromitDir string) (Coordinator, error) {
	store, err := newRunnerIntegrationQueueStoreFn(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("initializing integration queue store: %w", errors.Join(errIntegrationQueueStoreInit, err))
	}

	repoDir := filepath.Dir(gromitDir)
	gitopsAdapter, err := newIntegrationQueueGitOpsAdapterFn(repoDir, cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing integration queue gitops adapter: %w", err)
	}

	gateAdapter, err := newIntegrationQueueScopedGateAdapterFn(cfg, repoDir)
	if err != nil {
		return nil, fmt.Errorf("initializing integration queue scoped gate adapter: %w", err)
	}
	if gateAdapter == nil {
		gateAdapter = &integrationQueueScopedGateAdapter{}
	}

	return newIntegrationQueueCoordinatorFn(store, gitopsAdapter, gateAdapter), nil
}

var _ integrationqueue.ScopedGate = (*integrationQueueScopedGateAdapter)(nil)

func runIntegrationQueueGitCommand(ctx context.Context, repoDir string, args ...string) (string, error) {
	if err := procutil.WaitForProcessCapacity(ctx, integrationQueueGitCommandProcessCapacityWait); err != nil {
		return "", fmt.Errorf("waiting for process capacity: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir

	procutil.SetProcessGroupKill(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", err
	}

	procutil.KillDescendantsOnCancel(ctx, cmd)
	defer procutil.ReapProcessTree(cmd)

	if err := cmd.Wait(); err != nil {
		return stdout.String() + "\n" + stderr.String(), err
	}

	return stdout.String(), nil
}
