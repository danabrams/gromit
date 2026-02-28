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

	entry := findReadyEntry(queue.Entries)
	if entry == nil {
		return nil
	}

	entry.AttemptCount++
	entry.State = StateIntegrating
	entry.RetryCount = 0
	if err := c.store.Save(*entry); err != nil {
		return fmt.Errorf("marking entry integrating: %w", err)
	}

	if err := c.gitops.FetchAndRebase(ctx, *entry); err != nil {
		return fmt.Errorf("fetch/rebase branch: %w", err)
	}

	if err := c.gate.Run(ctx, *entry); err != nil {
		entry.RetryCount = 1
		if err := c.gitops.FetchAndRebase(ctx, *entry); err != nil {
			return fmt.Errorf("fetch/rebase branch: %w", err)
		}
		if err := c.gate.Run(ctx, *entry); err != nil {
			entry.RetryCount++
			entry.State = StateFailedGates
			entry.LastErrorCode = string(StateFailedGates)
			entry.LastErrorMessage = err.Error()
			entry.LastTransitionReason = string(StateFailedGates)
			if saveErr := c.store.Save(*entry); saveErr != nil {
				return fmt.Errorf("marking entry failed_gates: %w", saveErr)
			}
			return fmt.Errorf("running scoped gates: %w", err)
		}
	}

	if err := c.gitops.MergeToMain(ctx, *entry); err != nil {
		// Check if this is a lane violation
		if strings.Contains(strings.ToLower(err.Error()), "lane violation") {
			entry.State = StateLaneViolation
			entry.LastErrorCode = "lane_violation"
			entry.LastErrorMessage = err.Error()
			entry.LastTransitionReason = "lane violation detected"
		} else {
			entry.State = StateConflict
			entry.LastErrorCode = "merge_conflict"
			entry.LastErrorMessage = err.Error()
			entry.LastTransitionReason = "merge conflict detected"
		}
		if saveErr := c.store.Save(*entry); saveErr != nil {
			return fmt.Errorf("marking entry conflict: %w", saveErr)
		}
		return fmt.Errorf("merging branch: %w", err)
	}

	if err := c.gitops.Push(ctx); err != nil {
		return fmt.Errorf("pushing main: %w", err)
	}

	if err := c.gitops.Cleanup(ctx, *entry); err != nil {
		return fmt.Errorf("cleaning up metadata: %w", err)
	}

	entry.State = StateMerged
	entry.ChangedFiles = nil
	entry.ChangedFilesHash = ""
	entry.LastErrorCode = ""
	entry.LastErrorMessage = ""
	entry.LastTransitionReason = mergedTransitionReason

	if err := c.store.Save(*entry); err != nil {
		return fmt.Errorf("finalizing entry: %w", err)
	}

	return nil
}

func findReadyEntry(entries []Entry) *Entry {
	for i := range entries {
		if entries[i].State == StateReady {
			return &entries[i]
		}
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
		if queue.Entries[i].State == StateIntegrating {
			queue.Entries[i].State = StateReady
			queue.Entries[i].LastErrorCode = "crash_recovery"
			queue.Entries[i].LastErrorMessage = "recovered from crash: entry was in integrating state"
			queue.Entries[i].LastTransitionReason = "crash recovery"
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
