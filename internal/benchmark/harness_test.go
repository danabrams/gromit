package benchmark

import "testing"

func TestBuildModeOverlay_PinsProviderAndModelFamilyAcrossModes(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}

	for _, mode := range []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"} {
		overlay, err := BuildModeOverlay(manifest, mode)
		if err != nil {
			t.Fatalf("BuildModeOverlay(%q) error = %v", mode, err)
		}
		if overlay.Provider != manifest.Provider {
			t.Fatalf("mode %q provider = %q, want %q", mode, overlay.Provider, manifest.Provider)
		}
		if overlay.ModelFamily != manifest.ModelFamily {
			t.Fatalf("mode %q model_family = %q, want %q", mode, overlay.ModelFamily, manifest.ModelFamily)
		}
		if overlay.TierModels.Low != manifest.LowTierModel {
			t.Fatalf("mode %q low tier model = %q, want %q", mode, overlay.TierModels.Low, manifest.LowTierModel)
		}
		if overlay.TierModels.Medium != manifest.MediumTierModel {
			t.Fatalf("mode %q medium tier model = %q, want %q", mode, overlay.TierModels.Medium, manifest.MediumTierModel)
		}
		if overlay.TierModels.High != manifest.HighTierModel {
			t.Fatalf("mode %q high tier model = %q, want %q", mode, overlay.TierModels.High, manifest.HighTierModel)
		}
	}
}

func TestBuildModeOverlay_UsesLowTierDefaultsForBuildAndValidation(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}

	for _, mode := range []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"} {
		overlay, err := BuildModeOverlay(manifest, mode)
		if err != nil {
			t.Fatalf("BuildModeOverlay(%q) error = %v", mode, err)
		}
		if overlay.BuildTierDefault != "low" {
			t.Fatalf("mode %q build tier default = %q, want %q", mode, overlay.BuildTierDefault, "low")
		}
		if overlay.ValidationTierDefault != "low" {
			t.Fatalf("mode %q validation tier default = %q, want %q", mode, overlay.ValidationTierDefault, "low")
		}
	}
}

func TestBuildModeOverlay_ConfiguresFinalHighTierReviewWithApplyFixes(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}

	overlay, err := BuildModeOverlay(manifest, "single_pass")
	if err != nil {
		t.Fatalf("BuildModeOverlay() error = %v", err)
	}
	if !overlay.FinalReview.Enabled {
		t.Fatal("final review enabled = false, want true")
	}
	if !overlay.FinalReview.NonInteractive {
		t.Fatal("final review non_interactive = false, want true")
	}
	if overlay.FinalReview.Tier != "high" {
		t.Fatalf("final review tier = %q, want %q", overlay.FinalReview.Tier, "high")
	}
	if !overlay.FinalReview.ApplyFixes {
		t.Fatal("final review apply_fixes = false, want true")
	}
}

func TestFinalizeModeRunResult_RecordsFinalValidationAfterReview(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}
	overlay, err := BuildModeOverlay(manifest, "single_pass")
	if err != nil {
		t.Fatalf("BuildModeOverlay() error = %v", err)
	}

	result := FinalizeModeRunResult(overlay, false)
	if !result.FinalReviewRan {
		t.Fatal("final review ran = false, want true")
	}
	if !result.FinalValidationRan {
		t.Fatal("final validation ran = false, want true")
	}
	if !result.FinalValidationAfterReview {
		t.Fatal("final validation after review = false, want true")
	}
	if result.FinalValidationPassed {
		t.Fatal("final validation passed = true, want false")
	}
}
