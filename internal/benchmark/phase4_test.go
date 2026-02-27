package benchmark

import (
	"os"
	"path/filepath"
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
