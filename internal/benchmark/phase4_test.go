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
		MedianDiscoveryInputTokens: 70,   // 30% reduction
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
		name         string
		baseline     Phase4RunMetrics
		retrieval    Phase4RunMetrics
		expectedPass bool
		gateToCheck  string
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
				MedianDiscoveryInputTokens: 80, // exactly 20% reduction
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
				MedianDiscoveryInputTokens: 81, // 19% reduction
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
				MedianDiscoveryLatencyMs:   850, // exactly 15% reduction
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
				SuccessRate:                0.90, // exactly 5% drop
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
				WrongFileRate:              0.05, // exactly 5% threshold
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
				WrongFileRate:              0.051, // above 5% threshold
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

func TestPhase4FullEvaluation_NoAdoptWhenGatesFail(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "paired.jsonl")

	// Create a scenario where retrieval performs worse than baseline
	logContent := `{"discovery_input_tokens_baseline":100,"discovery_input_tokens_retrieval":95,"discovery_latency_ms_baseline":1000,"discovery_latency_ms_retrieval":980,"success_baseline":true,"success_retrieval":false,"wrong_file_retrieval":true}
{"discovery_input_tokens_baseline":90,"discovery_input_tokens_retrieval":88,"discovery_latency_ms_baseline":950,"discovery_latency_ms_retrieval":940,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":true}
{"discovery_input_tokens_baseline":110,"discovery_input_tokens_retrieval":105,"discovery_latency_ms_baseline":1050,"discovery_latency_ms_retrieval":1030,"success_baseline":true,"success_retrieval":false,"wrong_file_retrieval":true}
`

	if err := os.WriteFile(logPath, []byte(logContent), 0o644); err != nil {
		t.Fatalf("write test log: %v", err)
	}

	report, err := RunPhase4Measurement(logPath)
	if err != nil {
		t.Fatalf("RunPhase4Measurement failed: %v", err)
	}

	// Verify gates fail (minimal improvement doesn't meet thresholds)
	if report.Gates.CanAdopt {
		t.Errorf("CanAdopt should be false when gates fail")
	}

	// Verify specific gate failures
	if report.Gates.TokenReductionGate {
		t.Errorf("TokenReductionGate should fail (only 3%% reduction)")
	}
	if report.Gates.WrongFileRateGate {
		t.Errorf("WrongFileRateGate should fail (67%% wrong-file rate)")
	}
	if report.Gates.SuccessRateParityGate {
		t.Errorf("SuccessRateParityGate should fail (33%% success drop)")
	}

	// Verify adoption decision reflects failures
	decision := ComputePhase4AdoptionDecision(report.Gates)
	if decision.ShouldAdopt {
		t.Errorf("adoption decision should be no-adopt")
	}
	if len(decision.Reasons) < 3 {
		t.Errorf("should have at least 3 failure reasons, got %d", len(decision.Reasons))
	}

	// Verify all failure reasons are documented
	reasonsStr := stdstrings.Join(decision.Reasons, ",")
	requiredReasons := []string{"token", "wrong_file", "success_rate"}
	for _, required := range requiredReasons {
		if !stdstrings.Contains(reasonsStr, required) {
			t.Errorf("expected reason containing '%s' in %v", required, decision.Reasons)
		}
	}
}

func TestPhase4ReadPairedIterationRecords_HandlesEmptyAndMalformedFiles(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		expectCount int
	}{
		{
			name:        "empty file",
			content:     "",
			expectError: false,
			expectCount: 0,
		},
		{
			name:        "file with only whitespace",
			content:     "\n\n  \n",
			expectError: false,
			expectCount: 0,
		},
		{
			name: "file with valid and empty lines",
			content: `{"discovery_input_tokens_baseline":100,"discovery_input_tokens_retrieval":70,"discovery_latency_ms_baseline":1000,"discovery_latency_ms_retrieval":800,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}

{"discovery_input_tokens_baseline":90,"discovery_input_tokens_retrieval":65,"discovery_latency_ms_baseline":950,"discovery_latency_ms_retrieval":780,"success_baseline":true,"success_retrieval":false,"wrong_file_retrieval":true}
`,
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "file with truly invalid JSON",
			content:     `{invalid json}`,
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "test.jsonl")
			if err := os.WriteFile(logPath, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("write test log: %v", err)
			}

			records, err := readPhase4PairedIterationRecords(logPath)

			if tt.expectError && err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for %s: %v", tt.name, err)
			}

			if len(records) != tt.expectCount {
				t.Errorf("expected %d records for %s, got %d", tt.expectCount, tt.name, len(records))
			}
		})
	}
}

