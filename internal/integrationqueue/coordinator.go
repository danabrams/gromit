package integrationqueue

import (
	"context"
	"fmt"
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
	if err := c.store.Save(*entry); err != nil {
		return fmt.Errorf("marking entry integrating: %w", err)
	}

	if err := c.gitops.FetchAndRebase(ctx, *entry); err != nil {
		return fmt.Errorf("fetch/rebase branch: %w", err)
	}

	if err := c.gate.Run(ctx, *entry); err != nil {
		return fmt.Errorf("running scoped gates: %w", err)
	}

	if err := c.gitops.MergeToMain(ctx, *entry); err != nil {
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
