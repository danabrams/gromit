package events

import (
	"sync"
	"sync/atomic"
	"time"
)

// subscriberBufferSize is the buffer size for each subscriber's event channel.
const subscriberBufferSize = 100

// Emitter is a concurrency-safe event bus that distributes events to multiple subscribers.
type Emitter struct {
	mu           sync.RWMutex
	subscribers  map[chan Event]bool
	closed       bool
	droppedCount int64
}

// NewEmitter creates a new event emitter.
func NewEmitter() *Emitter {
	return &Emitter{
		subscribers: make(map[chan Event]bool),
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
	}
}

// Emit sends an event to all currently subscribed channels.
// It is non-blocking per subscriber: if a subscriber's channel is full, the event is dropped for that subscriber.
// Emit is safe to call concurrently and after Close (it becomes a no-op).
func (e *Emitter) Emit(event Event) {
	e.mu.RLock()

	if e.closed {
		e.mu.RUnlock()
		return
	}

	dropped := int64(0)
	for ch := range e.subscribers {
		select {
		case ch <- event:
			// Event sent successfully
		default:
			// Channel is full, drop event for this subscriber to avoid blocking
			dropped++
		}
	}
	e.mu.RUnlock()

	if dropped == 0 {
		return
	}

	totalDrops := atomic.AddInt64(&e.droppedCount, dropped)
	e.emitDroppedEventsEvent(dropped, totalDrops)
}

// HasSubscribers reports whether there are active subscribers.
func (e *Emitter) HasSubscribers() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return !e.closed && len(e.subscribers) > 0
}

// DroppedCount returns the total number of events that were dropped due to full subscriber channels.
func (e *Emitter) DroppedCount() int64 {
	return atomic.LoadInt64(&e.droppedCount)
}

func (e *Emitter) emitDroppedEventsEvent(delta, total int64) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed || len(e.subscribers) == 0 {
		return
	}

	event := &DroppedEventsEvent{
		Count:     delta,
		Total:     total,
		TimeMixin: TimeMixin{Time: time.Now()},
	}

	for ch := range e.subscribers {
		select {
		case ch <- event:
		default:
			// Drop notification for slow subscribers but do not count it to avoid loops.
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
}
