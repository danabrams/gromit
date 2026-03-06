package event

import (
	"log"
	"sync"
)

type subscriber struct {
	fn func(TypedEvent)

	mu     sync.Mutex
	queue  []TypedEvent
	notify chan struct{}
	done   chan struct{}
}

func newSubscriber(fn func(TypedEvent)) *subscriber {
	sub := &subscriber{
		fn:     fn,
		notify: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}

	go sub.run()
	return sub
}

func (s *subscriber) run() {
	for {
		select {
		case <-s.done:
			return
		default:
		}
		evt := s.next()
		s.dispatch(evt)
	}
}

func (s *subscriber) next() TypedEvent {
	s.mu.Lock()
	for len(s.queue) == 0 {
		s.mu.Unlock()
		<-s.notify
		s.mu.Lock()
	}

	evt := s.queue[0]
	s.queue[0] = nil
	s.queue = s.queue[1:]
	s.mu.Unlock()

	return evt
}

func (s *subscriber) enqueue(evt TypedEvent) {
	s.mu.Lock()
	s.queue = append(s.queue, evt)
	s.mu.Unlock()

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *subscriber) dispatch(evt TypedEvent) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("subscriber panic recovered: %v", r)
		}
	}()

	s.fn(evt)
}

// Emitter fans out events to registered subscribers.
type Emitter struct {
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
	done        chan struct{}
}

// NewEmitter returns a fresh Emitter.
func NewEmitter() *Emitter {
	return &Emitter{
		subscribers: make(map[*subscriber]struct{}),
		done:        make(chan struct{}),
	}
}

// Subscribe registers fn to receive emitted events.
func (e *Emitter) Subscribe(fn func(TypedEvent)) {
	sub := newSubscriber(fn)

	e.mu.Lock()
	e.subscribers[sub] = struct{}{}
	e.mu.Unlock()
}

// Emit delivers evt to all subscribers without waiting for them to complete.
func (e *Emitter) Emit(evt TypedEvent) {
	e.mu.RLock()
	for sub := range e.subscribers {
		sub.enqueue(evt)
	}
	e.mu.RUnlock()
}

// Close closes the emitter and all subscriber goroutines.
func (e *Emitter) Close() {
	e.mu.Lock()
	for sub := range e.subscribers {
		close(sub.done)
	}
	e.mu.Unlock()
}
