package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/spf13/cobra"
)

func TestResolveEffectiveModel_UsesConfigWhenFlagNotChanged(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Review: config.ReviewConfig{
			Thorough: config.ThoroughReviewConfig{
				Model: "sonnet",
			},
		},
	}

	cmd := &cobra.Command{Use: "review"}
	cmd.Flags().String("model", "opus", "model override")
	// Don't change the flag, so Flag.Changed("model") returns false

	// Determine effective model: use config if flag not changed
	effectiveModel := cfg.Review.Thorough.Model // fallback
	if cmd.Flags().Changed("model") {
		effectiveModel = resolveInteractiveModel(cmd, "model")
	}

	if effectiveModel != "sonnet" {
		t.Errorf("effective model = %q, want %q", effectiveModel, "sonnet")
	}
}

func TestResolveEffectiveModel_UsesFlagWhenChanged(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Review: config.ReviewConfig{
			Thorough: config.ThoroughReviewConfig{
				Model: "sonnet",
			},
		},
	}

	cmd := &cobra.Command{Use: "review"}
	cmd.Flags().String("model", "opus", "model override")
	if err := cmd.Flags().Set("model", "haiku"); err != nil {
		t.Fatalf("setting model flag: %v", err)
	}

	// Determine effective model: use flag if changed
	effectiveModel := cfg.Review.Thorough.Model // fallback
	if cmd.Flags().Changed("model") {
		effectiveModel = resolveInteractiveModel(cmd, "model")
	}

	if effectiveModel != "haiku" {
		t.Errorf("effective model = %q, want %q", effectiveModel, "haiku")
	}
}
