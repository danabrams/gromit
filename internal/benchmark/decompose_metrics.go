package benchmark

import "github.com/danabrams/gromit/internal/validate"

type DecomposeMetrics struct {
    BeadCount        int
    PerBeadViolations []validate.Violation
    BatchViolations  []validate.Violation
    Runtime          *RuntimeSignals
    Complexity        ComplexityMetrics
    Acceptance        AcceptanceMetrics
    SiblingOverlapHits int
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

type AcceptanceMetrics struct {
    Total       int
    MeanPerBead float64
}

func ComputeDecomposeMetrics(candidates []validate.BeadCandidate, parentTitle string, maxSubBeads int, runtime *RuntimeSignals) DecomposeMetrics {
    metrics := DecomposeMetrics{
        BeadCount: len(candidates),
        Runtime:   runtime,
        Complexity: ComplexityMetrics{
            Candidates: []ComplexityCandidate{},
        },
        Acceptance: AcceptanceMetrics{},
    }

    if len(candidates) == 0 {
        return metrics
    }

    validation := validate.ValidateDecomposeOutputWithMax(candidates, validate.DecomposeValidationModePipeline, parentTitle, maxSubBeads)
    metrics.PerBeadViolations = validation.Violations
    metrics.BatchViolations = validation.BatchViolations

    metrics.SiblingOverlapHits = countSiblingOverlap(validation.Violations)

    criteriaTotal := countAcceptanceCriteria(candidates)
    metrics.Acceptance.Total = criteriaTotal
    if len(candidates) > 0 {
        metrics.Acceptance.MeanPerBead = float64(criteriaTotal) / float64(len(candidates))
    }

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

func countSiblingOverlap(violations []validate.Violation) int {
    hits := 0
    for _, violation := range violations {
        if violation.Rule == "sibling_overlap" {
            hits++
        }
    }
    return hits
}

func countAcceptanceCriteria(candidates []validate.BeadCandidate) int {
    total := 0
    for _, candidate := range candidates {
        total += len(candidate.AcceptanceCriteria)
    }
    return total
}
