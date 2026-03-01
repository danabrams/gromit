package prepare

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
)

// RED: test for NewSpecSPCBlocker creating a valid blocker
func TestNewSpecSPCBlocker_CreatesBlocker(t *testing.T) {
	t.Parallel()

	records := []logger.CauseClassificationRecord{}
	blocker := NewSpecSPCBlocker(records)

	if blocker == nil {
		t.Fatalf("NewSpecSPCBlocker() = nil, want valid blocker")
	}
}

// RED: test for ShouldBlock method existing on SpecSPCBlocker
func TestSpecSPCBlocker_ShouldBlock_MethodExists(t *testing.T) {
	t.Parallel()

	records := []logger.CauseClassificationRecord{}
	blocker := NewSpecSPCBlocker(records)

	b := &bead.Bead{
		ID:    "test-1",
		Title: "test bead",
	}

	// This test just verifies the method exists and compiles
	blocked, reason, err := blocker.ShouldBlock(context.Background(), b)
	if err != nil {
		t.Fatalf("ShouldBlock() error = %v, want nil", err)
	}

	// With empty records, should not block
	if blocked {
		t.Fatalf("blocked = true, want false for bead without spec label")
	}
	if reason != "" {
		t.Fatalf("reason = %q, want empty string", reason)
	}
}

// RED: test for ShouldBlock not blocking when bead has no spec label
func TestSpecSPCBlocker_ShouldBlock_NoSpecLabel(t *testing.T) {
	t.Parallel()

	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:auth",
			Class:              logger.CauseClassSpecial,
			Latest:             1500,
			PersistenceWindows: 3,
			Severity:           "high",
		},
	}
	blocker := NewSpecSPCBlocker(records)

	b := &bead.Bead{
		ID:     "test-no-spec",
		Title:  "test bead",
		Labels: []string{"priority:p1"},
	}

	blocked, reason, err := blocker.ShouldBlock(context.Background(), b)
	if err != nil {
		t.Fatalf("ShouldBlock() error = %v, want nil", err)
	}

	if blocked {
		t.Fatalf("blocked = true, want false for bead without spec label")
	}
	if reason != "" {
		t.Fatalf("reason = %q, want empty string", reason)
	}
}

// RED: test for ShouldBlock blocking when spec label matches special cause anomaly
func TestSpecSPCBlocker_ShouldBlock_BlocksOnSpecialCauseAnomaly(t *testing.T) {
	t.Parallel()

	spec := "auth"
	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:" + spec,
			Class:              logger.CauseClassSpecial,
			Latest:             1500,
			PersistenceWindows: 3,
			Severity:           "high",
		},
	}
	blocker := NewSpecSPCBlocker(records)

	b := &bead.Bead{
		ID:     "test-spec-block",
		Title:  "test bead",
		Labels: []string{"spec:" + spec},
	}

	blocked, reason, err := blocker.ShouldBlock(context.Background(), b)
	if err != nil {
		t.Fatalf("ShouldBlock() error = %v, want nil", err)
	}

	if !blocked {
		t.Fatalf("blocked = false, want true for spec with special cause anomaly")
	}
	if reason == "" {
		t.Fatalf("reason = empty string, want non-empty reason")
	}
}

// RED: test for ShouldBlock not blocking when spec has stable cause classification
func TestSpecSPCBlocker_ShouldBlock_StableDoesNotBlock(t *testing.T) {
	t.Parallel()

	spec := "auth"
	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:" + spec,
			Class:              logger.CauseClassStable,
			Latest:             1500,
			PersistenceWindows: 3,
		},
	}
	blocker := NewSpecSPCBlocker(records)

	b := &bead.Bead{
		ID:     "test-spec-stable",
		Title:  "test bead",
		Labels: []string{"spec:" + spec},
	}

	blocked, reason, err := blocker.ShouldBlock(context.Background(), b)
	if err != nil {
		t.Fatalf("ShouldBlock() error = %v, want nil", err)
	}

	if blocked {
		t.Fatalf("blocked = true, want false for stable cause classification")
	}
	if reason != "" {
		t.Fatalf("reason = %q, want empty string", reason)
	}
}

