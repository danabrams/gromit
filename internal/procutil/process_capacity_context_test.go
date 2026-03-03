package procutil

import (
	"context"
	"errors"
	"sync"
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

func TestWaitForProcessCapacityDetectsContextCancelWhileNonPressured(t *testing.T) {
	original := processCreationPressuredFn
	t.Cleanup(func() {
		processCreationPressuredFn = original
	})

	called := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	processCreationPressuredFn = func() (bool, error) {
		once.Do(func() { close(called) })
		<-release
		return false, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- WaitForProcessCapacity(ctx, 50*time.Millisecond)
	}()

	<-called
	cancel()
	close(release)

	err := <-errCh
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitForProcessCapacity() error = %v, want context.Canceled", err)
	}
}
