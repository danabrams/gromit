package retrieval

import (
    "strings"
    "testing"
    "time"
)

func TestPolicyAcceptsFreshHighConfidence(t *testing.T) {
    now := time.Date(2026, time.February, 27, 0, 0, 0, 0, time.UTC)
    cfg := PolicyConfig{
        ConfidenceThreshold:    0.8,
        StalenessPolicy:        StalenessPolicyMaxAge,
        StalenessThresholdDays: 5,
        Detector:               NewStalenessDetector(func() time.Time { return now }),
    }
    policy := NewPolicy(cfg)

    guidance := Guidance{
        Metadata: IndexMetadata{LastUpdated: now.Add(-24 * time.Hour)},
        Snippets: []Snippet{{ConfidenceScore: 0.9}},
    }

    decision := policy.Evaluate(guidance)
    if !decision.UseRetrieval {
        t.Fatalf("expected retrieval accepted, got fallback reason %q", decision.FallbackReason)
    }
    if !decision.VerificationRequired {
        t.Fatalf("expected verification required when retrieval is accepted")
    }
    if decision.FallbackReason != "" {
        t.Fatalf("expected empty fallback reason for accepted guidance, got %q", decision.FallbackReason)
    }
}

func TestPolicyRejectsLowConfidence(t *testing.T) {
    policy := NewPolicy(PolicyConfig{
        ConfidenceThreshold:    0.5,
        StalenessPolicy:        StalenessPolicyNone,
        StalenessThresholdDays: 0,
        Detector:               NewStalenessDetector(nil),
    })

    guidance := Guidance{
        Snippets: []Snippet{{ConfidenceScore: 0.3}},
    }

    decision := policy.Evaluate(guidance)
    if decision.UseRetrieval {
        t.Fatal("expected retrieval guidance to be rejected because confidence is too low")
    }
    if decision.VerificationRequired {
        t.Fatal("expected verification not required when fallback triggers")
    }
    if !strings.Contains(decision.FallbackReason, "confidence") {
        t.Fatalf("expected confidence reason, got %q", decision.FallbackReason)
    }
}

func TestPolicyRejectsStaleIndex(t *testing.T) {
    now := time.Date(2026, time.February, 27, 0, 0, 0, 0, time.UTC)
    policy := NewPolicy(PolicyConfig{
        ConfidenceThreshold:    0.5,
        StalenessPolicy:        StalenessPolicyMaxAge,
        StalenessThresholdDays: 5,
        Detector:               NewStalenessDetector(func() time.Time { return now }),
    })

    guidance := Guidance{
        Metadata: IndexMetadata{LastUpdated: now.Add(-10 * 24 * time.Hour)},
        Snippets: []Snippet{{ConfidenceScore: 0.9}},
    }

    decision := policy.Evaluate(guidance)
    if decision.UseRetrieval {
        t.Fatal("expected retrieval guidance to be rejected when index is stale")
    }
    if decision.VerificationRequired {
        t.Fatal("verification should not be required when fallback triggers due to staleness")
    }
    if !strings.Contains(decision.FallbackReason, "stale") {
        t.Fatalf("expected stale reason, got %q", decision.FallbackReason)
    }
}
