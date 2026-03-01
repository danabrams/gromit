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
