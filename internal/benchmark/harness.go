package benchmark

import (
	"context"
	"fmt"
)

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

type FinalReviewPolicy struct {
	Enabled        bool
	NonInteractive bool
	Tier           string
	ApplyFixes     bool
}

type ModeOverlay struct {
	Mode        string
	Provider    string
	ModelFamily string
	TierModels  OverlayTierModels
	BuildTierDefault      string
	ValidationTierDefault string
	FinalReview           FinalReviewPolicy
}

type ModeRunResult struct {
	Mode                       string
	FinalReviewRan             bool
	FinalValidationRan         bool
	FinalValidationAfterReview bool
	FinalValidationPassed      bool
}

type RunModesInput struct {
	Manifest       HarnessManifest
	SelectedBeads  []string
	BaseCommitHint string
	Resolver       BaseCommitResolver
	Runner         ModeWorktreeRunner
}

func RunModesInIsolatedWorktrees(ctx context.Context, input RunModesInput) ([]ModeWorktreeRun, string, error) {
	if input.Resolver == nil {
		return nil, "", fmt.Errorf("base commit resolver is required")
	}
	if input.Runner == nil {
		return nil, "", fmt.Errorf("mode runner is required")
	}

	baseCommit, err := input.Resolver.ResolveBaseCommit(ctx, input.BaseCommitHint)
	if err != nil {
		return nil, "", err
	}

	modes := []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"}
	runs := make([]ModeWorktreeRun, 0, len(modes))
	for _, mode := range modes {
		overlay, err := BuildModeOverlay(input.Manifest, mode)
		if err != nil {
			return nil, "", err
		}
		req := ModeWorktreeRequest{
			Mode:          mode,
			BaseCommit:    baseCommit,
			SelectedBeads: append([]string(nil), input.SelectedBeads...),
			Overlay:       overlay,
		}
		run, err := input.Runner.RunMode(ctx, req)
		if err != nil {
			return nil, "", err
		}
		runs = append(runs, run)
	}
	return runs, baseCommit, nil
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
			BuildTierDefault:      "low",
			ValidationTierDefault: "low",
			FinalReview: FinalReviewPolicy{
				Enabled:        true,
				NonInteractive: true,
				Tier:           "high",
				ApplyFixes:     true,
			},
		}, nil
	default:
		return ModeOverlay{}, fmt.Errorf("unsupported benchmark mode %q", mode)
	}
}

func FinalizeModeRunResult(overlay ModeOverlay, finalValidationPassed bool) ModeRunResult {
	return ModeRunResult{
		Mode:                       overlay.Mode,
		FinalReviewRan:             overlay.FinalReview.Enabled,
		FinalValidationRan:         true,
		FinalValidationAfterReview: overlay.FinalReview.Enabled,
		FinalValidationPassed:      finalValidationPassed,
	}
}