func TestPhase4DisabledRetrievalMatchesBaseline_ProducesNoAdoptDecision(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "disabled_retrieval.jsonl")

	// Scenario: Retrieval is disabled, so retrieval metrics exactly match baseline
	logContent := `{"discovery_input_tokens_baseline":100,"discovery_input_tokens_retrieval":100,"discovery_latency_ms_baseline":1000,"discovery_latency_ms_retrieval":1000,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
{"discovery_input_tokens_baseline":90,"discovery_input_tokens_retrieval":90,"discovery_latency_ms_baseline":950,"discovery_latency_ms_retrieval":950,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
{"discovery_input_tokens_baseline":110,"discovery_input_tokens_retrieval":110,"discovery_latency_ms_baseline":1050,"discovery_latency_ms_retrieval":1050,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
`

	if err := os.WriteFile(logPath, []byte(logContent), 0o644); err != nil {
		t.Fatalf("write test log: %v", err)
	}

	report, err := RunPhase4Measurement(logPath)
	if err != nil {
		t.Fatalf("RunPhase4Measurement failed: %v", err)
	}

	// Verify retrieval metrics match baseline exactly
	if report.Baseline.MedianDiscoveryInputTokens != report.Retrieval.MedianDiscoveryInputTokens {
		t.Errorf("disabled retrieval tokens should match baseline: baseline %d vs retrieval %d",
			report.Baseline.MedianDiscoveryInputTokens, report.Retrieval.MedianDiscoveryInputTokens)
	}
	if report.Baseline.MedianDiscoveryLatencyMs != report.Retrieval.MedianDiscoveryLatencyMs {
		t.Errorf("disabled retrieval latency should match baseline: baseline %d vs retrieval %d",
			report.Baseline.MedianDiscoveryLatencyMs, report.Retrieval.MedianDiscoveryLatencyMs)
	}

	// When disabled (0% reduction), all gates should fail
	if report.Gates.TokenReductionGate {
		t.Errorf("TokenReductionGate should fail when retrieval is disabled (0%% reduction)")
	}
	if report.Gates.LatencyReductionGate {
		t.Errorf("LatencyReductionGate should fail when retrieval is disabled (0%% reduction)")
	}

	// Verify no-adopt decision
	decision := ComputePhase4AdoptionDecision(report.Gates)
	if decision.ShouldAdopt {
		t.Errorf("decision should be no-adopt when retrieval is disabled")
	}
	if len(decision.Reasons) < 2 {
		t.Errorf("should have at least 2 failure reasons (token and latency), got %d", len(decision.Reasons))
	}
}

func TestPhase4SingleGateFailure_EachGateFailureProducesNoAdopt(t *testing.T) {
	tests := []struct {
		name             string
		baseline         Phase4RunMetrics
		retrieval        Phase4RunMetrics
		failingGate      string
		expectedNotAdopt bool
	}{
		{
			name: "token_reduction_gate_failure_alone",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 81,   // 19% reduction (below 20% threshold)
				MedianDiscoveryLatencyMs:   800,  // 20% reduction (passes)
				SuccessRate:                0.94, // 1% drop (passes)
				WrongFileRate:              0.01, // passes
			},
			failingGate:      "token_reduction",
			expectedNotAdopt: true,
		},
		{
			name: "latency_reduction_gate_failure_alone",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 70,   // 30% reduction (passes)
				MedianDiscoveryLatencyMs:   860,  // 14% reduction (below 15% threshold)
				SuccessRate:                0.94, // 1% drop (passes)
				WrongFileRate:              0.01, // passes
			},
			failingGate:      "latency_reduction",
			expectedNotAdopt: true,
		},
		{
			name: "success_rate_parity_gate_failure_alone",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 70,   // 30% reduction (passes)
				MedianDiscoveryLatencyMs:   800,  // 20% reduction (passes)
				SuccessRate:                0.89, // 6% drop (above 5% threshold)
				WrongFileRate:              0.01, // passes
			},
			failingGate:      "success_rate_parity",
			expectedNotAdopt: true,
		},
		{
			name: "wrong_file_rate_gate_failure_alone",
			baseline: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 100,
				MedianDiscoveryLatencyMs:   1000,
				SuccessRate:                0.95,
				WrongFileRate:              0.01,
			},
			retrieval: Phase4RunMetrics{
				MedianDiscoveryInputTokens: 70,    // 30% reduction (passes)
				MedianDiscoveryLatencyMs:   800,   // 20% reduction (passes)
				SuccessRate:                0.94,  // 1% drop (passes)
				WrongFileRate:              0.051, // above 5% threshold
			},
			failingGate:      "wrong_file_rate",
			expectedNotAdopt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gates := EvaluatePhase4AdoptionGates(tt.baseline, tt.retrieval)
			decision := ComputePhase4AdoptionDecision(gates)

			if !tt.expectedNotAdopt && decision.ShouldAdopt {
				t.Fatalf("expected no-adopt for %s, but got adopt", tt.failingGate)
			}
			if tt.expectedNotAdopt && decision.ShouldAdopt {
				t.Fatalf("single gate failure (%s) should produce no-adopt", tt.failingGate)
			}

			// Verify the failing gate is documented in reasons
			reasonsStr := stdstrings.Join(decision.Reasons, ",")
			failureFound := false
			switch tt.failingGate {
			case "token_reduction":
				failureFound = stdstrings.Contains(reasonsStr, "token")
			case "latency_reduction":
				failureFound = stdstrings.Contains(reasonsStr, "latency")
			case "success_rate_parity":
				failureFound = stdstrings.Contains(reasonsStr, "success_rate")
			case "wrong_file_rate":
				failureFound = stdstrings.Contains(reasonsStr, "wrong_file")
			}

			if !failureFound {
				t.Errorf("expected %s failure in reasons, got %v", tt.failingGate, decision.Reasons)
			}
		})
	}
}

