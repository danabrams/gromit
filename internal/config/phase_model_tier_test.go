package config

import "testing"

func TestConfigPhaseModelTier(t *testing.T) {
	cfg := &Config{
		Methodology: MethodologyConfig{
			PhaseModels: PhaseModelsConfig{
				Decompose: "high",
				Build:     "medium",
				Red:       "low",
				Green:     "",
				Refactor:  "high",
			},
		},
	}

	tests := []struct {
		name     string
		phase    string
		beadTier string
		want     string
	}{
		{name: "decompose override", phase: "decompose", beadTier: "low", want: "high"},
		{name: "build override", phase: "build", beadTier: "low", want: "medium"},
		{name: "red override", phase: "red", beadTier: "medium", want: "low"},
		{name: "green fallback when empty", phase: "green", beadTier: "medium", want: "medium"},
		{name: "refactor override", phase: "refactor", beadTier: "low", want: "high"},
		{name: "unknown fallback", phase: "unknown", beadTier: "medium", want: "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.PhaseModelTier(tt.phase, tt.beadTier); got != tt.want {
				t.Fatalf("PhaseModelTier(%q, %q) = %q, want %q", tt.phase, tt.beadTier, got, tt.want)
			}
		})
	}
}
