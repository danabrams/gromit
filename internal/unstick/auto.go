package unstick

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/logger"
)

const (
	// RestartReasonClosedDependency is recorded when a dependency closed signal fires.
	RestartReasonClosedDependency = "closed_dependency"
	// RestartReasonMetadataChanged is recorded when metadata changed since last attempt.
	RestartReasonMetadataChanged = "metadata_changed"
)

// BeadClient lets the AutoChecker inspect bead state when evaluating signals.
type BeadClient interface {
	ClosedDependencySignal(ctx context.Context, beadID string) (bool, error)
	MetadataChangedSignal(ctx context.Context, beadID string) (bool, error)
}

// eventEmitter captures the subset of emitter behavior the AutoChecker uses.
type eventEmitter interface {
	Emit(events.Event)
}

// AutoChecker evaluates external signals and writes restart points for stuck beads.
type AutoChecker struct {
	Client BeadClient
	NowFn  func() time.Time
}

// Check inspects the stuck beads and restarts any that see new signals.
func (c *AutoChecker) Check(ctx context.Context, stuck []*bead.Bead, stats map[string]logger.BeadStats, store *Store, emitter eventEmitter) error {
	if store == nil {
		return fmt.Errorf("store is required")
	}
	if c == nil || c.Client == nil {
		return nil
	}

	for _, b := range stuck {
		if b == nil || b.ID == "" {
			continue
		}

		lastAttempt := stats[b.ID].LastAttempt
		if c.shouldSkipRestart(b.ID, lastAttempt, store) {
			continue
		}

		closed, err := c.Client.ClosedDependencySignal(ctx, b.ID)
		if err != nil {
			return fmt.Errorf("closed-dependency signal for %s: %w", b.ID, err)
		}
		metadataChanged, err := c.Client.MetadataChangedSignal(ctx, b.ID)
		if err != nil {
			return fmt.Errorf("metadata-changed signal for %s: %w", b.ID, err)
		}

		reason := reasonForSignals(closed, metadataChanged)
		if reason == "" {
			continue
		}

		if err := c.emitRestart(b.ID, store, emitter, reason); err != nil {
			return err
		}
	}

	return nil
}

func reasonForSignals(closed, metadataChanged bool) string {
	if metadataChanged {
		return RestartReasonMetadataChanged
	}
	if closed {
		return RestartReasonClosedDependency
	}
	return ""
}

func (c *AutoChecker) emitRestart(beadID string, store *Store, emitter eventEmitter, reason string) error {
	now := c.currentTime()
	store.Set(beadID, RestartPoint{Time: now, Reason: reason})
	if err := store.Save(); err != nil {
		return fmt.Errorf("saving restart point for %s: %w", beadID, err)
	}
	if emitter != nil {
		emitter.Emit(&events.BeadUnstickedEvent{
			BeadID: beadID,
			Reason: reason,
		})
	}
	return nil
}

func (c *AutoChecker) currentTime() time.Time {
	if c != nil && c.NowFn != nil {
		return c.NowFn()
	}
	return time.Now().UTC()
}

func (c *AutoChecker) shouldSkipRestart(beadID string, lastAttempt time.Time, store *Store) bool {
	if store == nil || lastAttempt.IsZero() {
		return false
	}
	point, ok := store.Get(beadID)
	if !ok || point.Time.IsZero() {
		return false
	}
	return point.Time.After(lastAttempt)
}
