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

// Coordinate processes at most one ready branch in FIFO order.
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

	entry := OldestReady(queue)
	if entry == nil {
		return nil
	}

	entry.AttemptCount++
	entry.RetryCount = 0
	if err := ApplyTransition(entry, string(StateIntegrating), "coordinator: starting integration"); err != nil {
		return fmt.Errorf("transitioning entry to integrating: %w", err)
	}
	if err := c.store.Save(*entry); err != nil {
		return fmt.Errorf("marking entry integrating: %w", err)
	}

	if err := c.gitops.FetchAndRebase(ctx, *entry); err != nil {
		entry.LastErrorCode = "rebase_conflict"
		entry.LastErrorMessage = err.Error()
		if transErr := ApplyTransition(entry, string(StateConflict), "rebase conflict during initial fetch"); transErr == nil {
			_ = c.store.Save(*entry)
		}
		return fmt.Errorf("fetch/rebase branch: %w", err)
	}

	if err := c.gate.Run(ctx, *entry); err != nil {
		entry.RetryCount = 1
		if err := c.gitops.FetchAndRebase(ctx, *entry); err != nil {
			entry.LastErrorCode = "rebase_conflict"
			entry.LastErrorMessage = err.Error()
			if transErr := ApplyTransition(entry, string(StateConflict), "rebase conflict during retry fetch"); transErr == nil {
				_ = c.store.Save(*entry)
			}
			return fmt.Errorf("fetch/rebase branch: %w", err)
		}
		if err := c.gate.Run(ctx, *entry); err != nil {
			entry.RetryCount++
			entry.LastErrorCode = string(StateFailedGates)
			entry.LastErrorMessage = err.Error()
			if transErr := ApplyTransition(entry, string(StateFailedGates), "scoped gates failed after retry"); transErr == nil {
				if saveErr := c.store.Save(*entry); saveErr != nil {
					return fmt.Errorf("marking entry failed_gates: %w", saveErr)
				}
			}
			return fmt.Errorf("running scoped gates: %w", err)
		}
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
			return fmt.Errorf("marking entry conflict: %w", saveErr)
		}
		return fmt.Errorf("merging branch: %w", err)
	}

	if err := c.gitops.Push(ctx); err != nil {
		entry.LastErrorCode = "push_failed"
		entry.LastErrorMessage = err.Error()
		if transErr := ApplyTransition(entry, string(StatePushFailure), "push to remote failed"); transErr == nil {
			_ = c.store.Save(*entry)
		}
		return fmt.Errorf("pushing main: %w", err)
	}

	cleanupErr := c.gitops.Cleanup(ctx, *entry)

	entry.ChangedFiles = nil
	entry.ChangedFilesHash = ""
	entry.LastErrorCode = ""
	entry.LastErrorMessage = ""
	if transErr := ApplyTransition(entry, string(StateMerged), mergedTransitionReason); transErr != nil {
		return fmt.Errorf("applying merged transition: %w", transErr)
	}

	if err := c.store.Save(*entry); err != nil {
		return fmt.Errorf("finalizing entry: %w", err)
	}

	if cleanupErr != nil {
		return fmt.Errorf("cleaning up metadata: %w", cleanupErr)
	}

	return nil
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
		entry := &queue.Entries[i]
		if entry.State == StateIntegrating {
			targetState := StateReady
			reason := "crash recovery"
			clearError := true

			switch entry.LastErrorCode {
			case string(StateFailedGates):
				targetState = StateFailedGates
				reason = "crash recovery: failed gates"
				clearError = false
			case "merge_conflict":
				targetState = StateConflict
				reason = "crash recovery: merge conflict"
				clearError = false
			}

			if err := ApplyTransition(entry, string(targetState), reason); err != nil {
				return fmt.Errorf("transitioning recovered entry %s: %w", entry.Branch, err)
			}

			if clearError {
				entry.LastErrorCode = string(crashRecoveryErrorCode)
				entry.LastErrorMessage = "recovered from crash: entry was in integrating state"
			}

			// Save each recovered entry
			if err := c.store.Save(*entry); err != nil {
				return fmt.Errorf("saving recovered entry %s: %w", entry.Branch, err)
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
