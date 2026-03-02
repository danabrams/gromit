package trackertest

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/tracker"
)

func TestStubTrackerClientReadyCanBeOverridden(t *testing.T) {
	t.Parallel()

	stub := NewStubTrackerClient()
	stub.ReadyFn = func(ctx context.Context) (*tracker.Item, error) {
		if ctx == nil {
			t.Fatal("context should not be nil")
		}
		return &tracker.Item{ID: "ready"}, nil
	}

	item, err := stub.Ready(context.Background())
	if err != nil {
		t.Fatalf("Ready returned error: %v", err)
	}
	if item == nil || item.ID != "ready" {
		t.Fatalf("unexpected item: %v", item)
	}
}
