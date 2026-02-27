package status

import (
	"context"
	"fmt"
	"sync"

	"github.com/danabrams/gromit/internal/events"
)

// StatusWriter is the interface for writing status updates.
type StatusWriter interface {
	Write(key string, value interface{}) error
}

// StatusSubscriber consumes events and writes status updates.
type StatusSubscriber struct {
	writer  StatusWriter
	emitter *events.Emitter
	mu      sync.Mutex
}

// NewStatusSubscriber creates a new status subscriber.
func NewStatusSubscriber(writer StatusWriter, emitter *events.Emitter) *StatusSubscriber {
	return &StatusSubscriber{
		writer:  writer,
		emitter: emitter,
	}
}

// Start consumes events from the emitter until the context is cancelled or the emitter is closed.
func (s *StatusSubscriber) Start(ctx context.Context) error {
	if s.emitter == nil {
		return fmt.Errorf("emitter is nil")
	}

	ch := s.emitter.Subscribe()
	defer s.emitter.Unsubscribe(ch)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				// Channel closed, emitter is done
				return nil
			}
			s.handleEvent(event)
		case <-ctx.Done():
			// Context cancelled
			return nil
		}
	}
}

// handleEvent processes a single event and updates status if applicable.
func (s *StatusSubscriber) handleEvent(event events.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch e := event.(type) {
	case *events.RunStartEvent:
		s.writer.Write("running", true)
		s.writer.Write("max_iterations", e.MaxIterations)
		s.writer.Write("time_budget", e.TimeBudget.String())
	case *events.RunCompleteEvent:
		s.writer.Write("running", false)
		s.writer.Write("iterations_completed", e.IterationsCompleted)
		s.writer.Write("completion_reason", e.Reason)
	case *events.IterationStartEvent:
		s.writer.Write("iteration", e.Iteration)
		s.writer.Write("bead_id", e.BeadID)
		s.writer.Write("bead_title", e.BeadTitle)
		s.writer.Write("iteration_start_time", e.EventTime().Unix())
	case *events.IterationCompleteEvent:
		s.writer.Write("iteration_success", e.Success)
		s.writer.Write("iteration_duration", e.Duration.String())
	case *events.BeadCompleteEvent:
		s.writer.Write("bead_status", "complete")
	case *events.BeadFailedEvent:
		s.writer.Write("bead_status", "failed")
	case *events.BeadStuckEvent:
		s.writer.Write("bead_status", "stuck")
	case *events.BeadSkippedEvent:
		s.writer.Write("bead_status", "skipped")
	case *events.BuildStartEvent:
		s.writer.Write("phase", "build")
		s.writer.Write("model", e.Model)
	case *events.ValidationStartEvent:
		s.writer.Write("phase", "validation")
	case *events.ReviewStartEvent:
		s.writer.Write("phase", "review")
	default:
		// Ignore unknown events
	}
}
