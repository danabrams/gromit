package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/procutil"
)

const integrationQueueGitCommandProcessCapacityWait = 1500 * time.Millisecond

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
	newIntegrationQueueScopedGateAdapterFn = func() (*integrationQueueScopedGateAdapter, error) {
		return &integrationQueueScopedGateAdapter{}, nil
	}
)

func newIntegrationQueueCoordinator(cfg *config.Config, gromitDir string) (Coordinator, error) {
	store, err := newRunnerIntegrationQueueStoreFn(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("initializing integration queue store: %w", err)
	}

	repoDir := filepath.Dir(gromitDir)
	gitopsAdapter, err := newIntegrationQueueGitOpsAdapterFn(repoDir, cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing integration queue gitops adapter: %w", err)
	}

	gateAdapter, _ := newIntegrationQueueScopedGateAdapterFn()
	if gateAdapter == nil {
		gateAdapter = &integrationQueueScopedGateAdapter{}
	}

	return integrationqueue.NewCoordinator(store, gitopsAdapter, gateAdapter), nil
}

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
