package benchmark

import (
	"os"
	"path/filepath"
	stdstrings "strings"
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

func TestPhase4ReadPairedIterationRecords_ParsesJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "paired.jsonl")

	logContent := `{"discovery_input_tokens_baseline":100,"discovery_input_tokens_retrieval":70,"discovery_latency_ms_baseline":1000,"discovery_latency_ms_retrieval":800,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
{"discovery_input_tokens_baseline":90,"discovery_input_tokens_retrieval":65,"discovery_latency_ms_baseline":950,"discovery_latency_ms_retrieval":780,"success_baseline":true,"success_retrieval":false,"wrong_file_retrieval":true}
`

	if err := os.WriteFile(logPath, []byte(logContent), 0o644); err != nil {
		t.Fatalf("write test log: %v", err)
	}

	records, err := readPhase4PairedIterationRecords(logPath)
	if err != nil {
		t.Fatalf("readPhase4PairedIterationRecords failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}

	// Verify first record
	if records[0].DiscoveryInputTokensBaseline != 100 {
		t.Errorf("first record baseline tokens: expected 100, got %d", records[0].DiscoveryInputTokensBaseline)
	}
	if records[0].DiscoveryInputTokensRetrieval != 70 {
		t.Errorf("first record retrieval tokens: expected 70, got %d", records[0].DiscoveryInputTokensRetrieval)
	}
	if records[0].DiscoveryLatencyMsBaseline != 1000 {
		t.Errorf("first record baseline latency: expected 1000, got %d", records[0].DiscoveryLatencyMsBaseline)
	}
	if records[0].DiscoveryLatencyMsRetrieval != 800 {
		t.Errorf("first record retrieval latency: expected 800, got %d", records[0].DiscoveryLatencyMsRetrieval)
	}

	// Verify second record
	if records[1].WrongFileRetrieval != true {
		t.Errorf("second record wrong_file: expected true, got %v", records[1].WrongFileRetrieval)
	}
}

func TestPhase4SummarizeRun_ComputesMediansAndRates(t *testing.T) {
	records := []phase4PairedIterationRecord{
		{
			DiscoveryInputTokensBaseline:  100,
			DiscoveryInputTokensRetrieval: 70,
			DiscoveryLatencyMsBaseline:    1000,
			DiscoveryLatencyMsRetrieval:   800,
			SuccessBaseline:               true,
			SuccessRetrieval:              true,
			WrongFileRetrieval:            false,
		},
		{
			DiscoveryInputTokensBaseline:  90,
			DiscoveryInputTokensRetrieval: 65,
			DiscoveryLatencyMsBaseline:    950,
			DiscoveryLatencyMsRetrieval:   780,
			SuccessBaseline:               true,
			SuccessRetrieval:              false,
			WrongFileRetrieval:            true,
		},
		{
			DiscoveryInputTokensBaseline:  110,
			DiscoveryInputTokensRetrieval: 75,
			DiscoveryLatencyMsBaseline:    1050,
			DiscoveryLatencyMsRetrieval:   820,
			SuccessBaseline:               true,
			SuccessRetrieval:              true,
			WrongFileRetrieval:            false,
		},
	}

	baselineMetrics := summarizePhase4BaselineRun(records)
	retrievalMetrics := summarizePhase4RetrievalRun(records)

	// Verify baseline medians
	expectedBaselineTokens := 100
	if baselineMetrics.MedianDiscoveryInputTokens != expectedBaselineTokens {
		t.Errorf("baseline median tokens: expected %d, got %d", expectedBaselineTokens, baselineMetrics.MedianDiscoveryInputTokens)
	}

	expectedBaselineLatency := 1000
	if baselineMetrics.MedianDiscoveryLatencyMs != expectedBaselineLatency {
		t.Errorf("baseline median latency: expected %d, got %d", expectedBaselineLatency, baselineMetrics.MedianDiscoveryLatencyMs)
	}

	expectedBaselineSuccessRate := 1.0 // all 3 succeeded
	if baselineMetrics.SuccessRate != expectedBaselineSuccessRate {
		t.Errorf("baseline success rate: expected %.2f, got %.2f", expectedBaselineSuccessRate, baselineMetrics.SuccessRate)
	}

	// Verify retrieval medians
	expectedRetrievalTokens := 70
	if retrievalMetrics.MedianDiscoveryInputTokens != expectedRetrievalTokens {
		t.Errorf("retrieval median tokens: expected %d, got %d", expectedRetrievalTokens, retrievalMetrics.MedianDiscoveryInputTokens)
	}

	expectedRetrievalLatency := 800
	if retrievalMetrics.MedianDiscoveryLatencyMs != expectedRetrievalLatency {
		t.Errorf("retrieval median latency: expected %d, got %d", expectedRetrievalLatency, retrievalMetrics.MedianDiscoveryLatencyMs)
	}

	expectedRetrievalSuccessRate := 2.0 / 3.0 // 2 out of 3 succeeded
	if retrievalMetrics.SuccessRate != expectedRetrievalSuccessRate {
		t.Errorf("retrieval success rate: expected %.2f, got %.2f", expectedRetrievalSuccessRate, retrievalMetrics.SuccessRate)
	}

	expectedWrongFileRate := 1.0 / 3.0 // 1 out of 3 had wrong file
	if retrievalMetrics.WrongFileRate != expectedWrongFileRate {
		t.Errorf("retrieval wrong file rate: expected %.2f, got %.2f", expectedWrongFileRate, retrievalMetrics.WrongFileRate)
	}
}

func TestPhase4RunMeasurement_IntegratesPairedMetricsAndGates(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "paired.jsonl")

	logContent := `{"discovery_input_tokens_baseline":100,"discovery_input_tokens_retrieval":70,"discovery_latency_ms_baseline":1000,"discovery_latency_ms_retrieval":800,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
{"discovery_input_tokens_baseline":90,"discovery_input_tokens_retrieval":65,"discovery_latency_ms_baseline":950,"discovery_latency_ms_retrieval":780,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
{"discovery_input_tokens_baseline":110,"discovery_input_tokens_retrieval":75,"discovery_latency_ms_baseline":1050,"discovery_latency_ms_retrieval":820,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
`

	if err := os.WriteFile(logPath, []byte(logContent), 0o644); err != nil {
		t.Fatalf("write test log: %v", err)
	}

	report, err := RunPhase4Measurement(logPath)
	if err != nil {
		t.Fatalf("RunPhase4Measurement failed: %v", err)
	}

	// Verify baseline metrics
	if report.Baseline.MedianDiscoveryInputTokens != 100 {
		t.Errorf("baseline median tokens: expected 100, got %d", report.Baseline.MedianDiscoveryInputTokens)
	}

	// Verify retrieval metrics
	if report.Retrieval.MedianDiscoveryInputTokens != 70 {
		t.Errorf("retrieval median tokens: expected 70, got %d", report.Retrieval.MedianDiscoveryInputTokens)
	}

	// Verify gates - all should pass with 30% token reduction and 20% latency reduction
	if !report.Gates.TokenReductionGate {
		t.Errorf("TokenReductionGate should pass")
	}
	if !report.Gates.LatencyReductionGate {
		t.Errorf("LatencyReductionGate should pass")
	}
	if !report.Gates.SuccessRateParityGate {
		t.Errorf("SuccessRateParityGate should pass")
	}
	if !report.Gates.WrongFileRateGate {
		t.Errorf("WrongFileRateGate should pass")
	}
	if !report.Gates.CanAdopt {
		t.Errorf("CanAdopt should be true when all gates pass")
	}
}

