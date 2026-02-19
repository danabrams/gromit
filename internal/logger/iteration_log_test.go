package logger

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestIterationLog_HasRateLimitRecoveryMsField verifies that IterationLog
// struct has a RateLimitRecoveryMs field for logging rate limit recovery time.
func TestIterationLog_HasRateLimitRecoveryMsField(t *testing.T) {
	log := &IterationLog{
		BeadID:              "test-1",
		Model:               "sonnet",
		RateLimitHits:       2,
		RateLimitRecoveryMs: 180,
	}

	if log.RateLimitRecoveryMs != 180 {
		t.Errorf("expected RateLimitRecoveryMs=180, got %d", log.RateLimitRecoveryMs)
	}
}

// TestIterationLog_ValidationDurationMsJSONTag verifies that ValidationDurationMs
// uses the expected json tag and omitempty behavior.
func TestIterationLog_ValidationDurationMsJSONTag(t *testing.T) {
	log := &IterationLog{
		BeadID:               "test-1",
		Model:                "sonnet",
		ValidationDurationMs: 1800,
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("marshal log: %v", err)
	}
	if !strings.Contains(string(data), "\"validation_duration_ms\":1800") {
		t.Fatalf("expected validation_duration_ms in JSON, got %s", string(data))
	}

	emptyData, err := json.Marshal(IterationLog{})
	if err != nil {
		t.Fatalf("marshal empty log: %v", err)
	}
	if strings.Contains(string(emptyData), "validation_duration_ms") {
		t.Fatalf("expected validation_duration_ms to be omitted, got %s", string(emptyData))
	}
}

// TestIterationLog_SpecIDJSONTag verifies that IterationLog has a SpecID field
// with json tag "spec_id" and omitempty behavior.
func TestIterationLog_SpecIDJSONTag(t *testing.T) {
	log := &IterationLog{
		BeadID: "bead-1",
		SpecID: "spec-xyz",
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("marshal log: %v", err)
	}
	if !strings.Contains(string(data), "\"spec_id\":\"spec-xyz\"") {
		t.Fatalf("expected spec_id in JSON, got %s", string(data))
	}

	emptyData, err := json.Marshal(IterationLog{})
	if err != nil {
		t.Fatalf("marshal empty log: %v", err)
	}
	if strings.Contains(string(emptyData), "spec_id") {
		t.Fatalf("expected spec_id to be omitted, got %s", string(emptyData))
	}
}

// TestIterationLog_FailurePhaseJSONTag verifies that IterationLog has a FailurePhase field
// with json tag "failure_phase" and omitempty behavior.
func TestIterationLog_FailurePhaseJSONTag(t *testing.T) {
	log := &IterationLog{
		BeadID:       "bead-1",
		Model:        "sonnet",
		FailurePhase: "validation",
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("marshal log: %v", err)
	}
	if !strings.Contains(string(data), "\"failure_phase\":\"validation\"") {
		t.Fatalf("expected failure_phase in JSON, got %s", string(data))
	}

	emptyData, err := json.Marshal(IterationLog{})
	if err != nil {
		t.Fatalf("marshal empty log: %v", err)
	}
	if strings.Contains(string(emptyData), "failure_phase") {
		t.Fatalf("expected failure_phase to be omitted, got %s", string(emptyData))
	}
}

func TestIterationLog_ProviderAndFailureCategoryJSONTags(t *testing.T) {
	log := &IterationLog{
		BeadID:          "test-1",
		Model:           "sonnet",
		Provider:        "codex",
		FailureCategory: "rate_limited",
	}

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("marshal log: %v", err)
	}
	if !strings.Contains(string(data), "\"provider\":\"codex\"") {
		t.Fatalf("expected provider in JSON, got %s", string(data))
	}
	if !strings.Contains(string(data), "\"failure_category\":\"rate_limited\"") {
		t.Fatalf("expected failure_category in JSON, got %s", string(data))
	}

	emptyData, err := json.Marshal(IterationLog{})
	if err != nil {
		t.Fatalf("marshal empty log: %v", err)
	}
	if strings.Contains(string(emptyData), "provider") {
		t.Fatalf("expected provider to be omitted, got %s", string(emptyData))
	}
	if strings.Contains(string(emptyData), "failure_category") {
		t.Fatalf("expected failure_category to be omitted, got %s", string(emptyData))
	}
}
