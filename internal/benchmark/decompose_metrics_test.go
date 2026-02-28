package benchmark

import (
    "testing"

    "github.com/danabrams/gromit/internal/validate"
)

func TestComputeDecomposeMetrics_Validation(t *testing.T) {
    candidates := []validate.BeadCandidate{
        {Title: "Task One", AcceptanceCriteria: []string{"done"}},
        {Title: "Task Two", AcceptanceCriteria: []string{"done"}},
    }

    metrics := ComputeDecomposeMetrics(candidates, "", 1, nil)

    if metrics.BeadCount != len(candidates) {
        t.Fatalf("BeadCount = %d, want %d", metrics.BeadCount, len(candidates))
    }

    if len(metrics.BatchViolations) != 1 {
        t.Fatalf("BatchViolations count = %d, want 1", len(metrics.BatchViolations))
    }
    if metrics.BatchViolations[0].Rule != "batch_size_max" {
        t.Fatalf("BatchViolations[0].Rule = %q, want batch_size_max", metrics.BatchViolations[0].Rule)
    }

    outputMissing := 0
    for _, v := range metrics.PerBeadViolations {
        if v.Rule == "output_missing" {
            outputMissing++
        }
    }
    if outputMissing < len(candidates) {
        t.Fatalf("expected at least %d output_missing violations, got %d", len(candidates), outputMissing)
    }
}

func TestComputeDecomposeMetrics_Complexity(t *testing.T) {
    candidates := []validate.BeadCandidate{
        {
            Title:          "Large Task",
            EstimatedFiles: 8,
            AcceptanceCriteria: []string{"done"},
            ExpectedOutputs:  []string{"result"},
        },
    }

    metrics := ComputeDecomposeMetrics(candidates, "", 5, nil)

    if metrics.Complexity.HighCount != 1 {
        t.Fatalf("Complexity.HighCount = %d, want 1", metrics.Complexity.HighCount)
    }

    if len(metrics.Complexity.Candidates) != 1 {
        t.Fatalf("Complexity.Candidates count = %d, want 1", len(metrics.Complexity.Candidates))
    }

    candidate := metrics.Complexity.Candidates[0]
    if candidate.Title != "Large Task" {
        t.Fatalf("Complexity candidate title = %q, want Large Task", candidate.Title)
    }
    if len(candidate.Reasons) == 0 {
        t.Fatalf("expected reasons for large task, got none")
    }
    if candidate.Reasons[0] != "estimated_files=8 crosses the high-complexity threshold" {
        t.Fatalf("unexpected complexity reason = %q", candidate.Reasons[0])
    }
}

func TestComputeDecomposeMetrics_CriteriaAndOverlap(t *testing.T) {
    overlap := "Ensure the telemetry pipeline records all errors quickly."
    candidates := []validate.BeadCandidate{
        {
            Title:              "Task A",
            AcceptanceCriteria: []string{overlap, "Perform cleanup"},
            ExpectedOutputs:    []string{"result-a"},
        },
        {
            Title:              "Task B",
            AcceptanceCriteria: []string{overlap + " even when scaling"},
            ExpectedOutputs:    []string{"result-b"},
        },
    }

    metrics := ComputeDecomposeMetrics(candidates, "", 5, nil)

    expectedTotal := 3
    if metrics.Acceptance.Total != expectedTotal {
        t.Fatalf("Acceptance.Total = %d, want %d", metrics.Acceptance.Total, expectedTotal)
    }

    expectedMean := 1.5
    if metrics.Acceptance.MeanPerBead != expectedMean {
        t.Fatalf("Acceptance.MeanPerBead = %f, want %f", metrics.Acceptance.MeanPerBead, expectedMean)
    }

    if metrics.SiblingOverlapHits == 0 {
        t.Fatalf("expected sibling overlap violations, got %d", metrics.SiblingOverlapHits)
    }
}

func TestComputeDecomposeMetrics_RuntimeSignals(t *testing.T) {
    candidates := []validate.BeadCandidate{
        {
            Title:              "Reliable Task",
            AcceptanceCriteria: []string{"done"},
            ExpectedOutputs:    []string{"result"},
        },
    }

    withoutSignals := ComputeDecomposeMetrics(candidates, "", 5, nil)
    if withoutSignals.Runtime != nil {
        t.Fatalf("expected nil runtime signals, got %+v", withoutSignals.Runtime)
    }

    signals := &RuntimeSignals{
        CostUSD:  1.23,
        LatencyMs: 240,
        TokenCount: 1024,
    }
    withSignals := ComputeDecomposeMetrics(candidates, "", 5, signals)
    if withSignals.Runtime == nil {
        t.Fatalf("expected runtime signals, got nil")
    }
    if withSignals.Runtime.CostUSD != signals.CostUSD ||
        withSignals.Runtime.LatencyMs != signals.LatencyMs ||
        withSignals.Runtime.TokenCount != signals.TokenCount {
        t.Fatalf("runtime signals mismatch: %+v", withSignals.Runtime)
    }
}
