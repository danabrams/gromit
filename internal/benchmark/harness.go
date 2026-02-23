package benchmark

import "fmt"

type HarnessManifest struct {
	Provider        string
	ModelFamily     string
	LowTierModel    string
	MediumTierModel string
	HighTierModel   string
}

type OverlayTierModels struct {
	Low    string
	Medium string
	High   string
}

type ModeOverlay struct {
	Mode        string
	Provider    string
	ModelFamily string
	TierModels  OverlayTierModels
}

func BuildModeOverlay(manifest HarnessManifest, mode string) (ModeOverlay, error) {
	switch mode {
	case "single_pass", "tdd_shared_context", "tdd_fresh_context":
		return ModeOverlay{
			Mode:        mode,
			Provider:    manifest.Provider,
			ModelFamily: manifest.ModelFamily,
			TierModels: OverlayTierModels{
				Low:    manifest.LowTierModel,
				Medium: manifest.MediumTierModel,
				High:   manifest.HighTierModel,
			},
		}, nil
	default:
		return ModeOverlay{}, fmt.Errorf("unsupported benchmark mode %q", mode)
	}
}
