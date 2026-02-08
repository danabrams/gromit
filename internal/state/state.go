package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State represents persistent state stored in .gromit/state.json
type State struct {
	LastRetro              time.Time `json:"last_retro,omitempty"`
	LastReviewCommit       string    `json:"last_review_commit,omitempty"`
	LastReviewIteration    int       `json:"last_review_iteration,omitempty"`
	IterationsSinceReview  int       `json:"iterations_since_review,omitempty"`
	CleanExit              bool      `json:"clean_exit"`
	UpdatedAt              time.Time `json:"updated_at"`
	FilteredLearningHashes []string  `json:"filtered_learning_hashes,omitempty"`
}

// File manages the state.json file
type File struct {
	path  string
	state State
}

// NewFile creates a new state file manager
func NewFile(gromitDir string) (*File, error) {
	return &File{
		path: filepath.Join(gromitDir, "state.json"),
	}, nil
}

// Load reads the state from disk
func (f *File) Load() error {
	if f == nil {
		return fmt.Errorf("state file is nil")
	}
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state yet
		}
		return fmt.Errorf("reading state file: %w", err)
	}

	if err := json.Unmarshal(data, &f.state); err != nil {
		return fmt.Errorf("parsing state file: %w", err)
	}

	return nil
}

// Save writes the state to disk
func (f *File) Save() error {
	if f == nil {
		return fmt.Errorf("state file is nil")
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	// Auto-stamp UpdatedAt before marshalling
	f.state.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(f.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}

	return os.WriteFile(f.path, data, 0644)
}

// LastRetro returns the time of the last retrospective
func (f *File) LastRetro() time.Time {
	if f == nil {
		return time.Time{}
	}
	return f.state.LastRetro
}

// RecordRetro updates the last retro time to now
func (f *File) RecordRetro() error {
	if f == nil {
		return fmt.Errorf("state file is nil")
	}
	f.state.LastRetro = time.Now()
	return f.Save()
}

// LastReviewCommit returns the commit hash of the last review
func (f *File) LastReviewCommit() string {
	if f == nil {
		return ""
	}
	return f.state.LastReviewCommit
}

// LastReviewIteration returns the iteration number of the last review
func (f *File) LastReviewIteration() int {
	if f == nil {
		return 0
	}
	return f.state.LastReviewIteration
}

// IterationsSinceReview returns the number of iterations since the last review
func (f *File) IterationsSinceReview() int {
	if f == nil {
		return 0
	}
	return f.state.IterationsSinceReview
}

// IncrementIterationsSinceReview increments the counter of iterations since last review
func (f *File) IncrementIterationsSinceReview() {
	if f == nil {
		return
	}
	f.state.IterationsSinceReview++
}

// RecordReview updates the last review commit and iteration, resets counter to 0, and saves
func (f *File) RecordReview(commit string, iteration int) error {
	if f == nil {
		return fmt.Errorf("state file is nil")
	}
	f.state.LastReviewCommit = commit
	f.state.LastReviewIteration = iteration
	f.state.IterationsSinceReview = 0
	return f.Save()
}

// SetCleanExit sets the clean exit flag
func (f *File) SetCleanExit(cleanExit bool) {
	if f == nil {
		return
	}
	f.state.CleanExit = cleanExit
}

// CheckStaleness detects whether state is stale due to crash or old timestamp.
// Returns (isStale, reason) where reason explains what triggered staleness detection.
func (f *File) CheckStaleness(thresholdMinutes int) (bool, string) {
	if f == nil {
		return false, ""
	}

	// Primary signal: clean exit flag
	if !f.state.CleanExit {
		return true, "previous run did not exit cleanly (crash detected)"
	}

	// Secondary signal: timestamp age
	if !f.state.UpdatedAt.IsZero() {
		age := time.Since(f.state.UpdatedAt)
		threshold := time.Duration(thresholdMinutes) * time.Minute
		if age > threshold {
			return true, fmt.Sprintf("state file is stale (last updated %v ago, threshold is %v)", age.Round(time.Second), threshold)
		}
	}

	return false, ""
}

// AutoHeal resets unreliable counter fields while preserving git anchors and historical timestamps.
// This should be called when staleness is detected via CheckStaleness.
func (f *File) AutoHeal() {
	if f == nil {
		return
	}
	f.state.IterationsSinceReview = 0
	f.state.LastReviewIteration = 0
	// Preserve: LastReviewCommit, LastRetro
}

// GetFilteredHashes returns a map of filtered learning hashes for O(1) lookups
func (f *File) GetFilteredHashes() map[string]bool {
	if f == nil {
		return nil
	}
	result := make(map[string]bool, len(f.state.FilteredLearningHashes))
	for _, hash := range f.state.FilteredLearningHashes {
		result[hash] = true
	}
	return result
}

// AddFilteredHashes merges new hashes with existing ones, deduplicating
func (f *File) AddFilteredHashes(hashes []string) {
	if f == nil {
		return
	}
	// Create a set of existing hashes for O(1) lookups
	existing := make(map[string]bool, len(f.state.FilteredLearningHashes))
	for _, hash := range f.state.FilteredLearningHashes {
		existing[hash] = true
	}
	// Add new hashes that don't already exist
	for _, hash := range hashes {
		if !existing[hash] {
			f.state.FilteredLearningHashes = append(f.state.FilteredLearningHashes, hash)
			existing[hash] = true
		}
	}
}

// ReconcileFilteredHashes filters FilteredLearningHashes in-place, keeping only hashes present in currentHashes.
// Returns true if any hashes were pruned.
func (f *File) ReconcileFilteredHashes(currentHashes map[string]bool) bool {
	if f == nil {
		return false
	}

	// Filter in-place, keeping only hashes that are in currentHashes
	kept := f.state.FilteredLearningHashes[:0] // reuse backing array
	for _, hash := range f.state.FilteredLearningHashes {
		if currentHashes[hash] {
			kept = append(kept, hash)
		}
	}

	// Check if anything was removed
	pruned := len(kept) < len(f.state.FilteredLearningHashes)
	f.state.FilteredLearningHashes = kept

	return pruned
}
