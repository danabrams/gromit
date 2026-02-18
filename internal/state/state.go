package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State represents persistent state stored in .gromit/state.json
//
// Review state (LastReviewCommit, LastReviewIteration) lives exclusively in
// InteractiveState / interactive-state.json.  Do NOT add review fields here;
// the compiler will catch callers that try to use state.File for review state.
type State struct {
	LastRetro                time.Time            `json:"last_retro,omitempty"`
	IterationsSinceReview    int                  `json:"iterations_since_review,omitempty"`
	CleanExit                bool                 `json:"clean_exit"`
	UpdatedAt                time.Time            `json:"updated_at"`
	FilteredLearningHashes   []string             `json:"filtered_learning_hashes,omitempty"`
	ArchivedLearningHashes   []string             `json:"archived_learning_hashes,omitempty"`
	ProviderCounts           map[string]int       `json:"provider_counts,omitempty"`
	ProviderUnavailableUntil map[string]time.Time `json:"provider_unavailable_until,omitempty"`
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
	f.state.NormalizeNilFields()

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

// ResetIterationsSinceReview resets the iterations-since-review counter to 0 and saves.
func (f *File) ResetIterationsSinceReview() error {
	if f == nil {
		return fmt.Errorf("state file is nil")
	}
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

// AutoHeal resets unreliable counter fields while preserving historical timestamps.
// This should be called when staleness is detected via CheckStaleness.
func (f *File) AutoHeal() {
	if f == nil {
		return
	}
	f.state.IterationsSinceReview = 0
	// Preserve: LastRetro
}

// GetFilteredHashes returns a map of filtered learning hashes for O(1) lookups
func (f *File) GetFilteredHashes() map[string]bool {
	if f == nil {
		return nil
	}
	return sliceToSet(f.state.FilteredLearningHashes)
}

// AddFilteredHashes merges new hashes with existing ones, deduplicating
func (f *File) AddFilteredHashes(hashes []string) {
	if f == nil {
		return
	}
	f.state.FilteredLearningHashes = mergeHashes(f.state.FilteredLearningHashes, hashes)
}

// GetArchivedHashes returns a map of archived learning hashes for O(1) lookups
func (f *File) GetArchivedHashes() map[string]bool {
	if f == nil {
		return nil
	}
	return sliceToSet(f.state.ArchivedLearningHashes)
}

// AddArchivedHashes merges new hashes with existing ones, deduplicating
func (f *File) AddArchivedHashes(hashes []string) {
	if f == nil {
		return
	}
	f.state.ArchivedLearningHashes = mergeHashes(f.state.ArchivedLearningHashes, hashes)
}

// sliceToSet converts a string slice to a set (map[string]bool) for O(1) lookups
func sliceToSet(slice []string) map[string]bool {
	result := make(map[string]bool, len(slice))
	for _, item := range slice {
		result[item] = true
	}
	return result
}

// mergeHashes merges new hashes with existing ones, deduplicating
func mergeHashes(existing, new []string) []string {
	// Create a set of existing hashes for O(1) lookups
	seen := sliceToSet(existing)
	// Add new hashes that don't already exist
	for _, hash := range new {
		if !seen[hash] {
			existing = append(existing, hash)
			seen[hash] = true
		}
	}
	return existing
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

// NormalizeNilFields converts nil slices and maps to empty slices/maps
func (s *State) NormalizeNilFields() {
	if s.FilteredLearningHashes == nil {
		s.FilteredLearningHashes = []string{}
	}
	if s.ArchivedLearningHashes == nil {
		s.ArchivedLearningHashes = []string{}
	}
	if s.ProviderCounts == nil {
		s.ProviderCounts = make(map[string]int)
	}
	if s.ProviderUnavailableUntil == nil {
		s.ProviderUnavailableUntil = make(map[string]time.Time)
	}
}

// IncrementProviderCount increments the invocation count for a provider
func (f *File) IncrementProviderCount(provider string) {
	if f == nil {
		return
	}
	f.ensureProviderMaps()
	f.state.ProviderCounts[provider]++
}

// GetProviderCounts returns a copy of the provider invocation counts
func (f *File) GetProviderCounts() map[string]int {
	if f == nil {
		return nil
	}
	if f.state.ProviderCounts == nil {
		return make(map[string]int)
	}
	// Return copy to prevent external mutations
	result := make(map[string]int, len(f.state.ProviderCounts))
	for k, v := range f.state.ProviderCounts {
		result[k] = v
	}
	return result
}

// ResetProviderCounts resets all provider invocation counts to zero
func (f *File) ResetProviderCounts() {
	if f == nil {
		return
	}
	f.state.ProviderCounts = make(map[string]int)
}

// SetProviderUnavailable marks a provider as unavailable until the specified time
func (f *File) SetProviderUnavailable(provider string, until time.Time) {
	if f == nil {
		return
	}
	f.ensureProviderMaps()
	f.state.ProviderUnavailableUntil[provider] = until
}

// IsProviderAvailable returns true if the provider is available (not in cooldown period)
func (f *File) IsProviderAvailable(provider string) bool {
	if f == nil {
		return true // nil-safe default: available
	}
	if f.state.ProviderUnavailableUntil == nil {
		return true
	}
	until, exists := f.state.ProviderUnavailableUntil[provider]
	if !exists {
		return true
	}
	// Available if the cooldown time has passed
	return time.Now().After(until)
}

// ClearProviderUnavailable removes the unavailable status for a provider
func (f *File) ClearProviderUnavailable(provider string) {
	if f == nil {
		return
	}
	f.ensureProviderMaps()
	delete(f.state.ProviderUnavailableUntil, provider)
}

// ensureProviderMaps initializes provider-related maps if they are nil
func (f *File) ensureProviderMaps() {
	if f.state.ProviderCounts == nil {
		f.state.ProviderCounts = make(map[string]int)
	}
	if f.state.ProviderUnavailableUntil == nil {
		f.state.ProviderUnavailableUntil = make(map[string]time.Time)
	}
}
