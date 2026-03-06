package event

import "sync"

// Emitter fans out events to registered subscribers.
type Emitter struct {
	mu          sync.RWMutex
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

// Emit delivers evt to all subscribers without waiting for them to complete.
func (e *Emitter) Emit(evt Event) {
	e.mu.RLock()
	subs := append([]func(Event){}, e.subscribers...)
	e.mu.RUnlock()

	for _, fn := range subs {
		go func(fn func(Event), evt Event) {
			defer func() {
				recover()
			}()
			fn(evt)
		}(fn, evt)
	}
}
