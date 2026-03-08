package routing_test

import (
	"bytes"
	"log"
	"strings"
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

func TestResolveModelUnmappedTierLogsWarningAndReturnsTierString(t *testing.T) {
	// Capture log output to verify the warning.
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0) // strip timestamps for easier matching
	defer func() {
		log.SetOutput(nil)
		log.SetFlags(log.LstdFlags)
	}()

	models := map[string]string{
		"low": "claude-haiku-4-5-20251001",
	}

	got := routing.ResolveModel("ultra", models)
	if got != "ultra" {
		t.Fatalf("ResolveModel(%q) = %q, want %q (raw tier as fallback)", "ultra", got, "ultra")
	}

	logOut := buf.String()
	if !strings.Contains(logOut, `no model mapping for tier "ultra"`) {
		t.Fatalf("expected warning log about unmapped tier, got: %q", logOut)
	}
}

func TestEscalationTierLowToMediumToHigh(t *testing.T) {
	cases := []struct {
		level int
		want  string
	}{
		{0, routing.TierLow},
		{1, routing.TierMedium},
		{2, routing.TierHigh},
		{3, routing.TierHigh}, // caps at highest
	}
	for _, tc := range cases {
		got := routing.EscalationTier(routing.TierLow, tc.level)
		if got != tc.want {
			t.Errorf("EscalationTier(low, %d) = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestEscalationTierMediumToHigh(t *testing.T) {
	if got := routing.EscalationTier(routing.TierMedium, 0); got != routing.TierMedium {
		t.Errorf("EscalationTier(medium, 0) = %q, want %q", got, routing.TierMedium)
	}
	if got := routing.EscalationTier(routing.TierMedium, 1); got != routing.TierHigh {
		t.Errorf("EscalationTier(medium, 1) = %q, want %q", got, routing.TierHigh)
	}
}

func TestEscalationTierHighStaysHigh(t *testing.T) {
	if got := routing.EscalationTier(routing.TierHigh, 0); got != routing.TierHigh {
		t.Errorf("EscalationTier(high, 0) = %q, want %q", got, routing.TierHigh)
	}
	if got := routing.EscalationTier(routing.TierHigh, 1); got != routing.TierHigh {
		t.Errorf("EscalationTier(high, 1) = %q, want %q", got, routing.TierHigh)
	}
}

func TestEscalationTierUnknownTierReturnsUnchanged(t *testing.T) {
	got := routing.EscalationTier("unknown", 1)
	if got != "unknown" {
		t.Errorf("EscalationTier(%q, 1) = %q, want %q", "unknown", got, "unknown")
	}
}
