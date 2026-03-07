package routing_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/v2/routing"
)

func TestTierForPhaseUsesOverrideWhenConfigured(t *testing.T) {
	phaseModels := map[string]string{
		"build": "low",
		"red":   "low",
	}

	got := routing.TierForPhase("build", phaseModels, "medium")
	if got != "low" {
		t.Fatalf("TierForPhase(%q) = %q, want %q", "build", got, "low")
	}
}

func TestTierForPhaseFallsBackWhenNoOverride(t *testing.T) {
	phaseModels := map[string]string{
		"build": "low",
	}

	got := routing.TierForPhase("validate", phaseModels, "medium")
	if got != "medium" {
		t.Fatalf("TierForPhase(%q) = %q, want %q", "validate", got, "medium")
	}
}

func TestResolveModelMapsAbstractTierToProviderModel(t *testing.T) {
	models := map[string]string{
		"low":    "claude-haiku-4-5-20251001",
		"medium": "claude-sonnet-4-6",
		"high":   "claude-opus-4-6",
	}

	got := routing.ResolveModel("low", models)
	if got != "claude-haiku-4-5-20251001" {
		t.Fatalf("ResolveModel(%q) = %q, want %q", "low", got, "claude-haiku-4-5-20251001")
	}
}

func TestResolveModelReturnsTierWhenNoMapping(t *testing.T) {
	got := routing.ResolveModel("medium", map[string]string{})
	if got != "medium" {
		t.Fatalf("ResolveModel with empty map = %q, want %q", got, "medium")
	}
}

func TestTierForPhaseEmptyStringOverrideFallsBack(t *testing.T) {
	phaseModels := map[string]string{
		"build": "",
	}

	got := routing.TierForPhase("build", phaseModels, "medium")
	if got != "medium" {
		t.Fatalf("TierForPhase with empty string override = %q, want %q", got, "medium")
	}
}

func TestResolveModelEmptyStringModelFallsBack(t *testing.T) {
	models := map[string]string{
		"low": "",
	}

	got := routing.ResolveModel("low", models)
	if got != "low" {
		t.Fatalf("ResolveModel with empty string model = %q, want %q", got, "low")
	}
}
