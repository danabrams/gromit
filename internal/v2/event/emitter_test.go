package event

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEmitterDoesNotBlockOnSlowSubscriber(t *testing.T) {
	emitter := NewEmitter()

	slowStart := make(chan struct{})
	slowBlock := make(chan struct{})

	emitter.Subscribe(func(Event) {
		close(slowStart)
		<-slowBlock
	})

	emitDone := make(chan struct{})
	go func() {
		emitter.Emit(Event{Type: "slow"})
		close(emitDone)
	}()

	<-slowStart

	select {
	case <-emitDone:
		// success
	case <-time.After(50 * time.Millisecond):
		close(slowBlock)
		t.Fatal("Emit blocked on slow subscriber")
	}

	close(slowBlock)
}

func TestEmitterProcessesSubscriberEventsSequentially(t *testing.T) {
	emitter := NewEmitter()

	var (
		calls        int32
		active       atomic.Bool
		firstStarted = make(chan struct{})
		releaseFirst = make(chan struct{})
		secondDone   = make(chan struct{})
		concurrent   = make(chan struct{}, 1)
		firstOnce    sync.Once
		releaseOnce  sync.Once
		secondOnce   sync.Once
	)

	defer releaseOnce.Do(func() { close(releaseFirst) })

	emitter.Subscribe(func(Event) {
		idx := atomic.AddInt32(&calls, 1)
		if idx == 1 {
			active.Store(true)
			firstOnce.Do(func() { close(firstStarted) })
			<-releaseFirst
			active.Store(false)
			return
		}

		if active.Load() {
			select {
			case concurrent <- struct{}{}:
			default:
			}
		}

		secondOnce.Do(func() { close(secondDone) })
	})

	go emitter.Emit(Event{Type: "first"})
	<-firstStarted

	go emitter.Emit(Event{Type: "second"})

	select {
	case <-concurrent:
		t.Fatal("subscriber ran concurrently")
	case <-time.After(100 * time.Millisecond):
		// no concurrency detected yet
	}

	releaseOnce.Do(func() { close(releaseFirst) })

	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second subscriber never finished")
	}
}

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
