package runner

import (
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/tracker"
)

func resolveTrackerBackend(cfg *config.Config) string {
	if cfg == nil {
		return "bd"
	}
	return cfg.ResolveCompatibilityContext().TrackerBackend.Value
}

func resolveTrackerBackendDeprecationMarker(cfg *config.Config) string {
	if cfg == nil {
		return RunnerDeprecationMarkerLegacyTrackerBackendFallback
	}
	resolved := cfg.ResolveCompatibilityContext()
	if resolved.TrackerBackend.Source == config.CompatibilitySourceExplicit {
		return ""
	}
	return RunnerDeprecationMarkerLegacyTrackerBackendFallback
}

func newTrackerClient(backend string) (tracker.Client, error) {
	switch backend {
	case "bd":
		beadClient, err := bead.NewClient()
		if err != nil {
			return nil, err
		}
		return bead.NewBDAdapter(beadClient), nil
	default:
		return nil, fmt.Errorf("unsupported tracker backend: %s", backend)
	}
}
