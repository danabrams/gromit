package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
)

func resolveGromitDir(cfg *config.Config) string {
	if cfg != nil && cfg.Paths.GromitDir != "" {
		return cfg.Paths.GromitDir
	}
	// Default to .gromit in current directory
	return ".gromit"
}

// resolveSpecsDir returns the specs directory path from config or default
func resolveSpecsDir(cfg *config.Config) string {
	if cfg != nil && cfg.Paths.Specs != "" {
		return cfg.Paths.Specs
	}
	return ".gromit/specs"
}

// resolvePlansDir returns the plans directory path from config or default
func resolvePlansDir(cfg *config.Config) string {
	if cfg != nil && cfg.Paths.Plans != "" {
		return cfg.Paths.Plans
	}
	return ".gromit/plans"
}

// resolveTemplatesDir returns the templates directory path from config or default
func resolveTemplatesDir(cfg *config.Config) string {
	if cfg != nil && cfg.Paths.Templates != "" {
		return cfg.Paths.Templates
	}
	return ".gromit/templates"
}

// resolveProjectClaudeMD returns the project CLAUDE.md path from config or default
func resolveProjectClaudeMD(cfg *config.Config) string {
	if cfg != nil && cfg.Paths.ProjectClaudeMD != "" {
		return cfg.Paths.ProjectClaudeMD
	}
	return "CLAUDE.md"
}

// resolveExperimentStateDir returns the experiment state directory from config or default
func resolveExperimentStateDir(cfg *config.Config) string {
	gromitDir := resolveGromitDir(cfg)
	return filepath.Join(gromitDir, "experiment-state")
}

func resolveMainRepoLogsDir(gromitDir string) string {
	localLogs := filepath.Join(gromitDir, "logs")
	if info, err := os.Stat(localLogs); err == nil && info.IsDir() {
		return localLogs
	}

	// Local logs dir doesn't exist — likely running from a session worktree.
	// Use git to find the main repo root via --git-common-dir.
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	output, err := cmd.Output()
	if err != nil {
		return localLogs // fallback to local path
	}

	gitCommonDir := strings.TrimSpace(string(output))
	if gitCommonDir == "" || gitCommonDir == ".git" {
		return localLogs // already in the main repo
	}

	absGitCommonDir, absErr := filepath.Abs(gitCommonDir)
	if absErr != nil {
		return localLogs
	}

	// gitCommonDir is something like /path/to/main-repo/.git
	// The main repo root is the parent directory.
	mainRepoRoot := filepath.Dir(absGitCommonDir)
	mainLogsDir := filepath.Join(mainRepoRoot, ".gromit", "logs")

	if info, err := os.Stat(mainLogsDir); err == nil && info.IsDir() {
		return mainLogsDir
	}

	return localLogs // fallback if main repo logs don't exist either
}

// setupRetroWorktreeLogsSymlink creates a symlink from worktree/.gromit/logs
// to the main repo's .gromit/logs directory. This ensures that retro running
// in a worktree (where .gromit/logs is gitignored and doesn't exist) can still
// access the main repo's logs for experiment evaluations.
func setupRetroWorktreeLogsSymlink(worktreeGromitDir, mainGromitDir string) error {
	if worktreeGromitDir == "" || mainGromitDir == "" {
		return nil // Skip if either path is empty
	}

	worktreeLogsPath := filepath.Join(worktreeGromitDir, "logs")
	mainLogsPath := filepath.Join(mainGromitDir, "logs")

	// Check if worktree logs already exist (skip if already set up)
	if info, err := os.Stat(worktreeLogsPath); err == nil {
		if info.IsDir() {
			return nil // logs directory already exists
		}
		// If it's a symlink or other file, proceed to replace it
		if err := os.Remove(worktreeLogsPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err // Some other error occurred
	}

	// Create symlink from worktree logs to main repo logs
	if err := os.Symlink(mainLogsPath, worktreeLogsPath); err != nil {
		return err
	}

	return nil
}

// ensureRetroWorktreeLogsSetup ensures that retro can access logs in a session worktree.
// DEPRECATED: Use ResolveRetroWorktreeLogsDir instead.
// It attempts to symlink the main repo's logs into the worktree, or determines
// the main repo logs path for use with retro.SetLogsDir.
// If worktreeGromitDir and mainGromitDir are provided, it creates a symlink.
// If mainGromitDir is empty, it tries to resolve the main repo via git.
func ensureRetroWorktreeLogsSetup(worktreeGromitDir, mainGromitDir string) error {
	_, err := ResolveRetroWorktreeLogsDir(worktreeGromitDir, mainGromitDir)
	return err
}

// prepareRetroWorktreeWithMainRepoLogs prepares retro to work in a session worktree
// by either symlinking the main repo's logs or determining the main repo logs path.
// DEPRECATED: Use ResolveRetroWorktreeLogsDir instead.
// It returns the logs directory path that should be used with retro.SetLogsDir.
func prepareRetroWorktreeWithMainRepoLogs(worktreeGromitDir, mainGromitDir string) (string, error) {
	return ResolveRetroWorktreeLogsDir(worktreeGromitDir, mainGromitDir)
}

// setupRetroLogsForWorktree is a convenience function that sets up retro logs for a
// worktree by calling prepareRetroWorktreeWithMainRepoLogs. If mainGromitDir is empty,
// it uses resolveMainRepoLogsDir to find the main repo's logs directory.
// DEPRECATED: Use ResolveRetroWorktreeLogsDir instead.
func setupRetroLogsForWorktree(worktreeGromitDir, mainGromitDir string) (string, error) {
	return ResolveRetroWorktreeLogsDir(worktreeGromitDir, mainGromitDir)
}

// ResolveRetroWorktreeLogsDir is the consolidated function for retro worktree logs resolution.
// It replaces setupRetroLogsForWorktree, prepareRetroWorktreeWithMainRepoLogs, and
// ensureRetroWorktreeLogsSetup with a single, clear interface.
//
// It attempts to set up logs access in a worktree (either via symlink or by resolving
// the main repo's logs path) and returns the path to use for logs.
func ResolveRetroWorktreeLogsDir(worktreeGromitDir, mainGromitDir string) (string, error) {
	if worktreeGromitDir == "" {
		return "", nil
	}

	// Try to set up a symlink from worktree logs to main repo logs
	if mainGromitDir != "" {
		_ = setupRetroWorktreeLogsSymlink(worktreeGromitDir, mainGromitDir)
		// Even if symlink creation fails, return the main repo logs path
		return filepath.Join(mainGromitDir, "logs"), nil
	}

	// If mainGromitDir is empty, resolve the main repo logs path via git
	return resolveMainRepoLogsDir(worktreeGromitDir), nil
}