func TestPhase4ComputeAdoptionDecision_IncludesReasonsForFailure(t *testing.T) {
	// Scenario 1: All gates pass
	passingGates := Phase4AdoptionGates{
		TokenReductionGate:    true,
		LatencyReductionGate:  true,
		SuccessRateParityGate: true,
		WrongFileRateGate:     true,
		CanAdopt:              true,
	}
	decision := ComputePhase4AdoptionDecision(passingGates)
	if !decision.ShouldAdopt {
		t.Errorf("decision should be adopt when all gates pass")
	}
	if len(decision.Reasons) > 0 {
		t.Errorf("decision should have no reasons when all gates pass, got %v", decision.Reasons)
	}

	// Scenario 2: Token reduction fails
	tokenFailGates := Phase4AdoptionGates{
		TokenReductionGate:    false,
		LatencyReductionGate:  true,
		SuccessRateParityGate: true,
		WrongFileRateGate:     true,
		CanAdopt:              false,
	}
	decision = ComputePhase4AdoptionDecision(tokenFailGates)
	if decision.ShouldAdopt {
		t.Errorf("decision should not adopt when token gate fails")
	}
	if len(decision.Reasons) == 0 {
		t.Errorf("decision should have reasons when gates fail")
	}
	foundTokenReason := false
	for _, reason := range decision.Reasons {
		if stdstrings.Contains(reason, "token") {
			foundTokenReason = true
		}
	}
	if !foundTokenReason {
		t.Errorf("decision reasons should mention token reduction, got %v", decision.Reasons)
	}

	// Scenario 3: Multiple gates fail
	multiFailGates := Phase4AdoptionGates{
		TokenReductionGate:    false,
		LatencyReductionGate:  false,
		SuccessRateParityGate: true,
		WrongFileRateGate:     false,
		CanAdopt:              false,
	}
	decision = ComputePhase4AdoptionDecision(multiFailGates)
	if decision.ShouldAdopt {
		t.Errorf("decision should not adopt when multiple gates fail")
	}
	if len(decision.Reasons) < 3 {
		t.Errorf("decision should have at least 3 reasons, got %d", len(decision.Reasons))
	}
}

