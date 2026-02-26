package bead

import (
    "context"
    "testing"
)

func TestBDAdapterReadyReturnsNilWhenNoReadyBead(t *testing.T) {
    t.Parallel()

    client := &Client{
        RunFn: func(args ...string) (string, error) {
            return "[]", nil
        },
    }

    adapter := BDAdapter{client: client}

    item, err := adapter.Ready(context.Background())
    if err != nil {
        t.Fatalf("Ready() returned unexpected error: %v", err)
    }
    if item != nil {
        t.Fatalf("Ready() returned item %v, expected nil", item)
    }
}
