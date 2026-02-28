package runner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/runner/display"
)

const integrationQueueDisplayLimit = 10

var blockedQueueStates = map[string]struct{}{
	string(integrationqueue.StateConflict):      {},
	string(integrationqueue.StateFailedGates):   {},
	string(integrationqueue.StateLaneViolation): {},
}

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
	if snapshot == nil {
		return nil
	}

	var readyEntries []*integrationqueue.Entry
	var integratingEntries []*integrationqueue.Entry
	var blockedEntries []*integrationqueue.Entry

	status := &display.IntegrationQueueStatus{
		QueueLength: len(snapshot.Entries),
	}

	for i := range snapshot.Entries {
		entry := &snapshot.Entries[i]
		state := strings.ToLower(string(entry.State))
		switch state {
		case string(integrationqueue.StateReady):
			readyEntries = append(readyEntries, entry)
			status.ReadyCount++
		case string(integrationqueue.StateIntegrating):
			integratingEntries = append(integratingEntries, entry)
			status.IntegratingCount++
		case string(integrationqueue.StateMerged):
			status.MergedCount++
		default:
			if _, ok := blockedQueueStates[state]; ok {
				blockedEntries = append(blockedEntries, entry)
				status.BlockedCount++
			}
		}
	}

	sort.SliceStable(readyEntries, func(i, j int) bool {
		return readyEntries[i].FifoSeq < readyEntries[j].FifoSeq
	})

	sort.SliceStable(blockedEntries, func(i, j int) bool {
		if blockedEntries[i].UpdatedAt.Equal(blockedEntries[j].UpdatedAt) {
			return blockedEntries[i].FifoSeq > blockedEntries[j].FifoSeq
		}
		return blockedEntries[i].UpdatedAt.After(blockedEntries[j].UpdatedAt)
	})

	views := make([]*display.IntegrationQueueEntryView, 0, len(snapshot.Entries))
	for _, entry := range integratingEntries {
		views = append(views, entryToView(entry, 0))
	}
	for idx, entry := range readyEntries {
		views = append(views, entryToView(entry, idx+1))
	}
	for _, entry := range blockedEntries {
		views = append(views, entryToView(entry, 0))
	}

	if len(views) > integrationQueueDisplayLimit {
		views = views[:integrationQueueDisplayLimit]
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
