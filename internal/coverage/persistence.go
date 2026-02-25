package coverage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const coverageTrackerFileName = "coverage-tracker.json"

// ReadFileFn abstracts reading file bytes at a given path.
type ReadFileFn func(path string) ([]byte, error)

// WriteFileFn abstracts writing bytes to a given path.
type WriteFileFn func(path string, data []byte, perm os.FileMode) error

// FileOption configures a coverage persistence file.
type FileOption func(*File)

// WithReadFileFn overrides the read function used by the persistence file.
func WithReadFileFn(fn ReadFileFn) FileOption {
	return func(f *File) {
		if fn != nil {
			f.readFile = fn
		}
	}
}

// WithWriteFileFn overrides the write function used by the persistence file.
func WithWriteFileFn(fn WriteFileFn) FileOption {
	return func(f *File) {
		if fn != nil {
			f.writeFile = fn
		}
	}
}

// File manages coverage tracker persistence on disk.
type File struct {
	path      string
	readFile  ReadFileFn
	writeFile WriteFileFn
}

// NewFile creates a persistence file backed by gromitDir/coverage-tracker.json.
func NewFile(gromitDir string, opts ...FileOption) *File {
	f := &File{
		path:      filepath.Join(gromitDir, coverageTrackerFileName),
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Load reads the persisted tracker state and applies it if tracker is non-nil.
func (f *File) Load(tracker *CoverageTracker) (*Snapshot, error) {
	if f == nil {
		return nil, fmt.Errorf("coverage file is nil")
	}

	data, err := f.readFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading coverage tracker file: %w", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("parsing coverage tracker snapshot: %w", err)
	}
	if tracker != nil {
		tracker.ApplySnapshot(snapshot)
	}
	return &snapshot, nil
}

// Save persists the provided tracker snapshot using the configured writer.
func (f *File) Save(tracker *CoverageTracker) error {
	if f == nil {
		return fmt.Errorf("coverage file is nil")
	}
	if tracker == nil {
		return fmt.Errorf("coverage tracker is nil")
	}

	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("creating coverage directory: %w", err)
	}

	data, err := json.MarshalIndent(tracker.Snapshot(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling coverage tracker snapshot: %w", err)
	}

	if err := f.writeFile(f.path, data, 0o644); err != nil {
		return fmt.Errorf("writing coverage tracker file: %w", err)
	}
	return nil
}
