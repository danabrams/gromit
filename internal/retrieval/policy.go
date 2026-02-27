package retrieval

import "fmt"

// PolicyConfig configures how retrieval guidance is evaluated.
type PolicyConfig struct {
	ConfidenceThreshold    float64
	StalenessPolicy        StalenessPolicy
	StalenessThresholdDays int
	Detector               *StalenessDetector
}

// Policy enforces confidence and staleness guardrails for retrieval guidance.
type Policy struct {
	confidenceThreshold    float64
	stalenessPolicy        StalenessPolicy
	stalenessThresholdDays int
	detector               *StalenessDetector
}

// Guidance bundles retrieval snippets and index metadata for policy evaluation.
type Guidance struct {
	Snippets []Snippet
	Metadata IndexMetadata
}

// Decision describes how to act on retrieval guidance.
type Decision struct {
	UseRetrieval         bool
	VerificationRequired bool
	FallbackReason       string
}

// NewPolicy creates a Policy from the given configuration.
func NewPolicy(cfg PolicyConfig) *Policy {
	detector := cfg.Detector
	if detector == nil {
		detector = NewStalenessDetector(nil)
	}
	return &Policy{
		confidenceThreshold:    cfg.ConfidenceThreshold,
		stalenessPolicy:        cfg.StalenessPolicy,
		stalenessThresholdDays: cfg.StalenessThresholdDays,
		detector:               detector,
	}
}

// Evaluate returns a decision that indicates whether retrieval guidance is usable.
func (p *Policy) Evaluate(guidance Guidance) Decision {
	if p == nil {
		return Decision{FallbackReason: "retrieval policy unavailable"}
	}
	if len(guidance.Snippets) == 0 {
		return Decision{FallbackReason: "retrieval returned no snippets"}
	}

	if !guidance.Metadata.LastUpdated.IsZero() {
		if isStale, reason := p.detector.Check(guidance.Metadata, p.stalenessPolicy, p.stalenessThresholdDays); isStale {
			if reason == "" {
				reason = "index stale"
			}
			return Decision{FallbackReason: reason}
		}
	}

	maxConfidence := guidance.Snippets[0].ConfidenceScore
	for _, snippet := range guidance.Snippets[1:] {
		if snippet.ConfidenceScore > maxConfidence {
			maxConfidence = snippet.ConfidenceScore
		}
	}

	if maxConfidence < p.confidenceThreshold {
		return Decision{FallbackReason: fmt.Sprintf("confidence %.2f below threshold %.2f", maxConfidence, p.confidenceThreshold)}
	}

	return Decision{UseRetrieval: true, VerificationRequired: true}
}
