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

	var wg sync.WaitGroup
	wg.Add(len(subs))

	for _, fn := range subs {
		go func(fn func(Event), evt Event) {
			defer wg.Done()
			defer func() {
				recover()
			}()
			fn(evt)
		}(fn, evt)
	}

	wg.Wait()
}
