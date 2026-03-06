package event

import "sync"

// Emitter fans out events to registered subscribers.
type Emitter struct {
    mu          sync.Mutex
    subscribers []func(Event)
}

// NewEmitter returns a fresh Emitter.
func NewEmitter() *Emitter {
    return &Emitter{}
}

// Subscribe registers fn to receive emitted events.
func (e *Emitter) Subscribe(fn func(Event)) {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.subscribers = append(e.subscribers, fn)
}

// Emit delivers evt to all subscribers.
func (e *Emitter) Emit(evt Event) {
    e.mu.Lock()
    subs := append([]func(Event){}, e.subscribers...)
    e.mu.Unlock()

    for _, fn := range subs {
        fn(evt)
    }
}
