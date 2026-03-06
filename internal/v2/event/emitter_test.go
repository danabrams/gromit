package event

import (
    "sync"
    "testing"
    "time"
)

func TestEmitterFansOutEvents(t *testing.T) {
    emitter := NewEmitter()

    var wg sync.WaitGroup
    wg.Add(2)

    events := make(chan Event, 2)
    emitter.Subscribe(func(evt Event) {
        events <- evt
        wg.Done()
    })
    emitter.Subscribe(func(evt Event) {
        events <- evt
        wg.Done()
    })

    emitter.Emit(Event{Type: "test-event"})

    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for subscribers")
    }

    close(events)
    count := 0
    for range events {
        count++
    }

    if count != 2 {
        t.Fatalf("expected 2 events, got %d", count)
    }
}

func TestEmitterIsolatesSlowAndPanicSubscribers(t *testing.T) {
	emitter := NewEmitter()

	slowStarted := make(chan struct{})
	slowContinue := make(chan struct{})
	fastDone := make(chan struct{})

	emitter.Subscribe(func(Event) {
		close(slowStarted)
		<-slowContinue
	})

	emitter.Subscribe(func(Event) {
		close(fastDone)
	})

	emitter.Subscribe(func(Event) {
		panic("subscriber panic")
	})

	emitDone := make(chan struct{})
	var emitPanic any
	go func() {
		defer func() {
			if r := recover(); r != nil {
				emitPanic = r
			}
			close(emitDone)
		}()
		emitter.Emit(Event{Type: "panic-slow"})
	}()

	var closeSlow sync.Once
	closeSlowOnce := func() {
		closeSlow.Do(func() {
			close(slowContinue)
		})
	}

	defer func() {
		closeSlowOnce()
		<-emitDone
		if emitPanic != nil && !t.Failed() {
			t.Fatalf("Emit panicked: %v", emitPanic)
		}
	}()

	<-slowStarted

	select {
	case <-fastDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("fast subscriber blocked by slow or panic")
	}

	closeSlowOnce()
}
