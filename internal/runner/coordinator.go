package runner

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/integrationqueue"
)

// IntegrationCoordinator handles processing of the integration queue between iterations.
type IntegrationCoordinator struct {
	store *integrationqueue.Store
}

// NewIntegrationCoordinator creates a new coordinator for integration queue processing.
func NewIntegrationCoordinator(gromitDir string) (*IntegrationCoordinator, error) {
	store, err := integrationqueue.NewStore(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("initializing integration queue store: %w", err)
	}
	return &IntegrationCoordinator{store: store}, nil
}

// Coordinate processes the integration queue, attempting to integrate ready branches into main.
// Errors in individual branch integrations are isolated and do not terminate the process.
func (c *IntegrationCoordinator) Coordinate(ctx context.Context) error {
	if c == nil || c.store == nil {
		return nil
	}

	snapshot, err := c.store.Snapshot()
	if err != nil {
		return fmt.Errorf("loading integration queue: %w", err)
	}

	entry := integrationqueue.OldestReady(snapshot)
	if entry == nil {
		return nil
	}

	if err := integrationqueue.ApplyTransition(entry, string(integrationqueue.StateIntegrating), "coordinator: starting integration"); err != nil {
		return fmt.Errorf("transitioning entry %s to integrating: %w", entry.Branch, err)
	}
	if err := c.store.Save(*entry); err != nil {
		return fmt.Errorf("saving integrating entry %s: %w", entry.Branch, err)
	}

	if err := integrationqueue.ApplyTransition(entry, string(integrationqueue.StateConflict), "coordinator: simulated conflict"); err != nil {
		return fmt.Errorf("transitioning entry %s to conflict: %w", entry.Branch, err)
	}
	if err := c.store.Save(*entry); err != nil {
		return fmt.Errorf("saving conflict entry %s: %w", entry.Branch, err)
	}

	return nil
}

// RecoverFromCrash detects entries left in integrating state by a prior crash
// and transitions them back to a recoverable state (e.g., ready).
func (c *IntegrationCoordinator) RecoverFromCrash(ctx context.Context) error {
	if c == nil || c.store == nil {
		return nil
	}

	snapshot, err := c.store.Snapshot()
	if err != nil {
		return fmt.Errorf("loading integration queue for recovery: %w", err)
	}

	recovered := false
	for i := range snapshot.Entries {
		entry := &snapshot.Entries[i]
		if entry.State != integrationqueue.StateIntegrating {
			continue
		}
		if err := integrationqueue.ApplyTransition(entry, string(integrationqueue.StateReady), "crash recovery"); err != nil {
			return fmt.Errorf("transitioning recovered entry %s: %w", entry.Branch, err)
		}
		entry.LastErrorCode = crashRecoveryErrorCode
		entry.LastErrorMessage = crashRecoveryMessage
		if err := c.store.Save(*entry); err != nil {
			return fmt.Errorf("saving recovered entry %s: %w", entry.Branch, err)
		}
		recovered = true
	}

	if !recovered {
		return nil
	}

	return nil
}

const (
	crashRecoveryErrorCode = "crash_recovery"
	crashRecoveryMessage   = "recovered from crash: entry was in integrating state"
)
