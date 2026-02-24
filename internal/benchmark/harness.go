package benchmark

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdstrings "strings"
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
	Mode                  string
	Provider              string
	ModelFamily           string
	TierModels            OverlayTierModels
	BuildStrategy         string
	FreshContextPerCycle  bool
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
	Modes          []string
	SelectedBeads  []string
	BaseCommitHint string
	Resolver       BaseCommitResolver
	Runner         ModeWorktreeRunner
}

var ensureBenchmarkBeadsOpenFn = ensureSelectedBeadsOpen

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

	modes := append([]string(nil), input.Modes...)
	if len(modes) == 0 {
		return nil, "", fmt.Errorf("at least one benchmark mode is required")
	}
	runs := make([]ModeWorktreeRun, 0, len(modes))
	for _, mode := range modes {
		if err := ensureBenchmarkBeadsOpenFn(ctx, input.SelectedBeads); err != nil {
			return nil, "", err
		}
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
			if run.Cleanup != nil {
				if cleanupErr := run.Cleanup(); cleanupErr != nil {
					return nil, "", fmt.Errorf("cleanup failed after mode error: %w", cleanupErr)
				}
			}
			return nil, "", err
		}
		deterministicLogPath, err := persistModeLog(run.Mode, run.LogPath)
		if err != nil {
			return nil, "", err
		}
		run.LogPath = deterministicLogPath
		if run.Cleanup != nil {
			if err := run.Cleanup(); err != nil {
				return nil, "", err
			}
		}
		runs = append(runs, run)
	}
	return runs, baseCommit, nil
}

func ensureSelectedBeadsOpen(ctx context.Context, beadIDs []string) error {
	for _, rawID := range beadIDs {
		id := stdstrings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, "bd", "update", id, "--status", "open", "--json")
		if output, err := cmd.CombinedOutput(); err != nil {
			msg := stdstrings.TrimSpace(string(output))
			if msg == "" {
				return fmt.Errorf("reopen selected bead %q before benchmark mode: %w", id, err)
			}
			return fmt.Errorf("reopen selected bead %q before benchmark mode: %w: %s", id, err, msg)
		}
	}
	return nil
}

func persistModeLog(mode, sourcePath string) (string, error) {
	if sourcePath == "" {
		return "", nil
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("read mode log %q: %w", sourcePath, err)
	}
	destPath := filepath.Join(".gromit", "benchmarks", "logs", mode+".jsonl")
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "", fmt.Errorf("create benchmark logs directory: %w", err)
	}
	if err := os.WriteFile(destPath, content, 0o644); err != nil {
		return "", fmt.Errorf("write deterministic mode log %q: %w", destPath, err)
	}
	return destPath, nil
}

func BuildModeOverlay(manifest HarnessManifest, mode string) (ModeOverlay, error) {
	switch mode {
	case "single_pass":
		return ModeOverlay{
			Mode:        mode,
			Provider:    manifest.Provider,
			ModelFamily: manifest.ModelFamily,
			TierModels: OverlayTierModels{
				Low:    manifest.LowTierModel,
				Medium: manifest.MediumTierModel,
				High:   manifest.HighTierModel,
			},
			BuildStrategy:         "single_pass",
			FreshContextPerCycle:  false,
			BuildTierDefault:      "low",
			ValidationTierDefault: "low",
			FinalReview: FinalReviewPolicy{
				Enabled:        true,
				NonInteractive: true,
				Tier:           "high",
				ApplyFixes:     true,
			},
		}, nil
	case "tdd_shared_context":
		return ModeOverlay{
			Mode:        mode,
			Provider:    manifest.Provider,
			ModelFamily: manifest.ModelFamily,
			TierModels: OverlayTierModels{
				Low:    manifest.LowTierModel,
				Medium: manifest.MediumTierModel,
				High:   manifest.HighTierModel,
			},
			BuildStrategy:         "tdd",
			FreshContextPerCycle:  false,
			BuildTierDefault:      "low",
			ValidationTierDefault: "low",
			FinalReview: FinalReviewPolicy{
				Enabled:        true,
				NonInteractive: true,
				Tier:           "high",
				ApplyFixes:     true,
			},
		}, nil
	case "tdd_fresh_context":
		return ModeOverlay{
			Mode:        mode,
			Provider:    manifest.Provider,
			ModelFamily: manifest.ModelFamily,
			TierModels: OverlayTierModels{
				Low:    manifest.LowTierModel,
				Medium: manifest.MediumTierModel,
				High:   manifest.HighTierModel,
			},
			BuildStrategy:         "tdd",
			FreshContextPerCycle:  true,
			BuildTierDefault:      "medium",
			ValidationTierDefault: "medium",
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
