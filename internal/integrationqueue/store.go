package integrationqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	queueFileName = "integration-queue.json"
)

// Snapshot maintains backwards compatibility for the old persisted snapshot type.
type Snapshot = Queue

// ErrSchemaInvalid indicates the persisted queue file is malformed or has
// invalid schema data that cannot be parsed into a Snapshot.
var ErrSchemaInvalid = errors.New("queue_schema_invalid")

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

	snapshot, err := s.load()
	if err != nil {
		return fmt.Errorf("loading integration queue snapshot: %w", err)
	}

	now := time.Now().UTC()
	entry.UpdatedAt = now
	sort.Strings(entry.ChangedFiles)

	existingIdx := s.findEntryIndex(snapshot.Entries, entry.Branch)
	if existingIdx == -1 {
		entry.FifoSeq = len(snapshot.Entries) + 1
		entry.CreatedAt = now
		snapshot.Entries = append(snapshot.Entries, entry)
	} else {
		entry.FifoSeq = snapshot.Entries[existingIdx].FifoSeq
		entry.CreatedAt = snapshot.Entries[existingIdx].CreatedAt
		snapshot.Entries[existingIdx] = entry
	}

	snapshot.SchemaVersion = SchemaVersion
	snapshot.UpdatedAt = now
	return s.write(snapshot)
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

// Snapshot loads the current queue snapshot.
func (s *Store) Snapshot() (*Snapshot, error) {
	return s.load()
}

func (s *Store) load() (*Snapshot, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Snapshot{SchemaVersion: SchemaVersion}, nil
		}
		return nil, err
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
	}
	if snapshot.SchemaVersion == 0 {
		snapshot.SchemaVersion = SchemaVersion
	}
	return &snapshot, nil
}

func (s *Store) write(snapshot *Snapshot) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating queue file directory: %w", err)
	}

	prepareQueueForWrite(snapshot)

	data, err := json.MarshalIndent(snapshot, "", "  ")
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

// LoadQueue reads the queue file at path using validation and returns the parsed queue.
func LoadQueue(path string) (*Queue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultQueue(), nil
		}
		return nil, err
	}

	var queue Queue
	if err := json.Unmarshal(data, &queue); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
	}
	if err := normalizeQueue(&queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

func defaultQueue() *Queue {
	return &Queue{SchemaVersion: SchemaVersion}
}

func normalizeQueue(queue *Queue) error {
	if queue == nil {
		return fmt.Errorf("queue is nil")
	}

	switch queue.SchemaVersion {
	case 0:
		queue.SchemaVersion = SchemaVersion
	case SchemaVersion:
	default:
		return fmt.Errorf("unsupported schema version %d", queue.SchemaVersion)
	}

	for i := range queue.Entries {
		if err := queue.Entries[i].Validate(); err != nil {
			return fmt.Errorf("entry %d validation failed: %w", i, err)
		}
	}
	return nil
}

// SaveQueue writes the queue using atomic rename and updates the timestamp.
func SaveQueue(path string, queue *Queue) error {
	if queue == nil {
		return fmt.Errorf("queue is nil")
	}
	if err := normalizeQueue(queue); err != nil {
		return err
	}

	queue.UpdatedAt = time.Now().UTC()
	queue.SchemaVersion = SchemaVersion
	prepareQueueForWrite(queue)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating queue file directory: %w", err)
	}

	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling queue file: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing queue file temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming queue file temp: %w", err)
	}
	return nil
}

func prepareQueueForWrite(queue *Queue) {
	if queue == nil {
		return
	}
	for i := range queue.Entries {
		sort.Strings(queue.Entries[i].ChangedFiles)
	}
	sort.SliceStable(queue.Entries, func(i, j int) bool {
		if queue.Entries[i].FifoSeq != queue.Entries[j].FifoSeq {
			return queue.Entries[i].FifoSeq < queue.Entries[j].FifoSeq
		}
		return queue.Entries[i].Branch < queue.Entries[j].Branch
	})
}

// RecoverFromMalformedQueue handles recovery when the queue file has schema errors.
// It loads the queue, resets any integrating entries to draft state with error code,
// and returns the recovered queue. This allows the system to recover from crashes
// that left entries in the integrating state.
func RecoverFromMalformedQueue(ctx context.Context, path string) (*Queue, error) {
	queue, err := LoadQueue(path)
	if err != nil {
		return nil, fmt.Errorf("loading queue during recovery: %w", err)
	}

	// Reset any integrating entries to draft state
	for i := range queue.Entries {
		if queue.Entries[i].State == StateIntegrating {
			queue.Entries[i].State = StateDraft
			queue.Entries[i].LastErrorCode = string(ErrorCodeSchemaInvalid)
			queue.Entries[i].LastErrorMessage = "recovered from schema error: entry was in integrating state"
			queue.Entries[i].UpdatedAt = time.Now()
		}
	}

	return queue, nil
}

// ErrorCodeSchemaInvalid represents a schema validation error in the queue.
const ErrorCodeSchemaInvalid ErrorCode = "queue_schema_invalid"
