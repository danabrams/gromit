package runner

import (
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/runner/display"
)

type integrationQueueStore interface {
	Snapshot() (*integrationqueue.Snapshot, error)
}

type integrationQueueStoreFactory func(string) (integrationQueueStore, error)

var newIntegrationQueueStore integrationQueueStoreFactory = func(gromitDir string) (integrationQueueStore, error) {
	store, err := integrationqueue.NewStore(gromitDir)
	if err != nil {
		return nil, err
	}
	return store, nil
}

// ReadIntegrationQueue loads the latest queue snapshot and converts it to display
// data. If the gromit directory is empty or initialization fails, it returns nil.
func ReadIntegrationQueue(gromitDir string) (*display.IntegrationQueueStatus, error) {
	if gromitDir == "" {
		return nil, nil
	}

	store, err := newIntegrationQueueStore(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("initializing integration queue store: %w", err)
	}

	snapshot, err := store.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("loading integration queue snapshot: %w", err)
	}

	return buildIntegrationQueueStatus(snapshot), nil
}

func buildIntegrationQueueStatus(snapshot *integrationqueue.Snapshot) *display.IntegrationQueueStatus {
	projection := integrationqueue.ProjectStatus(snapshot)
	if projection == nil {
		return nil
	}

	status := &display.IntegrationQueueStatus{
		QueueLength:      projection.QueueLength,
		ReadyCount:       projection.ReadyCount,
		IntegratingCount: projection.IntegratingCount,
		BlockedCount:     projection.BlockedCount,
		MergedCount:      projection.MergedCount,
	}

	if len(projection.Entries) == 0 {
		return status
	}

	views := make([]*display.IntegrationQueueEntryView, 0, len(projection.Entries))
	for _, entry := range projection.Entries {
		views = append(views, entryToView(entry.Entry, entry.ReadyPosition))
	}

	status.Entries = views
	return status
}

func entryToView(entry *integrationqueue.Entry, readyPos int) *display.IntegrationQueueEntryView {
	if entry == nil {
		return nil
	}
	return &display.IntegrationQueueEntryView{
		Branch:           entry.Branch,
		State:            strings.ToLower(string(entry.State)),
		Lane:             entry.Lane,
		ReadyPosition:    readyPos,
		LastErrorCode:    entry.LastErrorCode,
		LastErrorMessage: entry.LastErrorMessage,
	}
}
