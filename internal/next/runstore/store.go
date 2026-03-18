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

// List returns all runs matching the given project ID.
func (s *Store) List(projectID string) ([]*RunState, error) {
	runsDir := filepath.Join(s.rootDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*RunState{}, nil
		}
		return nil, fmt.Errorf("list runs: %w", err)
	}
	var result []*RunState
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rs, err := s.Get(e.Name())
		if err != nil {
			continue
		}
		if rs.ProjectID == projectID {
			result = append(result, rs)
		}
	}
	if result == nil {
		result = []*RunState{}
	}
	return result, nil
}

// ResetForNewCycle resets per-cycle gate fields on rs.
// Fields that persist across replan cycles (e.g. ContractsWritten, ReplanContext,
// AccumulatedCost, TotalReplans) are intentionally NOT reset here.
// ScenarioTestsWritten, FailureHistory, and TaskLineage are NOT reset — they persist across cycles.
func ResetForNewCycle(rs *RunState) {
	rs.FinalValidationPassed = false
	rs.FinalReviewPassed = false
	rs.FinalAcceptancePassed = false
	rs.ReviewFindings = []string{}
	rs.AcceptanceResults = []string{}
	// ContractsWritten, ScenarioTestsWritten, FailureHistory, and TaskLineage are NOT reset — they persist across cycles.
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

// RunEvidenceDir returns the directory path for run-level evidence.
func (s *Store) RunEvidenceDir(runID string) string {
	return filepath.Join(s.RunDir(runID), "evidence")
}

// WriteTaskArtifact marshals v to JSON and writes it to tasks/<taskID>/<filename>.
func (s *Store) WriteTaskArtifact(runID, taskID, filename string, v any) error {
	dir := s.TaskDir(runID, taskID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create task dir: %w", err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, filename), data, 0o644)
}

// ReadTaskArtifact reads a JSON artifact from tasks/<taskID>/<filename> into v.
func (s *Store) ReadTaskArtifact(runID, taskID, filename string, v any) error {
	path := filepath.Join(s.TaskDir(runID, taskID), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read artifact %s: %w", filename, err)
	}
	return json.Unmarshal(data, v)
}
