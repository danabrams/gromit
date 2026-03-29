package acceptor

import (
	"testing"
	"time"
)

func TestComputeCriterionTimeout(t *testing.T) {
	cfg := DefaultTimeoutConfig()

	simpleCriterion := "type X exists in package Y"
	simpleTimeout := ComputeCriterionTimeout(cfg, 500000, simpleCriterion)
	if simpleTimeout < 100*time.Second || simpleTimeout > 200*time.Second {
		t.Fatalf("unexpected simple timeout: %v", simpleTimeout)
	}

	complexCriterion := "end-to-end pipeline survives resume"
	complexTimeout := ComputeCriterionTimeout(cfg, 500000, complexCriterion)
	expectedBonus := time.Duration(cfg.ComplexityBonusSecs) * time.Second
	if diff := complexTimeout - simpleTimeout; diff != expectedBonus {
		t.Fatalf("complex timeout diff = %v, want %v", diff, expectedBonus)
	}

	hugeDiffTimeout := ComputeCriterionTimeout(cfg, 3000000, "integration workflow across scenario")
	maxDuration := time.Duration(cfg.HardMaximumSecs) * time.Second
	if hugeDiffTimeout != maxDuration {
		t.Fatalf("huge diff timeout = %v, want hard max %v", hugeDiffTimeout, maxDuration)
	}

	minTimeout := ComputeCriterionTimeout(cfg, 0, simpleCriterion)
	if minTimeout < time.Duration(cfg.BaseSeconds)*time.Second {
		t.Fatalf("min timeout %v below base %d", minTimeout, cfg.BaseSeconds)
	}
}

func TestComputeCriterionTimeout_InvalidConfigFallback(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TimeoutConfig
		diff    int
		want    time.Duration
	}{
		{
			name: "zero BaseSeconds uses fallback 1",
			cfg: TimeoutConfig{
				BaseSeconds:         0,
				RateConstant:        1000,
				ComplexityBonusSecs: 0,
				HardMaximumSecs:     0,
			},
			diff: 0,
			want: 1 * time.Second,
		},
		{
			name: "negative BaseSeconds uses fallback 1",
			cfg: TimeoutConfig{
				BaseSeconds:         -5,
				RateConstant:        1000,
				ComplexityBonusSecs: 0,
				HardMaximumSecs:     0,
			},
			diff: 0,
			want: 1 * time.Second,
		},
		{
			name: "zero RateConstant uses fallback 1 (no division by zero)",
			cfg: TimeoutConfig{
				BaseSeconds:         10,
				RateConstant:        0,
				ComplexityBonusSecs: 0,
				HardMaximumSecs:     0,
			},
			diff: 500,
			want: time.Duration(10+500) * time.Second,
		},
		{
			name: "negative RateConstant uses fallback 1",
			cfg: TimeoutConfig{
				BaseSeconds:         10,
				RateConstant:        -100,
				ComplexityBonusSecs: 0,
				HardMaximumSecs:     0,
			},
			diff: 100,
			want: time.Duration(10+100) * time.Second,
		},
		{
			name: "both BaseSeconds and RateConstant zero use fallbacks",
			cfg: TimeoutConfig{
				BaseSeconds:         0,
				RateConstant:        0,
				ComplexityBonusSecs: 0,
				HardMaximumSecs:     0,
			},
			diff: 500,
			want: time.Duration(1+500) * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeCriterionTimeout(tt.cfg, tt.diff, "field X exists")
			if got != tt.want {
				t.Fatalf("ComputeCriterionTimeout = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeCriterionTimeout_HardMaxBelowBase(t *testing.T) {
	// When HardMaximumSecs < BaseSeconds, the hard max must win (cap to hard max,
	// not raise back to base). This verifies the fix for the clamp ordering bug.
	cfg := TimeoutConfig{
		BaseSeconds:         60,
		RateConstant:        1000,
		ComplexityBonusSecs: 0,
		HardMaximumSecs:     30, // hard max is below base
	}

	dur := ComputeCriterionTimeout(cfg, 0, "field X exists")
	want := time.Duration(cfg.HardMaximumSecs) * time.Second
	if dur != want {
		t.Fatalf("timeout = %v, want hard max %v (hard max must win over base)", dur, want)
	}
}

func TestClassifyCriterionComplexity(t *testing.T) {
	tests := []struct {
		name      string
		criterion string
		want      string
	}{
		{"end-to-end keyword", "End-to-end pipeline behavior", complexityComplex},
		{"pipeline keyword", "Pipeline integration scenario", complexityComplex},
		{"behavior keyword", "Behavior across workflow", complexityComplex},
		{"case insensitive", "SURvive resume sequence", complexityComplex},
		{"simple criterion", "type X exists in package Y", complexitySimple},
		{"simple without keywords", "field Y is present", complexitySimple},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyCriterionComplexity(tt.criterion); got != tt.want {
				t.Fatalf("complexity = %q, want %q", got, tt.want)
			}
		})
	}
}
