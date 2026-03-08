package events

import (
	"context"
	"sync"
)

// subscriberBufferSize is the buffer size for each subscriber's event channel.
const subscriberBufferSize = 100

// Emitter is a concurrency-safe event bus that distributes events to multiple subscribers.
type Emitter struct {
	mu          sync.RWMutex
	subscribers map[chan Event]bool
	waiters     chan struct{}
	closed      bool
}

// NewEmitter creates a new event emitter.
func NewEmitter() *Emitter {
	return &Emitter{
		subscribers: make(map[chan Event]bool),
		waiters:     make(chan struct{}),
	}
}

// Subscribe registers a new subscriber and returns a buffered channel for receiving events.
// The channel is buffered with subscriberBufferSize capacity.
// If the emitter is already closed, Subscribe returns a pre-closed channel so
// callers ranging over it exit immediately.
func (e *Emitter) Subscribe() chan Event {
	e.mu.Lock()
	defer e.mu.Unlock()

	ch := make(chan Event, subscriberBufferSize)
	if e.closed {
		close(ch)
		return ch
	}
	e.subscribers[ch] = true
	e.notifyWaitersLocked()
	return ch
}

// Unsubscribe removes a subscriber from the emitter.
// It's safe to call with channels that are not subscribed.
func (e *Emitter) Unsubscribe(ch chan Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.subscribers[ch]; exists {
		delete(e.subscribers, ch)
		close(ch)
		e.notifyWaitersLocked()
	}
}

// Emit sends an event to all currently subscribed channels.
// It is non-blocking per subscriber: if a subscriber's channel is full, the event is dropped for that subscriber.
// Emit is safe to call concurrently and after Close (it becomes a no-op).
func (e *Emitter) Emit(event Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return
	}

	for ch := range e.subscribers {
		select {
		case ch <- event:
			// Event sent successfully
		default:
			// Channel is full, drop event for this subscriber to avoid blocking
		}
	}
}

// HasSubscribers reports whether there are active subscribers.
func (e *Emitter) HasSubscribers() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return !e.closed && len(e.subscribers) > 0
}

// WaitForSubscribers blocks until there is at least one active subscriber or ctx expires.
func (e *Emitter) WaitForSubscribers(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	for {
		e.mu.RLock()
		hasSubscribers := !e.closed && len(e.subscribers) > 0
		closed := e.closed
		waiters := e.waiters
		e.mu.RUnlock()

		if hasSubscribers || closed {
			return nil
		}

		select {
		case <-ctx.Done():
			// Re-check in case subscribe won the race with context cancellation.
			if e.HasSubscribers() {
				return nil
			}
			return ctx.Err()
		case <-waiters:
		}
	}
}

// Close shuts down the emitter and closes all subscriber channels.
// It is safe to call multiple times.
func (e *Emitter) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}

	e.closed = true
	for ch := range e.subscribers {
		close(ch)
	}
	e.subscribers = make(map[chan Event]bool)
	e.notifyWaitersLocked()
}

func (e *Emitter) notifyWaitersLocked() {
	close(e.waiters)
	e.waiters = make(chan struct{})
}
