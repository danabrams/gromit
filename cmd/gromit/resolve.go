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

// resolveMainRepoLogsDirFn resolves the logs directory, falling back to the main
// repo's .gromit/logs when the local gromitDir/logs doesn't exist (e.g., in a worktree).
// Injected as a variable for testing.
var resolveMainRepoLogsDirFn = resolveMainRepoLogsDir

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

	// gitCommonDir is something like /path/to/main-repo/.git
	// The main repo root is the parent directory.
	mainRepoRoot := filepath.Dir(gitCommonDir)
	mainLogsDir := filepath.Join(mainRepoRoot, ".gromit", "logs")

	if info, err := os.Stat(mainLogsDir); err == nil && info.IsDir() {
		return mainLogsDir
	}

	return localLogs // fallback if main repo logs don't exist either
}
