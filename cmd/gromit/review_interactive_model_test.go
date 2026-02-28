package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/spf13/cobra"
)

func TestResolveEffectiveReviewModelUsesConfig(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "review"}
	cmd.Flags().String(reviewModelFlagName, "opus", "model override")

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			InteractiveModels: &config.InteractiveModelsConfig{
				Review: "sonnet",
			},
		},
	}

	if got := resolveEffectiveInteractiveModel(cmd, cfg, reviewSessionCommand, reviewModelFlagName); got != "sonnet" {
		t.Fatalf("resolveEffectiveInteractiveModel() = %q, want %q", got, "sonnet")
	}
}

func TestResolveEffectiveReviewModelUsesFlag(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "review"}
	cmd.Flags().String(reviewModelFlagName, "opus", "model override")
	if err := cmd.Flags().Set(reviewModelFlagName, "haiku"); err != nil {
		t.Fatalf("setting model flag: %v", err)
	}

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			InteractiveModels: &config.InteractiveModelsConfig{
				Review: "sonnet",
			},
		},
	}

	if got := resolveEffectiveInteractiveModel(cmd, cfg, reviewSessionCommand, reviewModelFlagName); got != "haiku" {
		t.Fatalf("resolveEffectiveInteractiveModel() = %q, want %q", got, "haiku")
	}
}
