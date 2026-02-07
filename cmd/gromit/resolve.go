package main

import "github.com/danabrams/gromit/internal/config"

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
