package runner

import (
    "context"
    "fmt"
    "strings"
    "testing"
    "time"

    "github.com/danabrams/gromit/internal/tracker"
    "github.com/danabrams/gromit/internal/tracker/trackertest"
)

func TestSPCAutoTriage_CreatesIssuesForClassificationPermutations(t *testing.T) {
    ctx := context.Background()
    now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

    trackerClient := trackertest.NewStubTrackerClient()
    var createdRequests []tracker.CreateRequest
    trackerClient.CreateFn = func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
        createdRequests = append(createdRequests, req)
        return &tracker.Item{ID: fmt.Sprintf("issue-%d", len(createdRequests)), Status: tracker.StatusOpen}, nil
    }
    trackerClient.ListWithLabelFn = func(ctx context.Context, label string) ([]tracker.Item, error) {
        return nil, nil
    }

    cooldownStore := newTestCooldownStore()
    triager := NewSPCAutoTriager(trackerClient, cooldownStore, WithNowFunc(func() time.Time { return now }))

    const (
        metricCost        = "rolling_avg_cost_usd"
        metricDuration    = "rolling_avg_duration_ms"
        metricInputTokens = "rolling_avg_input_tokens"
    )

    records := []SPCCauseRecord{
        {
            Metric:                 metricCost,
            Stratum:                "",
            Class:                  CauseClassSpecial,
            PersistenceWindowCount: 2,
            Latest:                 123.4,
            DetectedAt:             now.Add(-2 * time.Hour),
            Limit: &TrendControlLimit{
                Metric: metricCost,
                Latest: 123.4,
                LCL:    100,
                UCL:    200,
            },
        },
        {
            Metric:                 metricDuration,
            Stratum:                "provider:claude",
            Class:                  CauseClassCommon,
            PersistenceWindowCount: 2,
            Latest:                 456,
            Drift:                  5,
            DetectedAt:             now.Add(-30 * time.Minute),
        },
        {
            Metric:                 metricInputTokens,
            Stratum:                "model:ultra",
            Class:                  CauseClassStable,
            PersistenceWindowCount: 4,
            Latest:                 789,
            DetectedAt:             now.Add(-15 * time.Minute),
        },
    }

    results, err := triager.Process(ctx, records)
    if err != nil {
        t.Fatalf("Process() returned error: %v", err)
    }
    if len(results) != 2 {
        t.Fatalf("expected 2 triage outcomes, got %d", len(results))
    }
    if len(createdRequests) != 2 {
        t.Fatalf("expected 2 tracker.Create calls, got %d", len(createdRequests))
    }

    for _, req := range createdRequests {
        dedupeLabel, ok := req.Metadata[metadataLabelKey]
        if !ok || !strings.HasPrefix(dedupeLabel, "spc-signal:") {
            t.Errorf("dedupe label missing or malformed in request metadata: %v", req.Metadata)
        }
    }

    var seenSpecial, seenCommon bool
    for _, req := range createdRequests {
        switch req.Metadata[metadataCauseClassKey] {
        case string(CauseClassSpecial):
            seenSpecial = true
        case string(CauseClassCommon):
            seenCommon = true
        default:
            t.Errorf("unexpected cause class %q", req.Metadata[metadataCauseClassKey])
        }
    }
    if !seenSpecial || !seenCommon {
        t.Fatalf("expected both special and common triage actions, got special=%v common=%v", seenSpecial, seenCommon)
    }
}

type testCooldownStore struct {
    values map[string]time.Time
}

func newTestCooldownStore() *testCooldownStore {
    return &testCooldownStore{values: map[string]time.Time{}}
}

func (s *testCooldownStore) Get(identity string) time.Time {
    if s == nil {
        return time.Time{}
    }
    return s.values[identity]
}

func (s *testCooldownStore) Set(identity string, when time.Time) {
    if s == nil {
        return
    }
    s.values[identity] = when
}
