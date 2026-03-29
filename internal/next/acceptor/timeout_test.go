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
