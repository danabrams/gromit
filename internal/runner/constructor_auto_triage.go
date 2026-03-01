package runner

import (
	"path/filepath"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/tracker"
)

func newAutoTriageService(cfg *config.Config, gromitDir string, client tracker.Client, sf *state.File) SPCAutoTriageService {
	return newSPCAutoTriageService(
		buildSPCAutoTriagePaths(cfg, gromitDir),
		client,
		newStateCooldownStore(sf),
	)
}

func buildSPCAutoTriagePaths(cfg *config.Config, gromitDir string) []string {
	var paths []string
	if gromitDir != "" {
		paths = append(paths, filepath.Join(gromitDir, "metrics", "process_trend.json"))
	}
	if cfg != nil && cfg.Paths.Logs != "" {
		paths = append(paths, filepath.Join(cfg.Paths.Logs, "process_trend.json"))
	}
	return paths
}