func TestPhase4AdoptionGates_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		baseline       Phase4RunMetrics
		retrieval      Phase4RunMetrics
		expectedPass   bool
		gateToCheck    string
	}{
		{
			name: "token_reduction_exactly_20_percent_passes",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 80,  // exactly 20% reduction
				MedianDiscoveryLatencyMs:   900,
				SuccessRate:                0.94,
				WrongFileRate:              0.01,
			},
			expectedPass: true,
			gateToCheck:  "token",
		},
		{
			name: "token_reduction_below_20_percent_fails",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 81,  // 19% reduction
				MedianDiscoveryLatencyMs:   900,
				SuccessRate:                0.94,
				WrongFileRate:              0.01,
			},
			expectedPass: false,
			gateToCheck:  "token",
		},
		{
			name: "latency_reduction_exactly_15_percent_passes",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 70,
				MedianDiscoveryLatencyMs:   850,  // exactly 15% reduction
				SuccessRate:                0.94,
				WrongFileRate:              0.01,
			},
			expectedPass: true,
			gateToCheck:  "latency",
		},
		{
			name: "success_rate_drop_exactly_5_percent_passes",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 70,
				MedianDiscoveryLatencyMs:   800,
				SuccessRate:                0.90,  // exactly 5% drop
				WrongFileRate:              0.01,
			},
			expectedPass: true,
			gateToCheck:  "success",
		},
		{
			name: "wrong_file_rate_exactly_5_percent_passes",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 70,
				MedianDiscoveryLatencyMs:   800,
				SuccessRate:                0.94,
				WrongFileRate:              0.05,  // exactly 5% threshold
			},
			expectedPass: true,
			gateToCheck:  "wrong_file",
		},
		{
			name: "wrong_file_rate_above_5_percent_fails",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 70,
				MedianDiscoveryLatencyMs:   800,
				SuccessRate:                0.94,
				WrongFileRate:              0.051,  // above 5% threshold
			},
			expectedPass: false,
			gateToCheck:  "wrong_file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gates := EvaluatePhase4AdoptionGates(tt.baseline, tt.retrieval)

			var gateValue bool
			switch tt.gateToCheck {
			case "token":
				gateValue = gates.TokenReductionGate
			case "latency":
				gateValue = gates.LatencyReductionGate
			case "success":
				gateValue = gates.SuccessRateParityGate
			case "wrong_file":
				gateValue = gates.WrongFileRateGate
			}

			if gateValue != tt.expectedPass {
				t.Errorf("expected %v for %s gate, got %v", tt.expectedPass, tt.gateToCheck, gateValue)
			}
		})
	}
}
