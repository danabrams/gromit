package main

import (
	"bytes"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestHandleRunSignals_FirstSIGINTStopsGracefully(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	stopCh := make(chan struct{})
	var stderr bytes.Buffer
	var cancelCalls atomic.Int32
	done := make(chan struct{})

	go func() {
		handleRunSignals(sigCh, stopCh, func() {
			cancelCalls.Add(1)
		}, &stderr)
		close(done)
	}()

	sigCh <- syscall.SIGINT

	select {
	case <-stopCh:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected stop channel to close on first SIGINT")
	}

	if got := cancelCalls.Load(); got != 0 {
		t.Fatalf("cancel called on first SIGINT: got %d, want 0", got)
	}

	if stderr.Len() == 0 {
		t.Fatal("expected graceful stop message on first SIGINT")
	}

	close(sigCh)
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("signal handler did not exit after channel close")
	}
}

func TestHandleRunSignals_SecondSIGINTCancels(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	stopCh := make(chan struct{})
	var stderr bytes.Buffer
	var cancelCalls atomic.Int32
	done := make(chan struct{})

	go func() {
		handleRunSignals(sigCh, stopCh, func() {
			cancelCalls.Add(1)
		}, &stderr)
		close(done)
	}()

	sigCh <- syscall.SIGINT
	sigCh <- syscall.SIGINT

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected handler to exit after second SIGINT")
	}

	if got := cancelCalls.Load(); got != 1 {
		t.Fatalf("cancel call count = %d, want 1", got)
	}

	select {
	case <-stopCh:
	default:
		t.Fatal("expected stop channel to close on first SIGINT")
	}
}

func TestHandleRunSignals_SIGTERMCancelsImmediately(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	stopCh := make(chan struct{})
	var stderr bytes.Buffer
	var cancelCalls atomic.Int32
	done := make(chan struct{})

	go func() {
		handleRunSignals(sigCh, stopCh, func() {
			cancelCalls.Add(1)
		}, &stderr)
		close(done)
	}()

	sigCh <- syscall.SIGTERM

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected handler to exit after SIGTERM")
	}

	if got := cancelCalls.Load(); got != 1 {
		t.Fatalf("cancel call count = %d, want 1", got)
	}

	select {
	case <-stopCh:
		t.Fatal("stop channel should not close on SIGTERM")
	default:
	}
}
