package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCoverageTrackerFile_SaveAndLoadRestoresSnapshotUsingInjectedIO(t *testing.T) {
	tempDir := t.TempDir()
	saved := struct {
		path string
		data []byte
	}{}

	file := NewFile(tempDir,
		WithWriteFileFn(func(path string, data []byte, perm os.FileMode) error {
			saved.path = path
			saved.data = append([]byte(nil), data...)
			return nil
		}),
		WithReadFileFn(func(path string) ([]byte, error) {
			if len(saved.data) == 0 {
				return nil, os.ErrNotExist
			}
			if path != saved.path {
				return nil, fmt.Errorf("unexpected path: %s", path)
			}
			return append([]byte(nil), saved.data...), nil
		}),
	)

	tracker := NewTracker([]Criterion{{Number: 1, Text: "First"}}, 2)
	tracker.MarkCovered(1)
	snapshot := tracker.Snapshot()

	if err := file.Save(tracker); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if saved.path != filepath.Join(tempDir, coverageTrackerFileName) {
		t.Fatalf("saved path = %q, want %q", saved.path, filepath.Join(tempDir, coverageTrackerFileName))
	}

	if len(saved.data) == 0 {
		t.Fatal("expected Save() to write data")
	}

	tracker.RecordRejection(1)

	clone := NewTracker([]Criterion{{Number: 1, Text: "First"}}, 2)
	if _, err := file.Load(clone); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if clone.State() != snapshot.State {
		t.Fatalf("clone state = %v, want %v", clone.State(), snapshot.State)
	}

	if len(clone.CoveredCriteria()) != 1 || clone.CoveredCriteria()[0].Number != 1 {
		t.Fatalf("clone covered criteria = %v, want criterion 1", clone.CoveredCriteria())
	}

	if len(clone.UncoveredCriteria()) != 0 {
		t.Fatalf("clone uncovered criteria = %v, want none", clone.UncoveredCriteria())
	}
}
