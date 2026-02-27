package benchmark

type Phase4RunMetrics struct {
	MedianDiscoveryInputTokens int
	MedianDiscoveryLatencyMs   int
	SuccessRate                float64
	WrongFileRate              float64
}

type Phase4AdoptionGates struct {
	TokenReductionGate     bool
	LatencyReductionGate   bool
	SuccessRateParityGate  bool
	WrongFileRateGate      bool
	CanAdopt               bool
}

func EvaluatePhase4AdoptionGates(baseline, retrieval Phase4RunMetrics) Phase4AdoptionGates {
	gates := Phase4AdoptionGates{}

	// Token reduction gate: median discovery input tokens reduced by >= 20%
	tokenDelta := float64(baseline.MedianDiscoveryInputTokens-retrieval.MedianDiscoveryInputTokens) / float64(baseline.MedianDiscoveryInputTokens)
	gates.TokenReductionGate = tokenDelta >= 0.20

	// Latency reduction gate: median discovery latency reduced by >= 15%
	latencyDelta := float64(baseline.MedianDiscoveryLatencyMs-retrieval.MedianDiscoveryLatencyMs) / float64(baseline.MedianDiscoveryLatencyMs)
	gates.LatencyReductionGate = latencyDelta >= 0.15

	// Success rate parity gate: no more than 5% drop in success rate
	gates.SuccessRateParityGate = baseline.SuccessRate-retrieval.SuccessRate <= 0.05

	// Wrong-file rate gate: wrong-file rate must be <= 5%
	gates.WrongFileRateGate = retrieval.WrongFileRate <= 0.05

	// All gates must pass to adopt
	gates.CanAdopt = gates.TokenReductionGate && gates.LatencyReductionGate && gates.SuccessRateParityGate && gates.WrongFileRateGate

	return gates
}
