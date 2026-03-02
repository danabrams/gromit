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
	DiscoveryInputTokensBaseline  int  `json:"discovery_input_tokens_baseline"`
	DiscoveryInputTokensRetrieval int  `json:"discovery_input_tokens_retrieval"`
	DiscoveryLatencyMsBaseline    int  `json:"discovery_latency_ms_baseline"`
	DiscoveryLatencyMsRetrieval   int  `json:"discovery_latency_ms_retrieval"`
	SuccessBaseline               bool `json:"success_baseline"`
	SuccessRetrieval              bool `json:"success_retrieval"`
	WrongFileRetrieval            bool `json:"wrong_file_retrieval"`
}

type Phase4RunMetrics struct {
	MedianDiscoveryInputTokens int
	MedianDiscoveryLatencyMs   int
	SuccessRate                float64
	WrongFileRate              float64
}

type Phase4AdoptionGates struct {
	TokenReductionGate    bool
	LatencyReductionGate  bool
	SuccessRateParityGate bool
	WrongFileRateGate     bool
	CanAdopt              bool
}

type Phase4AdoptionDecision struct {
	ShouldAdopt bool
	Reasons     []string
}

type Phase4MeasurementReport struct {
	Baseline  Phase4RunMetrics
	Retrieval Phase4RunMetrics
	Gates     Phase4AdoptionGates
}

func ComputePhase4AdoptionDecision(gates Phase4AdoptionGates) Phase4AdoptionDecision {
	reasons := make([]string, 0)

	if !gates.TokenReductionGate {
		reasons = append(reasons, "median_discovery_token_reduction_below_20_percent")
	}
	if !gates.LatencyReductionGate {
		reasons = append(reasons, "median_discovery_latency_reduction_below_15_percent")
	}
	if !gates.SuccessRateParityGate {
		reasons = append(reasons, "success_rate_drop_exceeds_5_percent")
	}
	if !gates.WrongFileRateGate {
		reasons = append(reasons, "wrong_file_rate_exceeds_5_percent")
	}

	return Phase4AdoptionDecision{
		ShouldAdopt: gates.CanAdopt,
		Reasons:     reasons,
	}
}

func EvaluatePhase4AdoptionGates(baseline, retrieval Phase4RunMetrics) Phase4AdoptionGates {
	gates := Phase4AdoptionGates{}

	// Token reduction gate: median discovery input tokens reduced by >= 20%
	if baseline.MedianDiscoveryInputTokens == 0 {
		gates.TokenReductionGate = false // fail gate if baseline is zero
	} else {
		tokenDelta := float64(baseline.MedianDiscoveryInputTokens-retrieval.MedianDiscoveryInputTokens) / float64(baseline.MedianDiscoveryInputTokens)
		gates.TokenReductionGate = tokenDelta >= 0.20
	}

	// Latency reduction gate: median discovery latency reduced by >= 15%
	if baseline.MedianDiscoveryLatencyMs == 0 {
		gates.LatencyReductionGate = false // fail gate if baseline is zero
	} else {
		latencyDelta := float64(baseline.MedianDiscoveryLatencyMs-retrieval.MedianDiscoveryLatencyMs) / float64(baseline.MedianDiscoveryLatencyMs)
		gates.LatencyReductionGate = latencyDelta >= 0.15
	}

	// Success rate parity gate: no more than 5% drop in success rate
	gates.SuccessRateParityGate = baseline.SuccessRate-retrieval.SuccessRate <= 0.05

	// Wrong-file rate gate: wrong-file rate must be <= 5%
	gates.WrongFileRateGate = retrieval.WrongFileRate <= 0.05

	// All gates must pass to adopt
	gates.CanAdopt = gates.TokenReductionGate && gates.LatencyReductionGate && gates.SuccessRateParityGate && gates.WrongFileRateGate

	return gates
}

func RunPhase4Measurement(logPath string) (Phase4MeasurementReport, error) {
	records, err := readPhase4PairedIterationRecords(logPath)
	if err != nil {
		return Phase4MeasurementReport{}, err
	}

	baseline := summarizePhase4BaselineRun(records)
	retrieval := summarizePhase4RetrievalRun(records)
	gates := EvaluatePhase4AdoptionGates(baseline, retrieval)

	return Phase4MeasurementReport{
		Baseline:  baseline,
		Retrieval: retrieval,
		Gates:     gates,
	}, nil
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

func summarizePhase4BaselineRun(records []phase4PairedIterationRecord) Phase4RunMetrics {
	if len(records) == 0 {
		return Phase4RunMetrics{}
	}

	tokens := make([]int, 0, len(records))
	latencies := make([]int, 0, len(records))
	successCount := 0

	for _, rec := range records {
		tokens = append(tokens, rec.DiscoveryInputTokensBaseline)
		latencies = append(latencies, rec.DiscoveryLatencyMsBaseline)
		if rec.SuccessBaseline {
			successCount++
		}
	}

	return Phase4RunMetrics{
		MedianDiscoveryInputTokens: MedianInt(tokens),
		MedianDiscoveryLatencyMs:   MedianInt(latencies),
		SuccessRate:                float64(successCount) / float64(len(records)),
		WrongFileRate:              0, // baseline doesn't track wrong-file rate
	}
}

func summarizePhase4RetrievalRun(records []phase4PairedIterationRecord) Phase4RunMetrics {
	if len(records) == 0 {
		return Phase4RunMetrics{}
	}

	tokens := make([]int, 0, len(records))
	latencies := make([]int, 0, len(records))
	successCount := 0
	wrongFileCount := 0

	for _, rec := range records {
		tokens = append(tokens, rec.DiscoveryInputTokensRetrieval)
		latencies = append(latencies, rec.DiscoveryLatencyMsRetrieval)
		if rec.SuccessRetrieval {
			successCount++
		}
		if rec.WrongFileRetrieval {
			wrongFileCount++
		}
	}

	return Phase4RunMetrics{
		MedianDiscoveryInputTokens: MedianInt(tokens),
		MedianDiscoveryLatencyMs:   MedianInt(latencies),
		SuccessRate:                float64(successCount) / float64(len(records)),
		WrongFileRate:              float64(wrongFileCount) / float64(len(records)),
	}
}
