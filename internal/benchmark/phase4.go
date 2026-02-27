package benchmark

import (
	"bufio"
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"os"
	stdstrings "strings"
)

type phase4PairedIterationRecord struct {
	DiscoveryInputTokensBaseline  int   `json:"discovery_input_tokens_baseline"`
	DiscoveryInputTokensRetrieval int   `json:"discovery_input_tokens_retrieval"`
	DiscoveryLatencyMsBaseline    int   `json:"discovery_latency_ms_baseline"`
	DiscoveryLatencyMsRetrieval   int   `json:"discovery_latency_ms_retrieval"`
	SuccessBaseline               bool  `json:"success_baseline"`
	SuccessRetrieval              bool  `json:"success_retrieval"`
	WrongFileRetrieval            bool  `json:"wrong_file_retrieval"`
}

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

func readPhase4PairedIterationRecords(path string) ([]phase4PairedIterationRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read phase-4 paired log %q: %w", path, err)
	}
	records := []phase4PairedIterationRecord{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := stdstrings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec phase4PairedIterationRecord
		if err := stdjson.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("decode phase-4 paired log line: %w", err)
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan phase-4 paired log: %w", err)
	}
	return records, nil
}
