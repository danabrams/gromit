package benchmark

import (
	"testing"
)

func TestPhase4EvaluateAdoptionGates_AllGatesPass_ReturnsAdopt(t *testing.T) {
	baseline := Phase4RunMetrics{
		MedianDiscoveryInputTokens: 100,
		MedianDiscoveryLatencyMs:   1000,
		SuccessRate:                0.95,
		WrongFileRate:              0.01,
	}
	retrieval := Phase4RunMetrics{
		MedianDiscoveryInputTokens: 70,  // 30% reduction
		MedianDiscoveryLatencyMs:   800,  // 20% reduction
		SuccessRate:                0.94, // 1% drop acceptable
		WrongFileRate:              0.02, // within threshold
	}

	gates := EvaluatePhase4AdoptionGates(baseline, retrieval)

	if !gates.TokenReductionGate {
		t.Errorf("TokenReductionGate should pass for 30%% reduction")
	}
	if !gates.LatencyReductionGate {
		t.Errorf("LatencyReductionGate should pass for 20%% reduction")
	}
	if !gates.SuccessRateParityGate {
		t.Errorf("SuccessRateParityGate should pass for 1%% drop")
	}
	if !gates.WrongFileRateGate {
		t.Errorf("WrongFileRateGate should pass for low wrong-file rate")
	}
	if !gates.CanAdopt {
		t.Errorf("CanAdopt should be true when all gates pass")
	}
}
