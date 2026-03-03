package procutil

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForProcessCapacityCanceledBeforePressuredCheck(t *testing.T) {
	original := processCreationPressuredFn
	processCreationPressuredFn = func() (bool, error) { return false, nil }
	t.Cleanup(func() {
		processCreationPressuredFn = original
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := WaitForProcessCapacity(ctx, 50*time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitForProcessCapacity() error = %v, want context.Canceled", err)
	}
}
