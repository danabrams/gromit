package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/integrationqueue"
)

type integrationQueueGitCommandFn func(ctx context.Context, repoDir string, args ...string) (string, error)

type integrationQueueGitOpsAdapter struct {
	repoDir       string
	baseBranch    string
	pushTimeout   time.Duration
	runGitCommand integrationQueueGitCommandFn
}

var errGitPushFailed = errors.New("git push failed")

func (a *integrationQueueGitOpsAdapter) FetchAndRebase(ctx context.Context, entry integrationqueue.Entry) error {
	dir, err := a.requireRepoDir()
	if err != nil {
		return err
	}
	branch := strings.TrimSpace(entry.Branch)
	if branch == "" {
		return fmt.Errorf("entry branch is empty")
	}
	if a.runGitCommand == nil {
		return fmt.Errorf("git runner is not configured")
	}

	base := strings.TrimSpace(a.baseBranch)
	if base == "" {
		base = config.DefaultBaseBranch
	}

	if _, err := a.runGitCommand(ctx, dir, "fetch", "origin", base); err != nil {
		return fmt.Errorf("fetching %s: %w", base, err)
	}
	if _, err := a.runGitCommand(ctx, dir, "checkout", branch); err != nil {
		return fmt.Errorf("checkout branch %s: %w", branch, err)
	}
	if _, err := a.runGitCommand(ctx, dir, "rebase", base); err != nil {
		return fmt.Errorf("rebasing branch %s onto %s: %w", branch, base, err)
	}
	return nil
}

func (a *integrationQueueGitOpsAdapter) MergeToMain(ctx context.Context, entry integrationqueue.Entry) error {
	dir, err := a.requireRepoDir()
	if err != nil {
		return err
	}
	branch := strings.TrimSpace(entry.Branch)
	if branch == "" {
		return fmt.Errorf("entry branch is empty")
	}
	if a.runGitCommand == nil {
		return fmt.Errorf("git runner is not configured")
	}

	base := strings.TrimSpace(a.baseBranch)
	if base == "" {
		base = config.DefaultBaseBranch
	}

	if _, err := a.runGitCommand(ctx, dir, "checkout", base); err != nil {
		return fmt.Errorf("checkout %s: %w", base, err)
	}
	if _, err := a.runGitCommand(ctx, dir, "merge", "--ff-only", branch); err != nil {
		return fmt.Errorf("merging branch %s: %w", branch, err)
	}

	return nil
}

func (a *integrationQueueGitOpsAdapter) Push(ctx context.Context) error {
	dir, err := a.requireRepoDir()
	if err != nil {
		return err
	}
	if a.runGitCommand == nil {
		return fmt.Errorf("git runner is not configured")
	}

	base := strings.TrimSpace(a.baseBranch)
	if base == "" {
		base = config.DefaultBaseBranch
	}

	pushCtx := ctx
	if a.pushTimeout > 0 {
		var cancel context.CancelFunc
		pushCtx, cancel = context.WithTimeout(ctx, a.pushTimeout)
		defer cancel()
	}

	if _, err := a.runGitCommand(pushCtx, dir, "push", "origin", base); err != nil {
		return fmt.Errorf("pushing %s: %w", base, fmt.Errorf("%w: %v", errGitPushFailed, err))
	}
	return nil
}

func (a *integrationQueueGitOpsAdapter) Cleanup(ctx context.Context, entry integrationqueue.Entry) error {
	dir, err := a.requireRepoDir()
	if err != nil {
		return err
	}
	if strings.TrimSpace(entry.Branch) == "" {
		return fmt.Errorf("entry branch is empty")
	}
	if a.runGitCommand == nil {
		return fmt.Errorf("git runner is not configured")
	}

	if _, err := a.runGitCommand(ctx, dir, "branch", "-D", entry.Branch); err != nil {
		return fmt.Errorf("cleanup branch %s: %w", entry.Branch, err)
	}
	return nil
}

func (a *integrationQueueGitOpsAdapter) requireRepoDir() (string, error) {
	if a == nil {
		return "", fmt.Errorf("gitops adapter is not configured")
	}
	dir := strings.TrimSpace(a.repoDir)
	if dir == "" {
		return "", fmt.Errorf("repo dir is not configured")
	}
	return dir, nil
}

type integrationQueueScopedGateEvaluator func(ctx context.Context, entry integrationqueue.Entry) error

type integrationQueueScopedGateAdapter struct {
	evaluator integrationQueueScopedGateEvaluator
}

func (a *integrationQueueScopedGateAdapter) Run(ctx context.Context, entry integrationqueue.Entry) error {
	if a == nil || a.evaluator == nil || strings.TrimSpace(entry.Branch) == "" {
		return nil
	}
	if err := a.evaluator(ctx, entry); err != nil {
		return fmt.Errorf("scoped gate evaluation for %s failed: %w", entry.Branch, err)
	}
	return nil
}

var (
	_ integrationqueue.GitOps     = (*integrationQueueGitOpsAdapter)(nil)
	_ integrationqueue.ScopedGate = (*integrationQueueScopedGateAdapter)(nil)
)
