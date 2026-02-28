package visionmetrics

import (
    "bufio"
    "encoding/json"
    "math"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestVisionMetricsAcceptanceRollup(t *testing.T) {
    t.Helper()

    fixturePath := filepath.Join("testdata", "vision_records.jsonl")
    rawRecords := loadFixtureRecords(t, fixturePath)
    if len(rawRecords) != 6 {
        t.Fatalf("fixture should load 6 records, got %d", len(rawRecords))
    }

    var valid, invalid []Record
    for _, rec := range rawRecords {
        if errs := Validate(rec); len(errs) > 0 {
            invalid = append(invalid, rec)
            continue
        }
        valid = append(valid, rec)
    }

    if len(valid) != 5 {
        t.Fatalf("expected 5 valid records, got %d", len(valid))
    }
    if len(invalid) == 0 {
        t.Fatalf("expected at least one invalid record in fixture")
    }

    rollup := ComputeRollup(valid)

    expectRate(t, rollup.HumanTacticalInterventionRate, 3, len(valid), 0.6)
    expectRate(t, rollup.HumanDebuggingInterventionRate, 2, len(valid), 0.4)
    expectRate(t, rollup.FirstIntegrationPassRate, 3, len(valid), 0.6)
    expectRate(t, rollup.EscapedRegressionRate, 1, len(valid), 0.2)

    carveOuts := countCarveOut(valid)
    expectRate(t, rollup.AcceptedWithoutReworkRate, 3, len(valid)-carveOuts, 0.75)

    if countCarveOut(rawRecords) == 0 {
        t.Fatal("fixture should expose a carve-out record so auditors can trace it")
    }

    if rollup.AcceptedWithoutReworkRate.Denominator != len(valid)-carveOuts {
        t.Fatalf("accepted-without-rework denominator should exclude carve-outs: want %d, got %d", len(valid)-carveOuts, rollup.AcceptedWithoutReworkRate.Denominator)
    }
}

func loadFixtureRecords(t *testing.T, path string) []Record {
    t.Helper()

    f, err := os.Open(path)
    if err != nil {
        t.Fatalf("open fixture %s: %v", path, err)
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    var records []Record
    line := 0
    for scanner.Scan() {
        line++
        text := strings.TrimSpace(scanner.Text())
        if text == "" {
            continue
        }
        var rec Record
        if err := json.Unmarshal([]byte(text), &rec); err != nil {
            t.Fatalf("decode line %d: %v", line, err)
        }
        records = append(records, rec)
    }
    if err := scanner.Err(); err != nil {
        t.Fatalf("scan fixture %s: %v", path, err)
    }

    return records
}

func expectRate(t *testing.T, got MetricRate, wantNum, wantDen int, wantRate float64) {
    t.Helper()
    if got.Numerator != wantNum {
        t.Fatalf("unexpected numerator %d, want %d", got.Numerator, wantNum)
    }
    if got.Denominator != wantDen {
        t.Fatalf("unexpected denominator %d, want %d", got.Denominator, wantDen)
    }
    if math.Abs(got.Rate-wantRate) > 1e-9 {
        t.Fatalf("unexpected rate %.3f, want %.3f", got.Rate, wantRate)
    }
}

func countCarveOut(records []Record) int {
    var count int
    for _, rec := range records {
        if rec.ReviewOutcome == ReviewOutcomeVisionChange {
            count++
        }
    }
    return count
}
