package benchmark

import "github.com/danabrams/gromit/internal/validate"

type DecomposeMetrics struct {
    BeadCount        int
    PerBeadViolations []validate.Violation
    BatchViolations  []validate.Violation
    Runtime          *RuntimeSignals
}

type RuntimeSignals struct {
    CostUSD  float64
    LatencyMs int
    TokenCount int
}

func ComputeDecomposeMetrics(candidates []validate.BeadCandidate, parentTitle string, maxSubBeads int, runtime *RuntimeSignals) DecomposeMetrics {
    metrics := DecomposeMetrics{
        BeadCount: len(candidates),
        Runtime:   runtime,
    }

    if len(candidates) == 0 {
        return metrics
    }

    validation := validate.ValidateDecomposeOutputWithMax(candidates, validate.DecomposeValidationModePipeline, parentTitle, maxSubBeads)
    metrics.PerBeadViolations = validation.Violations
    metrics.BatchViolations = validation.BatchViolations

    return metrics
}
