package visionmetrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVisionMetricsFixtureValidScenario(t *testing.T) {
	fixtures := loadIntegrationFixtureRecords(t, "valid.fixture")
	if len(fixtures) != 3 {
		t.Fatalf("expected 3 fixture records, got %d", len(fixtures))
	}

	var records []Record
	for i, entry := range fixtures {
		if len(entry.Errors) != 0 {
			t.Fatalf("record %d had validation errors: %v", i+1, entry.Errors)
		}
		records = append(records, entry.Record)
	}

	rollup := ComputeRollup(records)
	expectRate(t, rollup.HumanTacticalInterventionRate, 1, 3, 1.0/3)
	expectRate(t, rollup.HumanDebuggingInterventionRate, 1, 3, 1.0/3)
	expectRate(t, rollup.FirstIntegrationPassRate, 2, 3, 2.0/3)
	expectRate(t, rollup.EscapedRegressionRate, 0, 3, 0.0)
	if rollup.EscapedRegressionPendingCount != 0 {
		t.Fatalf("expected no pending escaped regressions, got %d", rollup.EscapedRegressionPendingCount)
	}
	expectRate(t, rollup.AcceptedWithoutReworkRate, 2, 2, 1.0)
}

func TestVisionMetricsFixtureInvalidScenario(t *testing.T) {
	fixtures := loadIntegrationFixtureRecords(t, "invalid.fixture")
	if len(fixtures) != 2 {
		t.Fatalf("expected 2 fixture records, got %d", len(fixtures))
	}

	var validRecords []Record
	var invalidRecords []integrationFixtureRecord
	for _, entry := range fixtures {
		if len(entry.Errors) == 0 {
			validRecords = append(validRecords, entry.Record)
			continue
		}
		invalidRecords = append(invalidRecords, entry)
	}

	if len(invalidRecords) != 1 {
		t.Fatalf("expected 1 invalid record, got %d", len(invalidRecords))
	}

	validationError := invalidRecords[0].Errors[0]
	if validationError.Field != FieldHumanDebuggingIntervention {
		t.Fatalf("expected debugging field err, got %s", validationError.Field)
	}
	if validationError.Reason != "requires tactical intervention" {
		t.Fatalf("expected debugging requirement error, got %s", validationError.Reason)
	}

	if len(validRecords) != 1 {
		t.Fatalf("expected 1 valid record, got %d", len(validRecords))
	}

	rollup := ComputeRollup(validRecords)
	expectRate(t, rollup.HumanTacticalInterventionRate, 0, 1, 0.0)
	expectRate(t, rollup.HumanDebuggingInterventionRate, 0, 1, 0.0)
	expectRate(t, rollup.FirstIntegrationPassRate, 1, 1, 1.0)
	expectRate(t, rollup.EscapedRegressionRate, 0, 1, 0.0)
	if rollup.EscapedRegressionPendingCount != 0 {
		t.Fatalf("expected no pending escapes, got %d", rollup.EscapedRegressionPendingCount)
	}
	expectRate(t, rollup.AcceptedWithoutReworkRate, 1, 1, 1.0)
}

func TestVisionMetricsFixturePendingResolutionScenario(t *testing.T) {
	fixtures := loadIntegrationFixtureRecords(t, "pending.fixture")
	if len(fixtures) != 2 {
		t.Fatalf("expected 2 fixture records, got %d", len(fixtures))
	}

	var records []Record
	for i, entry := range fixtures {
		if len(entry.Errors) != 0 {
			t.Fatalf("fixture record %d should validate, but got %d errors", i+1, len(entry.Errors))
		}
		records = append(records, entry.Record)
	}

	var pendingCount, escapedYesCount int
	for _, rec := range records {
		switch rec.EscapedRegressionWithin7D {
		case EscapedRegressionPending:
			pendingCount++
		case Yes:
			escapedYesCount++
		}
	}

	if pendingCount == 0 {
		t.Fatalf("expected at least one pending escaped regression")
	}

	resolvedCount := len(records) - pendingCount
	if resolvedCount == 0 {
		t.Fatalf("expected at least one resolved escaped regression record")
	}

	rollupPre := ComputeRollup(records)
	if rollupPre.EscapedRegressionPendingCount != pendingCount {
		t.Fatalf("expected %d pending count, got %d", pendingCount, rollupPre.EscapedRegressionPendingCount)
	}
	expectRate(t, rollupPre.EscapedRegressionRate, escapedYesCount, resolvedCount, float64(escapedYesCount)/float64(resolvedCount))

	resolvedRecords := append([]Record(nil), records...)
	for i := range resolvedRecords {
		if resolvedRecords[i].EscapedRegressionWithin7D == EscapedRegressionPending {
			resolvedRecords[i].EscapedRegressionWithin7D = No
			break
		}
	}

	rollupPost := ComputeRollup(resolvedRecords)
	if rollupPost.EscapedRegressionPendingCount != 0 {
		t.Fatalf("expected 0 pending count after resolution, got %d", rollupPost.EscapedRegressionPendingCount)
	}
	expectRate(t, rollupPost.EscapedRegressionRate, escapedYesCount, len(resolvedRecords), float64(escapedYesCount)/float64(len(resolvedRecords)))
}

type integrationFixtureRecord struct {
	Record Record
	Errors []ValidationError
}

func loadIntegrationFixtureRecords(t *testing.T, filename string) []integrationFixtureRecord {
	t.Helper()
	path := filepath.Join("testdata", "fixtures", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	sections := splitFixtureSections(string(data))
	var records []integrationFixtureRecord
	for i, section := range sections {
		rec, err := ParseFromPRBody(section)
		if err != nil {
			t.Fatalf("parse fixture record %d: %v", i+1, err)
		}
		errs := Validate(rec)
		records = append(records, integrationFixtureRecord{Record: rec, Errors: errs})
	}
	return records
}

func splitFixtureSections(input string) []string {
	parts := strings.Split(input, "\n---\n")
	var sections []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		sections = append(sections, trimmed)
	}
	return sections
}
