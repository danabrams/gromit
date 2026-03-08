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
	once   sync.Once
}

func newSubscriber(fn func(TypedEvent), wg *sync.WaitGroup) *subscriber {
	sub := &subscriber{
		fn:     fn,
		notify: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		sub.run()
	}()
	return sub
}

func (s *subscriber) run() {
	for {
		select {
		case <-s.done:
			s.drain()
			return
		default:
		}
		evt, ok := s.next()
		if !ok {
			s.drain()
			return
		}
		s.dispatch(evt)
	}
}

// drain processes any events remaining in the queue after the done channel is closed.
func (s *subscriber) drain() {
	s.mu.Lock()
	remaining := append([]TypedEvent(nil), s.queue...)
	s.queue = nil
	s.mu.Unlock()

	for _, evt := range remaining {
		s.dispatch(evt)
	}
}

func (s *subscriber) next() (TypedEvent, bool) {
	s.mu.Lock()
	for len(s.queue) == 0 {
		s.mu.Unlock()

		select {
		case <-s.notify:
		case <-s.done:
			return nil, false
		}

		s.mu.Lock()
	}

	evt := s.queue[0]
	s.queue[0] = nil
	s.queue = s.queue[1:]
	s.mu.Unlock()

	return evt, true
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

func (s *subscriber) stop() {
	s.once.Do(func() {
		close(s.done)
	})
}

// Emitter fans out events to registered subscribers.
type Emitter struct {
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
	done        chan struct{}
	closeOnce   sync.Once
	wg          sync.WaitGroup
}

// NewEmitter returns a fresh Emitter.
func NewEmitter() *Emitter {
	return &Emitter{
		subscribers: make(map[*subscriber]struct{}),
		done:        make(chan struct{}),
	}
}

// Subscribe registers fn to receive emitted events.
// The returned function may be called to unsubscribe.
func (e *Emitter) Subscribe(fn func(TypedEvent)) func() {
	sub := newSubscriber(fn, &e.wg)

	e.mu.Lock()
	e.subscribers[sub] = struct{}{}
	e.mu.Unlock()

	return func() {
		e.mu.Lock()
		if _, ok := e.subscribers[sub]; ok {
			delete(e.subscribers, sub)
			sub.stop()
		}
		e.mu.Unlock()
	}
}

// Emit delivers evt to all subscribers without waiting for them to complete.
func (e *Emitter) Emit(evt TypedEvent) {
	e.mu.RLock()
	for sub := range e.subscribers {
		sub.enqueue(evt)
	}
	e.mu.RUnlock()
}

// Close closes the emitter and waits for all subscriber goroutines to finish
// processing queued events. It is safe to call Close multiple times; only the
// first call has any effect.
func (e *Emitter) Close() {
	e.closeOnce.Do(func() {
		e.mu.Lock()
		for sub := range e.subscribers {
			delete(e.subscribers, sub)
			sub.stop()
		}
		e.mu.Unlock()
		e.wg.Wait()
	})
}
