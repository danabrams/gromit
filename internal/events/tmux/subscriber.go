package tmux

import (
	"context"
	"fmt"
	"sync"

	"github.com/danabrams/gromit/internal/events"
)

// TMUXManager is the interface for tmux operations.
type TMUXManager interface {
	SetTitle(title string) error
}

// TMUXSubscriber consumes events and updates tmux titles.
type TMUXSubscriber struct {
	manager TMUXManager
	emitter *events.Emitter
	mu      sync.Mutex
}

// NewTMUXSubscriber creates a new TMUX subscriber.
func NewTMUXSubscriber(manager interface{}, emitter *events.Emitter) *TMUXSubscriber {
	return &TMUXSubscriber{
		manager: manager.(TMUXManager),
		emitter: emitter,
	}
}

// Start consumes events from the emitter until the context is cancelled or the emitter is closed.
func (t *TMUXSubscriber) Start(ctx context.Context) error {
	if t.emitter == nil {
		return fmt.Errorf("emitter is nil")
	}

	ch := t.emitter.Subscribe()
	defer t.emitter.Unsubscribe(ch)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				// Channel closed, emitter is done
				return nil
			}
			t.handleEvent(event)
		case <-ctx.Done():
			// Context cancelled
			return nil
		}
	}
}

// handleEvent processes a single event and updates tmux title if applicable.
func (t *TMUXSubscriber) handleEvent(event events.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch e := event.(type) {
	case *events.RunStartEvent:
		t.manager.SetTitle("gromit: starting run")
	case *events.RunCompleteEvent:
		t.manager.SetTitle(fmt.Sprintf("gromit: completed (%s)", e.Reason))
	case *events.IterationStartEvent:
		t.manager.SetTitle(fmt.Sprintf("gromit: iteration %d — %s", e.Iteration, e.BeadTitle))
	case *events.IterationCompleteEvent:
		status := "pass"
		if !e.Success {
			status = "fail"
		}
		t.manager.SetTitle(fmt.Sprintf("gromit: iteration %d — %s", e.Iteration, status))
	case *events.BeadCompleteEvent:
		t.manager.SetTitle(fmt.Sprintf("gromit: bead complete — %s", e.BeadTitle))
	case *events.BeadFailedEvent:
		t.manager.SetTitle(fmt.Sprintf("gromit: bead failed — %s", e.BeadTitle))
	default:
		// Ignore unknown events
	}
}
