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
