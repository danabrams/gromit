package acceptor

import (
	"testing"
	"time"
)

func TestComputeTimeout_BonusDelta(t *testing.T) {
	cfg := TimeoutConfig{
		BaseSeconds:         1,
		RateConstant:        1000,
		ComplexityBonusSecs: 12,
		HardMaximumSecs:     0,
	}

	diffBytes := 250
	simple := ComputeCriterionTimeout(cfg, diffBytes, "field X exists")
	complex := ComputeCriterionTimeout(cfg, diffBytes, "integration workflow across scenario")
	expected := time.Duration(cfg.ComplexityBonusSecs) * time.Second
	if got := complex - simple; got != expected {
		t.Fatalf("unexpected bonus delta %v, want %v", got, expected)
	}
}

func TestComputeTimeout_MaxCap(t *testing.T) {
	cfg := TimeoutConfig{
		BaseSeconds:         1,
		RateConstant:        1,
		ComplexityBonusSecs: 0,
		HardMaximumSecs:     5,
	}

	dur := ComputeCriterionTimeout(cfg, 100, "integration behavior")
	max := time.Duration(cfg.HardMaximumSecs) * time.Second
	if dur != max {
		t.Fatalf("timeout %v did not clamp to hard max %v", dur, max)
	}
}

func TestComputeTimeout_MinFloor(t *testing.T) {
	cfg := TimeoutConfig{
		BaseSeconds:         10,
		RateConstant:        1,
		ComplexityBonusSecs: 0,
		HardMaximumSecs:     0,
	}

	dur := ComputeCriterionTimeout(cfg, -1000, "field Y exists")
	min := time.Duration(cfg.BaseSeconds) * time.Second
	if dur != min {
		t.Fatalf("timeout %v did not respect base floor %v", dur, min)
	}
}
