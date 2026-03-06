package event

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type dummyEvent struct {
	Event
}

func (dummyEvent) EventType() string { return "dummy" }

func TestEmitterDoesNotBlockOnSlowSubscriber(t *testing.T) {
	emitter := NewEmitter()

	slowStart := make(chan struct{})
	slowBlock := make(chan struct{})

	emitter.Subscribe(func(TypedEvent) {
		close(slowStart)
		<-slowBlock
	})

	emitDone := make(chan struct{})
	go func() {
		emitter.Emit(dummyEvent{Event: Event{Type: "slow"}})
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

func TestEmitterBackpressureIsolation(t *testing.T) {
	emitter := NewEmitter()

	start := make(chan struct{})
	block := make(chan struct{})
	var release sync.Once

	emitter.Subscribe(func(TypedEvent) {
		close(start)
		<-block
	})

	go emitter.Emit(dummyEvent{Event: Event{Type: "first"}})
	<-start

	emitDone := make(chan struct{})
	go func() {
		emitter.Emit(dummyEvent{Event: Event{Type: "second"}})
		close(emitDone)
	}()

	defer release.Do(func() { close(block) })

	select {
	case <-emitDone:
		// success
	case <-time.After(50 * time.Millisecond):
		release.Do(func() { close(block) })
		t.Fatal("Emit blocked on slow subscriber despite isolation")
	}
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

	emitter.Subscribe(func(TypedEvent) {
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

	go emitter.Emit(dummyEvent{Event: Event{Type: "first"}})
	<-firstStarted

	go emitter.Emit(dummyEvent{Event: Event{Type: "second"}})

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

	events := make(chan TypedEvent, 2)
	emitter.Subscribe(func(evt TypedEvent) {
		events <- evt
		wg.Done()
	})
	emitter.Subscribe(func(evt TypedEvent) {
		events <- evt
		wg.Done()
	})

	emitter.Emit(dummyEvent{Event: Event{Type: "test-event"}})

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

func TestEmitterFansOutAllEventsDespiteSlowSubscriber(t *testing.T) {
	emitter := NewEmitter()
	const totalEvents = 64
	started := make(chan struct{})
	release := make(chan struct{})
	processed := make(chan struct{}, totalEvents)
	var once sync.Once

	emitter.Subscribe(func(TypedEvent) {
		once.Do(func() { close(started) })
		<-release
		processed <- struct{}{}
	})

	emitter.Emit(dummyEvent{Event: Event{Type: "slow-handshake"}})
	<-started

	for i := 0; i < totalEvents-1; i++ {
		emitter.Emit(dummyEvent{Event: Event{Type: "slow"}})
	}

	close(release)

	for i := 0; i < totalEvents; i++ {
		select {
		case <-processed:
		case <-time.After(time.Second):
			t.Fatalf("event %d never processed", i)
		}
	}
}

func TestEmitterIsolatesSlowAndPanicSubscribers(t *testing.T) {
	emitter := NewEmitter()

	slowStarted := make(chan struct{})
	slowContinue := make(chan struct{})
	fastDone := make(chan struct{})

	emitter.Subscribe(func(TypedEvent) {
		close(slowStarted)
		<-slowContinue
	})

	emitter.Subscribe(func(TypedEvent) {
		close(fastDone)
	})

	emitter.Subscribe(func(TypedEvent) {
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
		emitter.Emit(dummyEvent{Event: Event{Type: "panic-slow"}})
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
