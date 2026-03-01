package visionmetrics

import "testing"

func TestVisionMetricsFixtureValidScenario(t *testing.T) {
    fixtures := loadFixtureRecords(t, "valid.fixture")
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
