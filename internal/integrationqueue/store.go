package integrationqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

const (
	queueFileName = "integration-queue.json"
)

// trimJSONPrefix finds the first '{' in data and returns from there.
// This tolerates non-JSON text (e.g. diagnostic output from editors)
// prepended before the actual JSON content.
// If no '{' is found, the original data is returned unchanged so that
// json.Unmarshal produces the appropriate error.
func trimJSONPrefix(data []byte) []byte {
	idx := bytes.IndexByte(data, '{')
	if idx <= 0 {
		return data
	}
	return data[idx:]
}

// Snapshot maintains backwards compatibility for the old persisted snapshot type.
type Snapshot = Queue

// ErrSchemaInvalid indicates the persisted queue file is malformed or has
// invalid schema data that cannot be parsed into a Snapshot.
var ErrSchemaInvalid = errors.New("queue_schema_invalid")

// withQueueFileLock acquires an exclusive advisory lock on a lock file adjacent to path,
// executes fn, then releases the lock. This prevents concurrent processes from
// reading/writing the queue file simultaneously.
func withQueueFileLock(path string, fn func() error) error {
	lockPath := path + ".lock"
	if dir := filepath.Dir(lockPath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating queue lock directory: %w", err)
		}
	}
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening queue lock file: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring queue file lock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	return fn()
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

	return withQueueFileLock(s.path, func() error {
		snapshot, err := s.loadUnlocked()
		if err != nil {
			return fmt.Errorf("loading integration queue snapshot: %w", err)
		}

		now := time.Now().UTC()
		entry.UpdatedAt = now
		sort.Strings(entry.ChangedFiles)

		existingIdx := s.findEntryIndex(snapshot.Entries, entry.Branch)
		if existingIdx == -1 {
			entry.FifoSeq = nextFifoSeq(snapshot.Entries)
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
	})
}

func (s *Store) findEntryIndex(entries []Entry, branch string) int {
	for i, entry := range entries {
		if entry.Branch == branch {
			return i
		}
	}
	return -1
}

func nextFifoSeq(entries []Entry) int {
	return maxFifoSeq(entries) + 1
}

func maxFifoSeq(entries []Entry) int {
	max := 0
	for _, entry := range entries {
		if entry.FifoSeq > max {
			max = entry.FifoSeq
		}
	}
	return max
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
	var snapshot *Snapshot
	err := withQueueFileLock(s.path, func() error {
		var loadErr error
		snapshot, loadErr = s.loadUnlocked()
		return loadErr
	})
	return snapshot, err
}

func (s *Store) loadUnlocked() (*Snapshot, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Snapshot{SchemaVersion: SchemaVersion}, nil
		}
		return nil, err
	}

	data = trimJSONPrefix(data)
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

	return verifyWritten(s.path)
}

// LoadQueue reads the queue file at path using validation and returns the parsed queue.
func LoadQueue(path string) (*Queue, error) {
	var queue *Queue
	err := withQueueFileLock(path, func() error {
		var loadErr error
		queue, loadErr = loadQueueUnlocked(path)
		return loadErr
	})
	return queue, err
}

func loadQueueUnlocked(path string) (*Queue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultQueue(), nil
		}
		return nil, err
	}

	data = trimJSONPrefix(data)
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
	return withQueueFileLock(path, func() error {
		return saveQueueUnlocked(path, queue)
	})
}

func saveQueueUnlocked(path string, queue *Queue) error {
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

	return verifyWritten(path)
}

// verifyWritten reads back the file at path and verifies it contains valid JSON.
func verifyWritten(path string) error {
	readBack, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("write verification read-back failed: %w", err)
	}
	var check json.RawMessage
	if err := json.Unmarshal(readBack, &check); err != nil {
		return fmt.Errorf("write verification failed: written file is not valid JSON: %w", err)
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
// It loads the queue, transitions any integrating entries to ready with error code,
// persists the recovered queue, and returns it.
func RecoverFromMalformedQueue(ctx context.Context, path string) (*Queue, error) {
	var queue *Queue
	err := withQueueFileLock(path, func() error {
		var loadErr error
		queue, loadErr = loadQueueForRecovery(path)
		if loadErr != nil {
			return fmt.Errorf("loading queue during recovery: %w", loadErr)
		}

		_ = ctx
		updated := queue.SchemaVersion != SchemaVersion
		queue.SchemaVersion = SchemaVersion
		for i := range queue.Entries {
        if queue.Entries[i].State == StateIntegrating {
            if err := ApplyTransition(&queue.Entries[i], string(StateReady), "schema recovery"); err != nil {
                return fmt.Errorf("transitioning recovered entry %s: %w", queue.Entries[i].Branch, err)
            }
            if queue.Entries[i].LastErrorCode == "" {
                queue.Entries[i].LastErrorCode = string(ErrorCodeSchemaInvalid)
                queue.Entries[i].LastErrorMessage = "recovered from schema error: entry was in integrating state"
            }
            updated = true
        }
		}

		if updated {
			if err := saveQueueUnlocked(path, queue); err != nil {
				return fmt.Errorf("persisting recovered queue: %w", err)
			}
		}

		return nil
	})
	return queue, err
}

func loadQueueForRecovery(path string) (*Queue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultQueue(), nil
		}
		return nil, err
	}

	data = trimJSONPrefix(data)
	var queue Queue
	if err := json.Unmarshal(data, &queue); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
	}
	return &queue, nil
}

// ErrorCodeSchemaInvalid represents a schema validation error in the queue.
const ErrorCodeSchemaInvalid ErrorCode = "queue_schema_invalid"
