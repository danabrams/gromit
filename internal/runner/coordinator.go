package runner

// NewIntegrationCoordinator constructs the actual integration queue coordinator used by
// the runner. The gromitDir path is used for the queue store; pass nil config to use
// default git settings.
func NewIntegrationCoordinator(gromitDir string) (Coordinator, error) {
	return newIntegrationQueueCoordinator(nil, gromitDir)
}
