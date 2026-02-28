package integrationqueue

import (
	"context"
	"fmt"
	"strings"
)

const mergedTransitionReason = "coordinator: merged"

// ScopedGate performs the scoped gate checks required before integration.
type ScopedGate interface {
	Run(ctx context.Context, entry Entry) error
}

// Coordinator drives the integration queue lifecycle for a single branch per cycle.
type Coordinator struct {
	store  *Store
	gitops GitOps
	gate   ScopedGate
}

// NewCoordinator wires together the store, gitops adapter, and scoped gate runner.
func NewCoordinator(store *Store, gitops GitOps, gate ScopedGate) *Coordinator {
	return &Coordinator{store: store, gitops: gitops, gate: gate}
}

// ProcessNext processes one ready entry from the queue. Returns (true, nil) when
// an entry was processed, (false, nil) when no entries are ready or a terminal
// failure occurs, or (false, err) for non-terminal errors.
func (c *Coordinator) ProcessNext(ctx context.Context) (bool, error) {
	if c.store == nil {
		return false, fmt.Errorf("store is required")
	}
	if c.gitops == nil {
		return false, fmt.Errorf("gitops adapter is required")
	}
	if c.gate == nil {
		return false, fmt.Errorf("scoped gate runner is required")
	}

	queue, err := c.store.load()
	if err != nil {
		return false, fmt.Errorf("loading integration queue: %w", err)
	}

	entry := OldestReady(queue)
	if entry == nil {
		return false, nil
	}

	// Try to process this entry
	processed, err := c.processEntry(ctx, entry)
	if err != nil {
		// Check if this is a terminal failure that shouldn't block other entries
		if isTerminalFailure(err) {
			return false, nil
		}
		// Non-terminal failure blocks processing
		return false, err
	}

	return processed, nil
}

// Coordinate processes ready branches in FIFO order, skipping terminal failures.
// Terminal failures (conflict, failed_gates, lane_violation) do not block
// FIFO progression for subsequent ready entries.
func (c *Coordinator) Coordinate(ctx context.Context) error {
	if c.store == nil {
		return fmt.Errorf("store is required")
	}
	if c.gitops == nil {
		return fmt.Errorf("gitops adapter is required")
	}
	if c.gate == nil {
		return fmt.Errorf("scoped gate runner is required")
	}

	queue, err := c.store.load()
	if err != nil {
		return fmt.Errorf("loading integration queue: %w", err)
	}

	// Process ready entries in FIFO order until one succeeds or all have terminal failures
	for {
		entry := OldestReady(queue)
		if entry == nil {
			return nil
		}

		// Try to process this entry
		processed, err := c.processEntry(ctx, entry)
		if err != nil {
			// Check if this is a terminal failure that shouldn't block other entries
			if isTerminalFailure(err) {
				// Reload queue to get updated state and try next entry
				queue, err = c.store.load()
				if err != nil {
					return fmt.Errorf("reloading queue after terminal failure: %w", err)
				}
				continue
			}
			// Non-terminal failure blocks processing
			return err
		}

		// If we successfully processed an entry, return success
		if processed {
			return nil
		}

		// No more ready entries
		return nil
	}
}

// processEntry attempts to integrate a single entry. Returns (true, nil) on success,
// (false, err) on terminal failures that don't block other entries, or (false, err)
// on non-terminal failures. Terminal failures are returned as errors that
// isTerminalFailure() will recognize.
func (c *Coordinator) processEntry(ctx context.Context, entry *Entry) (bool, error) {
	entry.AttemptCount++
	entry.RetryCount = 0
	if err := ApplyTransition(entry, string(StateIntegrating), "coordinator: starting integration"); err != nil {
		return false, fmt.Errorf("transitioning entry to integrating: %w", err)
	}
	if err := c.store.Save(*entry); err != nil {
		return false, fmt.Errorf("marking entry integrating: %w", err)
	}

	if err := c.gitops.FetchAndRebase(ctx, *entry); err != nil {
		entry.LastErrorCode = "rebase_conflict"
		entry.LastErrorMessage = err.Error()
		if transErr := ApplyTransition(entry, string(StateConflict), "rebase conflict during initial fetch"); transErr == nil {
			_ = c.store.Save(*entry)
		}
		return false, markTerminalFailure(fmt.Errorf("fetch/rebase branch: %w", err))
	}

	if err := c.runScopedGateWithRetry(ctx, entry); err != nil {
		return false, err
	}

	if err := c.gitops.MergeToMain(ctx, *entry); err != nil {
		// Check if this is a lane violation
		if strings.Contains(strings.ToLower(err.Error()), "lane violation") {
			entry.LastErrorCode = "lane_violation"
			entry.LastErrorMessage = err.Error()
			_ = ApplyTransition(entry, string(StateLaneViolation), "lane violation detected")
		} else {
			entry.LastErrorCode = "merge_conflict"
			entry.LastErrorMessage = err.Error()
			_ = ApplyTransition(entry, string(StateConflict), "merge conflict detected")
		}
		if saveErr := c.store.Save(*entry); saveErr != nil {
			return false, fmt.Errorf("marking entry conflict: %w", saveErr)
		}
		return false, markTerminalFailure(fmt.Errorf("merging branch: %w", err))
	}

	if err := c.gitops.Push(ctx); err != nil {
		entry.LastErrorCode = "push_failed"
		entry.LastErrorMessage = err.Error()
		if transErr := ApplyTransition(entry, string(StateFailedGates), "push to remote failed"); transErr == nil {
			_ = c.store.Save(*entry)
		}
		return false, fmt.Errorf("pushing main: %w", err)
	}

	cleanupErr := c.gitops.Cleanup(ctx, *entry)

	entry.ChangedFiles = nil
	entry.ChangedFilesHash = ""
	entry.LastErrorCode = ""
	entry.LastErrorMessage = ""
	if transErr := ApplyTransition(entry, string(StateMerged), mergedTransitionReason); transErr != nil {
		return false, fmt.Errorf("applying merged transition: %w", transErr)
	}

	if err := c.store.Save(*entry); err != nil {
		return false, fmt.Errorf("finalizing entry: %w", err)
	}

	if cleanupErr != nil {
		return false, fmt.Errorf("cleaning up metadata: %w", cleanupErr)
	}

	return true, nil
}

