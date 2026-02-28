package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/runner/display"
)

const integrationQueueFileName = "integration-queue.json"
const integrationQueueDisplayLimit = 10

var blockedQueueStates = map[string]struct{}{
	"conflict":       {},
	"failed_gates":   {},
	"lane_violation": {},
}

func ReadIntegrationQueue(gromitDir string) (*display.IntegrationQueueStatus, error) {
	if gromitDir == "" {
		return nil, nil
	}

	path := filepath.Join(gromitDir, integrationQueueFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading integration queue file: %w", err)
	}

	var payload integrationQueueFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parsing integration queue file: %w", err)
	}

	return buildIntegrationQueueStatus(&payload), nil
}

type integrationQueueFile struct {
	SchemaVersion int                     `json:"schema_version"`
	UpdatedAt     string                  `json:"updated_at"`
	Entries       []integrationQueueEntry `json:"entries"`
}

type integrationQueueEntry struct {
	Branch               string    `json:"branch"`
	SessionID            string    `json:"session_id"`
	OriginCommand        string    `json:"origin_command"`
	State                string    `json:"state"`
	Lane                 string    `json:"lane"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	AttemptCount         int       `json:"attempt_count"`
	RetryCount           int       `json:"retry_count"`
	FifoSeq              int       `json:"fifo_seq"`
	BaseRef              string    `json:"base_ref"`
	HeadSHA              string    `json:"head_sha"`
	ChangedFiles         []string  `json:"changed_files"`
	ChangedFilesHash     string    `json:"changed_files_hash"`
	LastErrorCode        string    `json:"last_error_code"`
	LastErrorMessage     string    `json:"last_error_message"`
	LastTransitionReason string    `json:"last_transition_reason"`
}

func buildIntegrationQueueStatus(payload *integrationQueueFile) *display.IntegrationQueueStatus {
	if payload == nil {
		return nil
	}

	var readyEntries []*integrationQueueEntry
	var integratingEntries []*integrationQueueEntry
	var blockedEntries []*integrationQueueEntry

	status := &display.IntegrationQueueStatus{
		QueueLength: len(payload.Entries),
	}

	for i := range payload.Entries {
		entry := &payload.Entries[i]
		state := strings.ToLower(entry.State)
		switch state {
		case "ready":
			readyEntries = append(readyEntries, entry)
			status.ReadyCount++
		case "integrating":
			integratingEntries = append(integratingEntries, entry)
			status.IntegratingCount++
		case "merged":
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

	views := make([]*display.IntegrationQueueEntryView, 0, len(payload.Entries))
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

func entryToView(entry *integrationQueueEntry, readyPos int) *display.IntegrationQueueEntryView {
	if entry == nil {
		return nil
	}
	return &display.IntegrationQueueEntryView{
		Branch:           entry.Branch,
		State:            strings.ToLower(entry.State),
		Lane:             entry.Lane,
		ReadyPosition:    readyPos,
		LastErrorCode:    entry.LastErrorCode,
		LastErrorMessage: entry.LastErrorMessage,
	}
}
