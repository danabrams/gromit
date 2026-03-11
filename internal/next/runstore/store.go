package runstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store manages run records and artifacts on disk.
type Store struct {
	rootDir string
}

// NewStore creates a new Store rooted at the given directory.
func NewStore(rootDir string) *Store {
	return &Store{rootDir: rootDir}
}

// Save writes a RunState to disk at runs/<run-id>/run.json.
func (s *Store) Save(rs *RunState) error {
	rs.NormalizeNilFields()
	dir := s.RunDir(rs.RunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}
	data, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run state: %w", err)
	}
	path := filepath.Join(dir, "run.json")
	return os.WriteFile(path, data, 0o644)
}

// Get reads a RunState from disk by run ID.
func (s *Store) Get(runID string) (*RunState, error) {
	path := filepath.Join(s.RunDir(runID), "run.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read run %s: %w", runID, err)
	}
	var rs RunState
	if err := json.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("unmarshal run %s: %w", runID, err)
	}
	rs.NormalizeNilFields()
	return &rs, nil
}

// RunDir returns the directory path for a given run ID.
func (s *Store) RunDir(runID string) string {
	return filepath.Join(s.rootDir, "runs", runID)
}

// TaskDir returns the directory path for a task within a run.
func (s *Store) TaskDir(runID, taskID string) string {
	return filepath.Join(s.RunDir(runID), "tasks", taskID)
}

// EvidenceDir returns the directory path for evidence within a task.
func (s *Store) EvidenceDir(runID, taskID string) string {
	return filepath.Join(s.TaskDir(runID, taskID), "evidence")
}
