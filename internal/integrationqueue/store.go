package integrationqueue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	queueFileName  = "integration-queue.json"
	schemaVersionV = 1
)

type queueFile struct {
	SchemaVersion int       `json:"schema_version"`
	UpdatedAt     time.Time `json:"updated_at"`
	Entries       []Entry   `json:"entries"`
}

// Store persists integration queue entries.
type Store struct {
	path            string
	validationHooks []ValidationHook
}

// StoreOption configures a Store.
type StoreOption func(*Store)

// ValidationHook runs custom validation before entries are persisted.
type ValidationHook func(entry Entry) error

// WithValidationHook registers a hook to run when Save is invoked.
func WithValidationHook(h ValidationHook) StoreOption {
	return func(s *Store) {
		if h == nil {
			return
		}
		s.validationHooks = append(s.validationHooks, h)
	}
}

// NewStore constructs a Store targeting the queue file under the provided gromit directory.
func NewStore(gromitDir string, opts ...StoreOption) (*Store, error) {
	if gromitDir == "" {
		return nil, fmt.Errorf("gromitDir is empty")
	}
	store := &Store{
		path: filepath.Join(gromitDir, queueFileName),
	}
	for _, opt := range opts {
		opt(store)
	}
	return store, nil
}

// Save persists the provided entry. If the branch already exists, the entry is replaced.
func (s *Store) Save(entry Entry) error {
	sort.Strings(entry.ChangedFiles)
	if err := s.runValidationHooks(entry); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	file, err := s.load()
	if err != nil {
		return fmt.Errorf("loading integration queue file: %w", err)
	}

	now := time.Now().UTC()
	entry.UpdatedAt = now
	sort.Strings(entry.ChangedFiles)

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

func (s *Store) runValidationHooks(entry Entry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	for _, hook := range s.validationHooks {
		if err := hook(entry); err != nil {
			return err
		}
	}
	return nil
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

	for i := range file.Entries {
		sort.Strings(file.Entries[i].ChangedFiles)
	}
	sort.SliceStable(file.Entries, func(i, j int) bool {
		if file.Entries[i].FifoSeq != file.Entries[j].FifoSeq {
			return file.Entries[i].FifoSeq < file.Entries[j].FifoSeq
		}
		return file.Entries[i].Branch < file.Entries[j].Branch
	})

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
