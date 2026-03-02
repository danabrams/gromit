package main

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
)

// sessionLauncherFn matches the signature of the session worktree
// launcher functions so we can reuse session wiring logic across commands.
type sessionLauncherFn func(ctx context.Context, gromitDir string, command string, conflict sessionConflictSettings, callback func(sessionDir string) error) (*worktree.SessionWorktree, error)

func launchInSessionIfEnabled(
	cfg *config.Config,
	gromitDir string,
	command string,
	launcher sessionLauncherFn,
	callback func(sessionDir string) error,
	fallback func() error,
) error {
	return launchInSessionIfEnabledWithContext(context.Background(), cfg, gromitDir, command, launcher, callback, fallback)
}

func launchInSessionIfEnabledWithContext(
	ctx context.Context,
	cfg *config.Config,
	gromitDir string,
	command string,
	launcher sessionLauncherFn,
	callback func(sessionDir string) error,
	fallback func() error,
) error {
	if cfg != nil && !cfg.Worktree.IsEnabled() {
		if fallback == nil {
			return fmt.Errorf("worktree disabled but fallback is nil")
		}
		return fallback()
	}

	if launcher == nil {
		return fmt.Errorf("session launcher is nil")
	}
	if callback == nil {
		return fmt.Errorf("session callback is nil")
	}

	conflictSettings := sessionConflictSettingsFromConfig(cfg)
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := launcher(ctx, gromitDir, command, conflictSettings, callback)
	return err
}
