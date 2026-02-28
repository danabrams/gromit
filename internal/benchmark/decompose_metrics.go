package benchmark

import "github.com/danabrams/gromit/internal/validate"

type DecomposeMetrics struct {
    BeadCount        int
    PerBeadViolations []validate.Violation
    BatchViolations  []validate.Violation
    Runtime          *RuntimeSignals
    Complexity        ComplexityMetrics
}

type RuntimeSignals struct {
    CostUSD  float64
    LatencyMs int
    TokenCount int
}

type ComplexityMetrics struct {
    HighCount  int
    Candidates []ComplexityCandidate
}

type ComplexityCandidate struct {
    Title   string
    Reasons []string
}

func ComputeDecomposeMetrics(candidates []validate.BeadCandidate, parentTitle string, maxSubBeads int, runtime *RuntimeSignals) DecomposeMetrics {
    metrics := DecomposeMetrics{
        BeadCount: len(candidates),
        Runtime:   runtime,
    }

    metrics.Complexity.Candidates = []ComplexityCandidate{}

    if len(candidates) == 0 {
        return metrics
    }

    validation := validate.ValidateDecomposeOutputWithMax(candidates, validate.DecomposeValidationModePipeline, parentTitle, maxSubBeads)
    metrics.PerBeadViolations = validation.Violations
    metrics.BatchViolations = validation.BatchViolations

    complexity := validate.ValidateDecomposeCandidates(candidates)
    metrics.Complexity.HighCount = complexity.ComplexityOutcome.HighCount
    metrics.Complexity.Candidates = make([]ComplexityCandidate, 0, len(complexity.ComplexityOutcome.HighComplexity))
    for _, candidate := range complexity.ComplexityOutcome.HighComplexity {
        metrics.Complexity.Candidates = append(metrics.Complexity.Candidates, ComplexityCandidate{
            Title:   candidate.Title,
            Reasons: append([]string(nil), candidate.Reasons...),
        })
    }

    return metrics
}
