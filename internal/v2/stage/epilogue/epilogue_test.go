package epilogue

import (
    "context"
    "testing"
    "time"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/events"
    "github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestEpilogueStageSuccessClosesBeadAndEmitsCostEvent(t *testing.T) {
    t.Parallel()

    tracker := &fakeTracker{}
    stageInstance, err := New(&config.Config{}, tracker)
    if err != nil {
        t.Fatalf("unexpected constructor error: %v", err)
    }

    telemetry := &stagepkg.LLMCostSummary{
        Model:        "haiku",
        Duration:     2 * time.Second,
        InputTokens:  10,
        OutputTokens: 5,
        CostUSD:      0.42,
    }

    req := &stagepkg.Request{
        Bead:      stagepkg.BeadInfo{ID: "bead-123"},
        Model:     "haiku",
        Telemetry: telemetry,
    }

    res, err := stageInstance.Run(context.Background(), req)
    if err != nil {
        t.Fatalf("Run() error = %v", err)
    }

    if len(tracker.closeCalls) != 1 || tracker.closeCalls[0] != "bead-123" {
        t.Fatalf("expected CloseBead on bead-123, got %v", tracker.closeCalls)
    }

    if res == nil {
        t.Fatal("expected result")
    }
    if res.Decision != stagepkg.DecisionProceed {
        t.Fatalf("decision = %v, want proceed", res.Decision)
    }
    if len(res.Events) != 1 {
        t.Fatalf("events count = %d, want 1", len(res.Events))
    }

    completionEvent, ok := res.Events[0].(*events.BeadCompleteEvent)
    if !ok {
        t.Fatalf("event type = %T, want *events.BeadCompleteEvent", res.Events[0])
    }
    if completionEvent.BeadID != "bead-123" {
        t.Fatalf("bead ID = %q, want %q", completionEvent.BeadID, "bead-123")
    }
    if completionEvent.Model != telemetry.Model {
        t.Fatalf("model = %q, want %q", completionEvent.Model, telemetry.Model)
    }
    if completionEvent.Duration != telemetry.Duration {
        t.Fatalf("duration = %v, want %v", completionEvent.Duration, telemetry.Duration)
    }
    if completionEvent.InputTokens != telemetry.InputTokens {
        t.Fatalf("input tokens = %d, want %d", completionEvent.InputTokens, telemetry.InputTokens)
    }
    if completionEvent.OutputTokens != telemetry.OutputTokens {
        t.Fatalf("output tokens = %d, want %d", completionEvent.OutputTokens, telemetry.OutputTokens)
    }
    if completionEvent.CostUSD != telemetry.CostUSD {
        t.Fatalf("cost = %v, want %v", completionEvent.CostUSD, telemetry.CostUSD)
    }
}

type fakeTracker struct {
    closeCalls []string
}

func (f *fakeTracker) NextBead(ctx context.Context) (*tasktracker.Bead, error) {
    return nil, nil
}

func (f *fakeTracker) ShowBead(ctx context.Context, beadID string) (*tasktracker.Bead, error) {
    return nil, nil
}

func (f *fakeTracker) CreateBead(ctx context.Context, title, description string, priority int, labels, dependencies []string) (*tasktracker.Bead, error) {
    return nil, nil
}

func (f *fakeTracker) CloseBead(ctx context.Context, beadID string) error {
    f.closeCalls = append(f.closeCalls, beadID)
    return nil
}

func (f *fakeTracker) QueryBeads(ctx context.Context, labels []string, status, parent string) ([]tasktracker.Bead, error) {
    return nil, nil
}