// terminalFailureMarker is a sentinel type to wrap errors that are terminal failures
type terminalFailureMarker struct {
	err error
}

// markTerminalFailure wraps an error to indicate it's a terminal failure
func markTerminalFailure(err error) error {
	return &terminalFailureMarker{err: err}
}

// isTerminalFailure checks if an error is a terminal failure that shouldn't block other entries
func isTerminalFailure(err error) bool {
	_, ok := err.(*terminalFailureMarker)
	return ok
}

// Error implements the error interface for terminalFailureMarker
func (t *terminalFailureMarker) Error() string {
	return t.err.Error()
}

func (c *Coordinator) runScopedGateWithRetry(ctx context.Context, entry *Entry) error {
	tracker := newGateRetryTracker(Lane(entry.Lane))

	if err := c.gate.Run(ctx, *entry); err != nil {
		return c.handleGateFailure(ctx, entry, &tracker, err)
	}

	return nil
}

func (c *Coordinator) handleGateFailure(ctx context.Context, entry *Entry, tracker *gateRetryTracker, err error) error {
	if tracker.CanRetry() {
		tracker.RecordRetry(entry)
		if err := c.gitops.FetchAndRebase(ctx, *entry); err != nil {
			entry.LastErrorCode = "rebase_conflict"
			entry.LastErrorMessage = err.Error()
			if transErr := ApplyTransition(entry, string(StateConflict), "rebase conflict during retry fetch"); transErr == nil {
				_ = c.store.Save(*entry)
			}
			return markTerminalFailure(fmt.Errorf("fetch/rebase branch: %w", err))
		}
		if err := c.gate.Run(ctx, *entry); err != nil {
			return c.transitionToFailedGates(entry, err, "scoped gates failed after retry")
		}
		return nil
	}
	return c.transitionToFailedGates(entry, err, "scoped gates failed")
}

func (c *Coordinator) transitionToFailedGates(entry *Entry, err error, reason string) error {
	entry.LastErrorCode = string(StateFailedGates)
	entry.LastErrorMessage = err.Error()
	if transErr := ApplyTransition(entry, string(StateFailedGates), reason); transErr == nil {
		if saveErr := c.store.Save(*entry); saveErr != nil {
			return fmt.Errorf("marking entry failed_gates: %w", saveErr)
		}
	}
	return markTerminalFailure(fmt.Errorf("running scoped gates: %w", err))
}

// RecoverFromCrash detects entries left in StateIntegrating and transitions
// them back to StateReady. This is called during startup to handle entries
// that were stranded mid-integration by a prior crash.
func (c *Coordinator) RecoverFromCrash(ctx context.Context) error {
	if c.store == nil {
		return fmt.Errorf("store is required")
	}

	queue, err := c.store.load()
	if err != nil {
		return fmt.Errorf("loading integration queue for recovery: %w", err)
	}

	// Find and reset any entries in integrating state
	recovered := false
	for i := range queue.Entries {
		if queue.Entries[i].State == StateIntegrating {
			if err := ApplyTransition(&queue.Entries[i], string(StateReady), "crash recovery"); err != nil {
				return fmt.Errorf("transitioning recovered entry %s: %w", queue.Entries[i].Branch, err)
			}
			queue.Entries[i].LastErrorCode = string(crashRecoveryErrorCode)
			queue.Entries[i].LastErrorMessage = "recovered from crash: entry was in integrating state"
			// Save each recovered entry
			if err := c.store.Save(queue.Entries[i]); err != nil {
				return fmt.Errorf("saving recovered entry %s: %w", queue.Entries[i].Branch, err)
			}
			recovered = true
		}
	}

	// If no entries needed recovery, just return early
	if !recovered {
		return nil
	}

	return nil
}

const crashRecoveryErrorCode ErrorCode = "crash_recovery"
