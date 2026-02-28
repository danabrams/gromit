package runner

import (
	"context"
)

// IntegrationCoordinator handles processing of the integration queue between iterations.
type IntegrationCoordinator struct{}

// NewIntegrationCoordinator creates a new coordinator for integration queue processing.
func NewIntegrationCoordinator() *IntegrationCoordinator {
	return &IntegrationCoordinator{}
}

// Coordinate processes the integration queue, attempting to integrate ready branches into main.
// Errors in individual branch integrations are isolated and do not terminate the process.
// Deprecated: Use ProcessNext instead.
func (c *IntegrationCoordinator) Coordinate(ctx context.Context) error {
	// TODO: Implement integration queue processing.
	// This is a stub implementation that will be expanded in future tasks.
	return nil
}

// ProcessNext processes one ready entry from the integration queue.
// Returns (true, nil) when an entry was processed, (false, nil) when no entries are ready
// or a terminal failure occurs, or (false, err) for non-terminal errors.
func (c *IntegrationCoordinator) ProcessNext(ctx context.Context) (bool, error) {
	// TODO: Implement single-entry processing.
	// This is a stub implementation that will be expanded in future tasks.
	return false, nil
}

// RecoverFromCrash detects entries left in integrating state by a prior crash
// and transitions them back to a recoverable state (e.g., ready).
func (c *IntegrationCoordinator) RecoverFromCrash(ctx context.Context) error {
	// TODO: Implement crash recovery for integration queue.
	// This is a stub implementation that will be expanded in future tasks.
	return nil
}
