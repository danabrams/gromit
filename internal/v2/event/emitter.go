package event

import "sync"

// subscriber invokes its handler sequentially as events arrive.
const subscriberBufferSize = 16

type subscriber struct {
	fn func(Event)
	ch chan Event
}

func (s *subscriber) run() {
	for evt := range s.ch {
		func() {
			defer func() {
				recover()
			}()
			s.fn(evt)
		}()
	}
}

// Emitter fans out events to registered subscribers.
type Emitter struct {
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
}

// NewEmitter returns a fresh Emitter.
func NewEmitter() *Emitter {
	return &Emitter{
		subscribers: make(map[*subscriber]struct{}),
	}
}

// Subscribe registers fn to receive emitted events.
func (e *Emitter) Subscribe(fn func(Event)) {
	sub := &subscriber{
		fn: fn,
		ch: make(chan Event, subscriberBufferSize),
	}

	e.mu.Lock()
	e.subscribers[sub] = struct{}{}
	e.mu.Unlock()

	go sub.run()
}

// Emit delivers evt to all subscribers without waiting for them to complete.
func (e *Emitter) Emit(evt Event) {
	e.mu.RLock()
	for sub := range e.subscribers {
		select {
		case sub.ch <- evt:
		default:
		}
	}
	e.mu.RUnlock()
}
