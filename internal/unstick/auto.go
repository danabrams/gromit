package unstick

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/logger"
)

// RestartReasonNewCommits is the reason recorded when new commits allow an automatic restart.
const RestartReasonNewCommits = "new_commits"

// BeadClient lets the AutoChecker inspect bead state when evaluating signals.
type BeadClient interface{}

// eventEmitter captures the subset of emitter behavior the AutoChecker uses.
type eventEmitter interface {
	Emit(events.Event)
}

// AutoChecker evaluates external signals and writes restart points for stuck beads.
type AutoChecker struct {
	// Client is reserved for future dependency and metadata checks.
	Client   BeadClient
	GitLogFn func(since time.Time) (bool, error)
	NowFn    func() time.Time
}

// Check inspects the stuck beads and restarts any that see new commits.
func (c *AutoChecker) Check(ctx context.Context, stuck []*bead.Bead, stats map[string]logger.BeadStats, store *Store, emitter eventEmitter) error {
	_ = ctx
	if store == nil {
		return fmt.Errorf("store is required")
	}
	if c == nil {
		return nil
	}
	gitLogFn := c.GitLogFn
	if gitLogFn == nil {
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
		hasNew, err := gitLogFn(lastAttempt)
		if err != nil {
			return fmt.Errorf("git log check for %s: %w", b.ID, err)
		}
		if !hasNew {
			continue
		}
		if err := c.emitRestart(b.ID, store, emitter); err != nil {
			return err
		}
	}

	return nil
}

func (c *AutoChecker) emitRestart(beadID string, store *Store, emitter eventEmitter) error {
	now := c.currentTime()
	store.Set(beadID, RestartPoint{
		Time:   now,
		Reason: RestartReasonNewCommits,
	})
	if err := store.Save(); err != nil {
		return fmt.Errorf("saving restart point for %s: %w", beadID, err)
	}
	if emitter != nil {
		emitter.Emit(&events.BeadUnstickedEvent{
			BeadID: beadID,
			Reason: RestartReasonNewCommits,
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
