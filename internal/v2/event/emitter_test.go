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