// RED: test for ShouldBlock not blocking when spec has common cause with no high severity
func TestSpecSPCBlocker_ShouldBlock_CommonCauseDoesNotBlock(t *testing.T) {
	t.Parallel()

	spec := "auth"
	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:" + spec,
			Class:              logger.CauseClassCommon,
			Latest:             1500,
			PersistenceWindows: 3,
			Severity:           "low",
		},
	}
	blocker := NewSpecSPCBlocker(records)

	b := &bead.Bead{
		ID:     "test-spec-common",
		Title:  "test bead",
		Labels: []string{"spec:" + spec},
	}

	blocked, reason, err := blocker.ShouldBlock(context.Background(), b)
	if err != nil {
		t.Fatalf("ShouldBlock() error = %v, want nil", err)
	}

	if blocked {
		t.Fatalf("blocked = true, want false for common cause classification")
	}
	if reason != "" {
		t.Fatalf("reason = %q, want empty string", reason)
	}
}

// RED: test for reason message format
func TestSpecSPCBlocker_ShouldBlock_ReasonMessageFormat(t *testing.T) {
	t.Parallel()

	spec := "payments"
	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:" + spec,
			Class:              logger.CauseClassSpecial,
			Latest:             2000,
			PersistenceWindows: 5,
			Severity:           "high",
		},
	}
	blocker := NewSpecSPCBlocker(records)

	b := &bead.Bead{
		ID:     "test-reason-format",
		Title:  "test bead",
		Labels: []string{"spec:" + spec},
	}

	blocked, reason, err := blocker.ShouldBlock(context.Background(), b)
	if err != nil {
		t.Fatalf("ShouldBlock() error = %v, want nil", err)
	}

	if !blocked {
		t.Fatalf("blocked = false, want true")
	}

	expectedReasonPrefix := "spec:" + spec
	if !containsSubstring(reason, expectedReasonPrefix) {
		t.Errorf("reason = %q, want to contain %q", reason, expectedReasonPrefix)
	}
}

// RED: test for spec label is case sensitive match
func TestSpecSPCBlocker_ShouldBlock_SpecLabelCaseMismatch(t *testing.T) {
	t.Parallel()

	spec := "auth"
	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:" + spec,
			Class:              logger.CauseClassSpecial,
			Latest:             1500,
			PersistenceWindows: 3,
			Severity:           "high",
		},
	}
	blocker := NewSpecSPCBlocker(records)

	b := &bead.Bead{
		ID:     "test-case-mismatch",
		Title:  "test bead",
		Labels: []string{"spec:Auth"}, // Different case
	}

	blocked, _, err := blocker.ShouldBlock(context.Background(), b)
	if err != nil {
		t.Fatalf("ShouldBlock() error = %v, want nil", err)
	}

	// Should not block due to case mismatch
	if blocked {
		t.Fatalf("blocked = true, want false for case mismatch")
	}
}

// RED: test for multiple special cause anomalies with only one matching spec
func TestSpecSPCBlocker_ShouldBlock_MultipleRecords(t *testing.T) {
	t.Parallel()

	records := []logger.CauseClassificationRecord{
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:auth",
			Class:              logger.CauseClassSpecial,
			Latest:             1500,
			PersistenceWindows: 3,
			Severity:           "high",
		},
		{
			Metric:             "rolling_avg_validation_ms",
			Stratum:            "spec:payments",
			Class:              logger.CauseClassStable,
			Latest:             1200,
			PersistenceWindows: 1,
			Severity:           "",
		},
		{
			Metric:             "rolling_avg_duration_ms",
			Stratum:            "spec:auth",
			Class:              logger.CauseClassCommon,
			Latest:             2000,
			PersistenceWindows: 2,
			Severity:           "low",
		},
	}
	blocker := NewSpecSPCBlocker(records)

	b := &bead.Bead{
		ID:     "test-multi-records",
		Title:  "test bead",
		Labels: []string{"spec:auth"},
	}

	blocked, reason, err := blocker.ShouldBlock(context.Background(), b)
	if err != nil {
		t.Fatalf("ShouldBlock() error = %v, want nil", err)
	}

	if !blocked {
		t.Fatalf("blocked = false, want true when matching spec has special cause with high severity")
	}
	if reason == "" {
		t.Fatalf("reason = empty string, want non-empty reason")
	}
}

// Helper function for substring checking
func containsSubstring(str, substr string) bool {
	if str == "" || substr == "" {
		return false
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