func TestPhase4AllGatesPass_ProducesAdoptWithFullEvidence(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "all_passing.jsonl")

	// Scenario: All gates pass with strong margins
	logContent := `{"discovery_input_tokens_baseline":100,"discovery_input_tokens_retrieval":70,"discovery_latency_ms_baseline":1000,"discovery_latency_ms_retrieval":800,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
{"discovery_input_tokens_baseline":100,"discovery_input_tokens_retrieval":70,"discovery_latency_ms_baseline":1000,"discovery_latency_ms_retrieval":800,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
{"discovery_input_tokens_baseline":100,"discovery_input_tokens_retrieval":70,"discovery_latency_ms_baseline":1000,"discovery_latency_ms_retrieval":800,"success_baseline":true,"success_retrieval":true,"wrong_file_retrieval":false}
`

	if err := os.WriteFile(logPath, []byte(logContent), 0o644); err != nil {
		t.Fatalf("write test log: %v", err)
	}

	report, err := RunPhase4Measurement(logPath)
	if err != nil {
		t.Fatalf("RunPhase4Measurement failed: %v", err)
	}

	// Verify all gates pass
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

	// Verify adoption decision with full evidence
	decision := ComputePhase4AdoptionDecision(report.Gates)
	if !decision.ShouldAdopt {
		t.Errorf("decision should be adopt when all gates pass")
	}
	if len(decision.Reasons) != 0 {
		t.Errorf("adoption with all gates passing should have no reasons, got %v", decision.Reasons)
	}

	// Verify full evidence is captured in the report
	if report.Baseline.MedianDiscoveryInputTokens == 0 {
		t.Errorf("baseline token evidence missing")
	}
	if report.Retrieval.MedianDiscoveryInputTokens == 0 {
		t.Errorf("retrieval token evidence missing")
	}
	if report.Baseline.MedianDiscoveryLatencyMs == 0 {
		t.Errorf("baseline latency evidence missing")
	}
	if report.Retrieval.MedianDiscoveryLatencyMs == 0 {
		t.Errorf("retrieval latency evidence missing")
	}
	if report.Baseline.SuccessRate == 0 {
		t.Errorf("baseline success rate evidence missing")
	}
	if report.Retrieval.SuccessRate == 0 {
		t.Errorf("retrieval success rate evidence missing")
	}
	if report.Retrieval.WrongFileRate < 0 {
		t.Errorf("wrong file rate evidence missing")
	}

	// Verify metrics show actual improvements
	if report.Baseline.MedianDiscoveryInputTokens <= report.Retrieval.MedianDiscoveryInputTokens {
		t.Errorf("retrieval should have fewer tokens: baseline %d vs retrieval %d",
			report.Baseline.MedianDiscoveryInputTokens, report.Retrieval.MedianDiscoveryInputTokens)
	}
	if report.Baseline.MedianDiscoveryLatencyMs <= report.Retrieval.MedianDiscoveryLatencyMs {
		t.Errorf("retrieval should have lower latency: baseline %d vs retrieval %d",
			report.Baseline.MedianDiscoveryLatencyMs, report.Retrieval.MedianDiscoveryLatencyMs)
	}
}

func TestPhase4EvaluateAdoptionGates_ZeroBaselineTokens_FailsTokenGate(t *testing.T) {
	baseline := Phase4RunMetrics{
		MedianDiscoveryInputTokens: 0, // zero denominator
		MedianDiscoveryLatencyMs:   1000,
		SuccessRate:                0.95,
		WrongFileRate:              0.01,
	}
	retrieval := Phase4RunMetrics{
		MedianDiscoveryInputTokens: 70,
		MedianDiscoveryLatencyMs:   800,
		SuccessRate:                0.94,
		WrongFileRate:              0.01,
	}

	gates := EvaluatePhase4AdoptionGates(baseline, retrieval)

	if gates.TokenReductionGate {
		t.Errorf("TokenReductionGate should fail when baseline tokens are zero")
	}
}

func TestPhase4EvaluateAdoptionGates_ZeroBaselineLatency_FailsLatencyGate(t *testing.T) {
	baseline := Phase4RunMetrics{
		MedianDiscoveryInputTokens: 100,
		MedianDiscoveryLatencyMs:   0, // zero denominator
		SuccessRate:                0.95,
		WrongFileRate:              0.01,
	}
	retrieval := Phase4RunMetrics{
		MedianDiscoveryInputTokens: 70,
		MedianDiscoveryLatencyMs:   800,
		SuccessRate:                0.94,
		WrongFileRate:              0.01,
	}

	gates := EvaluatePhase4AdoptionGates(baseline, retrieval)

	if gates.LatencyReductionGate {
		t.Errorf("LatencyReductionGate should fail when baseline latency is zero")
	}
}
