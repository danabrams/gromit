package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// withFileLock acquires an exclusive advisory lock on a lock file adjacent to path,
// executes fn, then releases the lock. This prevents concurrent processes from
// reading/writing the state file simultaneously.
func withFileLock(path string, fn func() error) error {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring file lock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	return fn()
}

// InteractiveState represents persistent state for interactive commands stored in .gromit/interactive-state.json
type InteractiveState struct {
	LastRetro              time.Time `json:"last_retro,omitempty"`
	LastReviewCommit       string    `json:"last_review_commit,omitempty"`
	LastReviewIteration    int       `json:"last_review_iteration,omitempty"`
	FilteredLearningHashes []string  `json:"filtered_learning_hashes,omitempty"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// InteractiveFile manages the interactive-state.json file
type InteractiveFile struct {
	path  string
	state InteractiveState
}

// NewInteractiveFile creates a new interactive state file manager
func NewInteractiveFile(gromitDir string) (*InteractiveFile, error) {
	return &InteractiveFile{
		path: filepath.Join(gromitDir, "interactive-state.json"),
	}, nil
}

// Load reads the interactive state from disk under an advisory file lock.
func (f *InteractiveFile) Load() error {
	if f == nil {
		return fmt.Errorf("interactive state file is nil")
	}
	return withFileLock(f.path, func() error {
		data, err := os.ReadFile(f.path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // No state yet
			}
			return fmt.Errorf("reading interactive state file: %w", err)
		}

		if err := json.Unmarshal(data, &f.state); err != nil {
			return fmt.Errorf("parsing interactive state file: %w", err)
		}

		return nil
	})
}

// Save writes the interactive state to disk under an advisory file lock.
func (f *InteractiveFile) Save() error {
	if f == nil {
		return fmt.Errorf("interactive state file is nil")
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating interactive state directory: %w", err)
	}

	return withFileLock(f.path, func() error {
		return f.writeLocked()
	})
}

// LastRetro returns the time of the last retrospective
func (f *InteractiveFile) LastRetro() time.Time {
	if f == nil {
		return time.Time{}
	}
	return f.state.LastRetro
}

// RecordRetro updates the last retro time to now
func (f *InteractiveFile) RecordRetro() error {
	if f == nil {
		return fmt.Errorf("interactive state file is nil")
	}
	return f.mutateAndSaveLocked(func(s *InteractiveState) {
		s.LastRetro = time.Now()
	})
}

// LastReviewCommit returns the commit hash of the last review
func (f *InteractiveFile) LastReviewCommit() string {
	if f == nil {
		return ""
	}
	return f.state.LastReviewCommit
}

// LastReviewIteration returns the iteration number of the last review
func (f *InteractiveFile) LastReviewIteration() int {
	if f == nil {
		return 0
	}
	return f.state.LastReviewIteration
}

// RecordReview updates the last review commit and iteration, and saves
func (f *InteractiveFile) RecordReview(commit string, iteration int) error {
	if f == nil {
		return fmt.Errorf("interactive state file is nil")
	}
	return f.mutateAndSaveLocked(func(s *InteractiveState) {
		s.LastReviewCommit = commit
		s.LastReviewIteration = iteration
	})
}

// GetFilteredHashes returns a map of filtered learning hashes for O(1) lookups
func (f *InteractiveFile) GetFilteredHashes() map[string]bool {
	if f == nil {
		return nil
	}
	return sliceToSet(f.state.FilteredLearningHashes)
}

// AddFilteredHashes merges new hashes with existing ones, deduplicating
func (f *InteractiveFile) AddFilteredHashes(hashes []string) {
	if f == nil {
		return
	}
	f.state.FilteredLearningHashes = mergeHashes(f.state.FilteredLearningHashes, hashes)
}

// NormalizeNilFields converts nil slices to empty slices
func (s *InteractiveState) NormalizeNilFields() {
	if s.FilteredLearningHashes == nil {
		s.FilteredLearningHashes = []string{}
	}
}

func (f *InteractiveFile) mutateAndSaveLocked(mutateFn func(*InteractiveState)) error {
	if f == nil {
		return fmt.Errorf("interactive state file is nil")
	}

	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating interactive state directory: %w", err)
	}

	return withFileLock(f.path, func() error {
		if err := f.loadLocked(); err != nil {
			return err
		}
		mutateFn(&f.state)
		return f.writeLocked()
	})
}

func (f *InteractiveFile) loadLocked() error {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading interactive state file: %w", err)
	}

	if err := json.Unmarshal(data, &f.state); err != nil {
		return fmt.Errorf("parsing interactive state file: %w", err)
	}
	return nil
}

func (f *InteractiveFile) writeLocked() error {
	f.state.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(f.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling interactive state: %w", err)
	}
	return os.WriteFile(f.path, data, 0644)
}
