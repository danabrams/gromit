package provider

import "testing"

func TestMergeCodexEventUsagePreservesExistingUsageWhenIncomingFieldsMissing(t *testing.T) {
    existing := &codexUsage{InputTokens: 500, OutputTokens: 200}
    event := codexEvent{
        Type: "turn.completed",
        Usage: &codexUsage{InputTokens: 600},
    }

    merged, extracted := mergeCodexEventUsage(existing, event)

    if extracted == nil {
        t.Fatal("extracted usage should not be nil")
    }
    if merged == nil {
        t.Fatal("merged usage should not be nil")
    }
    if merged.InputTokens != 600 {
        t.Fatalf("merged.InputTokens = %d, want 600", merged.InputTokens)
    }
    if merged.OutputTokens != 200 {
        t.Fatalf("merged.OutputTokens = %d, want 200", merged.OutputTokens)
    }
}

func TestMergeCodexEventUsageUsesNestedResponseUsage(t *testing.T) {
    event := codexEvent{
        Type: "response.completed",
        Response: &codexResponse{
            Usage: &codexUsage{InputTokens: 300, OutputTokens: 150, TotalCostUSD: 0.03},
        },
    }

    merged, extracted := mergeCodexEventUsage(nil, event)

    if extracted == nil {
        t.Fatal("expected extracted usage from nested response")
    }
    if merged == nil {
        t.Fatal("merged usage should not be nil")
    }
    if merged.InputTokens != 300 {
        t.Fatalf("merged.InputTokens = %d, want 300", merged.InputTokens)
    }
    if merged.TotalCostUSD != 0.03 {
        t.Fatalf("merged.TotalCostUSD = %f, want 0.03", merged.TotalCostUSD)
    }
}

func TestMergeCodexEventUsageReturnsNilWhenNoUsage(t *testing.T) {
    existing := &codexUsage{InputTokens: 1}
    event := codexEvent{Type: "turn.completed"}

    merged, extracted := mergeCodexEventUsage(existing, event)

    if extracted != nil {
        t.Fatalf("expected nil extracted usage, got %+v", extracted)
    }
    if merged != existing {
        t.Fatalf("merged usage pointer should be unchanged")
    }
}
