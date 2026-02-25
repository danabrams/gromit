package main

import (
	"path/filepath"

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
