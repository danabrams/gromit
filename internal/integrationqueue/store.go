package integrationqueue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	queueFileName  = "integration-queue.json"
	schemaVersionV = 1
)

// Entry represents a queue entry persisted under .gromit/integration-queue.json.
type Entry struct {
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
	ChangedFiles         []string  `json:"changed_files,omitempty"`
	ChangedFilesHash     string    `json:"changed_files_hash"`
	LastErrorCode        string    `json:"last_error_code"`
	LastErrorMessage     string    `json:"last_error_message"`
	LastTransitionReason string    `json:"last_transition_reason"`
}

type queueFile struct {
	SchemaVersion int       `json:"schema_version"`
	UpdatedAt     time.Time `json:"updated_at"`
	Entries       []Entry   `json:"entries"`
}

// Store persists integration queue entries.
type Store struct {
	path string
}

// NewStore constructs a Store targeting the queue file under the provided gromit directory.
func NewStore(gromitDir string) (*Store, error) {
	if gromitDir == "" {
		return nil, fmt.Errorf("gromitDir is empty")
	}
	return &Store{
		path: filepath.Join(gromitDir, queueFileName),
	}, nil
}

// Save persists the provided entry. If the branch already exists, the entry is replaced.
func (s *Store) Save(entry Entry) error {
	file, err := s.load()
	if err != nil {
		return fmt.Errorf("loading integration queue file: %w", err)
	}

	now := time.Now().UTC()
	entry.UpdatedAt = now

	existingIdx := s.findEntryIndex(file.Entries, entry.Branch)
	if existingIdx == -1 {
		entry.FifoSeq = len(file.Entries) + 1
		entry.CreatedAt = now
		file.Entries = append(file.Entries, entry)
	} else {
		entry.FifoSeq = file.Entries[existingIdx].FifoSeq
		entry.CreatedAt = file.Entries[existingIdx].CreatedAt
		file.Entries[existingIdx] = entry
	}

	file.SchemaVersion = schemaVersionV
	file.UpdatedAt = now
	return s.write(file)
}

func (s *Store) findEntryIndex(entries []Entry, branch string) int {
	for i, entry := range entries {
		if entry.Branch == branch {
			return i
		}
	}
	return -1
}

func (s *Store) load() (*queueFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &queueFile{SchemaVersion: schemaVersionV}, nil
		}
		return nil, err
	}

	var file queueFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if file.SchemaVersion == 0 {
		file.SchemaVersion = schemaVersionV
	}
	return &file, nil
}

func (s *Store) write(file *queueFile) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating queue file directory: %w", err)
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling queue file: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing queue file temp: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming queue file temp: %w", err)
	}
	return nil
}
