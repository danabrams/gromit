package learnings

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	// Regex for normalizing whitespace in content hashing
	whitespaceRegex = regexp.MustCompile(`\s+`)
	// Regex for parsing *Related to:* lines
	relatedToRegex = regexp.MustCompile(`\*Related to: (.+)\*`)
)

// FilterFunc determines if a learning is generic engineering advice or project-specific.
// Returns true if the learning is generic (should be filtered), false if project-specific.
type FilterFunc func(content string) (isGeneric bool, err error)

// Learning represents a single learning entry
type Learning struct {
	Date      time.Time
	BeadID    string
	Content   string
	Category  string // "conventions", "gotchas", "patterns"
	Hash      string // SHA256 of content for dedup
	RelatedTo string // ID of similar learning (if fuzzy matched)
}

// File manages the LEARNINGS.md file
type File struct {
	path           string
	archivePath    string
	confirmed      []Learning
	provisional    []Learning
	archived       []Learning
	archivedHashes map[string]bool
	filterFunc     FilterFunc
	autoSaveOff    bool
}

// Category constants
const (
	CategoryConventions = "conventions"
	CategoryGotchas     = "gotchas"
	CategoryPatterns    = "patterns"
)

// FuzzyMatchThreshold is the similarity threshold for promoting provisional learnings
// to confirmed status when a fuzzy match is detected. Learnings are fuzzy matched
// using trigram-based Jaccard similarity. A score above this threshold indicates
// the new learning is similar enough to an existing one to be considered a duplicate
// pattern (promoting from provisional to confirmed).
const FuzzyMatchThreshold = 0.7

const archiveFileHeader = "# Learnings Archive\n\n---\n\n## Archived\n\n"

// NewFile creates a new learnings file manager
func NewFile(dir string) (*File, error) {
	return &File{
		path:           filepath.Join(dir, "LEARNINGS.md"),
		archivePath:    filepath.Join(dir, "LEARNINGS_ARCHIVE.md"),
		confirmed:      []Learning{},
		provisional:    []Learning{},
		archived:       []Learning{},
		archivedHashes: map[string]bool{},
	}, nil
}

// SetAutoSave enables or disables automatic saving after mutations.
func (f *File) SetAutoSave(enabled bool) {
	if f == nil {
		return
	}
	f.autoSaveOff = !enabled
}

// SetFilter sets the filter function used to classify learnings as generic or project-specific.
func (f *File) SetFilter(fn FilterFunc) {
	if f == nil {
		return
	}
	f.filterFunc = fn
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
func (f *File) normalizeNilFields() {
	if f == nil {
		return
	}
	if f.confirmed == nil {
		f.confirmed = []Learning{}
	}
	if f.provisional == nil {
		f.provisional = []Learning{}
	}
	if f.archived == nil {
		f.archived = []Learning{}
	}
	if f.archivedHashes == nil {
		f.archivedHashes = map[string]bool{}
	}
}

// SetArchivedHashes replaces the archived hash set.
func (f *File) SetArchivedHashes(hashes []string) {
	if f == nil {
		return
	}
	next := make(map[string]bool, len(hashes))
	for _, hash := range hashes {
		next[hash] = true
	}
	f.archivedHashes = next
}

// GetArchivedHashes returns a copy of archived hash set.
func (f *File) GetArchivedHashes() map[string]bool {
	if f == nil {
		return map[string]bool{}
	}
	result := make(map[string]bool, len(f.archivedHashes))
	for hash := range f.archivedHashes {
		result[hash] = true
	}
	return result
}

func (f *File) appendToArchiveFile(learning Learning) error {
	if f == nil {
		return fmt.Errorf("learnings file is nil")
	}
	if err := os.MkdirAll(filepath.Dir(f.archivePath), 0755); err != nil {
		return fmt.Errorf("creating archive directory: %w", err)
	}

	needsHeader := false
	info, err := os.Stat(f.archivePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat archive file: %w", err)
		}
		needsHeader = true
	} else if info.Size() == 0 {
		needsHeader = true
	}

	file, err := os.OpenFile(f.archivePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening archive file: %w", err)
	}
	defer file.Close()

	if needsHeader {
		if _, err := file.WriteString(archiveFileHeader); err != nil {
			return fmt.Errorf("writing archive header: %w", err)
		}
	}

	var sb strings.Builder
	writeLearning(&sb, learning)
	if _, err := file.WriteString(sb.String()); err != nil {
		return fmt.Errorf("writing archive file: %w", err)
	}
	return nil
}

func (f *File) trackArchivedLearning(learning Learning) {
	f.archived = append(f.archived, learning)
	f.archivedHashes[learning.Hash] = true
}

// hashExists checks if a hash already exists in any section (confirmed, provisional, or archived)
func (f *File) hashExists(hash string) bool {
	for _, l := range f.confirmed {
		if l.Hash == hash {
			return true
		}
	}
	for _, l := range f.provisional {
		if l.Hash == hash {
			return true
		}
	}
	return f.archivedHashes[hash]
}

// Load reads and parses the LEARNINGS.md file
func (f *File) Load() error {
	if f == nil {
		return fmt.Errorf("learnings file is nil")
	}
	content, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - that's fine
			return nil
		}
		return fmt.Errorf("reading learnings file: %w", err)
	}

	f.confirmed, f.provisional, f.archived = parseLearnings(string(content))
	archiveContent, err := os.ReadFile(f.archivePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading archive file: %w", err)
		}
	} else {
		_, _, archivedFromFile := parseLearnings(string(archiveContent))
		f.archived = append(f.archived, archivedFromFile...)
	}
	f.normalizeNilFields()
	for _, learning := range f.archived {
		f.archivedHashes[learning.Hash] = true
	}
	return nil
}

// Add adds a new learning, checking for duplicates
func (f *File) Add(beadID, content, category string) (*Learning, error) {
	if f == nil {
		return nil, fmt.Errorf("learnings file is nil")
	}
	hash := hashContent(content)

	// Check for exact duplicate in any section
	if f.hashExists(hash) {
		return nil, nil // Exact duplicate, skip
	}

	// Apply filter if configured
	if f.filterFunc != nil {
		isGeneric, err := f.filterFunc(content)
		if err != nil {
			// Fall through to normal logic on filter error — don't block learning placement
		} else if isGeneric {
			// Archive as generic engineering advice
			learning := Learning{
				Date:     time.Now(),
				BeadID:   beadID,
				Content:  fmt.Sprintf("%s\n\n*Archived from new: filtered: generic engineering advice*", content),
				Category: category,
				Hash:     hash,
			}
			if err := f.appendToArchiveFile(learning); err != nil {
				return nil, err
			}
			f.trackArchivedLearning(learning)
			return nil, f.Save()
		}
	}

	learning := Learning{
		Date:     time.Now(),
		BeadID:   beadID,
		Content:  content,
		Category: category,
		Hash:     hash,
	}

	// Check for fuzzy match in provisional (might promote to confirmed)
	for i, existing := range f.provisional {
		if similarity(existing.Content, content) > FuzzyMatchThreshold {
			// Similar learning exists - promote to confirmed
			f.provisional = append(f.provisional[:i], f.provisional[i+1:]...)
			learning.RelatedTo = existing.BeadID
			f.confirmed = append(f.confirmed, learning)
			return &learning, f.Save()
		}
	}

	// Check for fuzzy match in confirmed (mark as related)
	for _, existing := range f.confirmed {
		if similarity(existing.Content, content) > FuzzyMatchThreshold {
			learning.RelatedTo = existing.BeadID
			break
		}
	}

	// Add as provisional
	f.provisional = append(f.provisional, learning)
	return &learning, f.Save()
}

// Save writes the learnings back to the file
func (f *File) Save() error {
	if f == nil {
		return fmt.Errorf("learnings file is nil")
	}
	// Ensure directory exists
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Learnings\n\n")
	sb.WriteString("Accumulated operational knowledge from Gromit iterations.\n")
	sb.WriteString("This file is automatically updated. Review periodically with `gromit retro`.\n\n")

	// Write confirmed learnings
	sb.WriteString("---\n\n## Confirmed\n\n")
	sb.WriteString("*Patterns seen multiple times - high confidence.*\n\n")
	if len(f.confirmed) == 0 {
		sb.WriteString("*No confirmed learnings yet.*\n\n")
	} else {
		for _, l := range f.confirmed {
			writeLearning(&sb, l)
		}
	}

	// Write provisional learnings
	sb.WriteString("---\n\n## Provisional\n\n")
	sb.WriteString("*Seen once - may be specific to one task.*\n\n")
	if len(f.provisional) == 0 {
		sb.WriteString("*No provisional learnings.*\n\n")
	} else {
		for _, l := range f.provisional {
			writeLearning(&sb, l)
		}
	}

	return os.WriteFile(f.path, []byte(sb.String()), 0644)
}

// FilterOptions controls filtering for GetConfirmedFiltered.
type FilterOptions struct {
	MaxChars int      // Character budget; 0 means unlimited
	Keywords []string // Case-insensitive substring filters (OR-matched)
}

// GetConfirmedFiltered returns confirmed learnings filtered by keywords and capped
// by a character budget. Keywords are OR-matched (case-insensitive substring).
// When MaxChars > 0, entries are selected from most recent to oldest until the
// budget is exceeded. Returns a non-nil empty slice when no entries match.
func (f *File) GetConfirmedFiltered(opts FilterOptions) []Learning {
	if f == nil {
		return []Learning{}
	}

	// Step 1: filter by keywords if provided
	candidates := f.confirmed
	if len(opts.Keywords) > 0 {
		filtered := []Learning{}
		for _, l := range candidates {
			lower := strings.ToLower(l.Content)
			for _, kw := range opts.Keywords {
				if strings.Contains(lower, strings.ToLower(kw)) {
					filtered = append(filtered, l)
					break
				}
			}
		}
		candidates = filtered
	}

	// Step 2: apply character budget, preferring most recent entries
	if opts.MaxChars > 0 {
		budgeted := []Learning{}
		remaining := opts.MaxChars
		// Iterate from most recent (end) to oldest (start)
		for i := len(candidates) - 1; i >= 0; i-- {
			charLen := len(candidates[i].Content)
			if charLen <= remaining {
				budgeted = append(budgeted, candidates[i])
				remaining -= charLen
			}
		}
		return budgeted
	}

	if len(candidates) == 0 {
		return []Learning{}
	}
	return candidates
}

// GetConfirmed returns all confirmed learnings
func (f *File) GetConfirmed() []Learning {
	if f == nil {
		return []Learning{}
	}
	return f.confirmed
}

// GetProvisional returns all provisional learnings
func (f *File) GetProvisional() []Learning {
	if f == nil {
		return []Learning{}
	}
	return f.provisional
}

// GetRecent returns provisional learnings from the last N iterations
// Since we don't track iteration numbers, we use time-based (last N hours)
func (f *File) GetRecent(hours int) []Learning {
	if f == nil {
		return []Learning{}
	}
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	recent := []Learning{}
	for _, l := range f.provisional {
		if l.Date.After(cutoff) {
			recent = append(recent, l)
		}
	}
	return recent
}

// Stats returns statistics about learnings
func (f *File) Stats() (confirmed, provisional int) {
	if f == nil {
		return 0, 0
	}
	return len(f.confirmed), len(f.provisional)
}

// GetByHash returns a learning by its hash, searching all sections
func (f *File) GetByHash(hash string) *Learning {
	if f == nil {
		return nil
	}
	// Search confirmed
	for i := range f.confirmed {
		if f.confirmed[i].Hash == hash {
			return &f.confirmed[i]
		}
	}
	// Search provisional
	for i := range f.provisional {
		if f.provisional[i].Hash == hash {
			return &f.provisional[i]
		}
	}
	// Search archived
	for i := range f.archived {
		if f.archived[i].Hash == hash {
			return &f.archived[i]
		}
	}
	return nil
}

// Remove removes a learning by hash from all sections
func (f *File) Remove(hash string) error {
	if f == nil {
		return fmt.Errorf("learnings file is nil")
	}

	removed := false

	// Remove from confirmed
	for i := range f.confirmed {
		if f.confirmed[i].Hash == hash {
			f.confirmed = append(f.confirmed[:i], f.confirmed[i+1:]...)
			removed = true
			break
		}
	}

	// Remove from provisional if not found in confirmed
	if !removed {
		for i := range f.provisional {
			if f.provisional[i].Hash == hash {
				f.provisional = append(f.provisional[:i], f.provisional[i+1:]...)
				removed = true
				break
			}
		}
	}

	// Remove from archived if not found elsewhere
	if !removed {
		for i := range f.archived {
			if f.archived[i].Hash == hash {
				f.archived = append(f.archived[:i], f.archived[i+1:]...)
				removed = true
				break
			}
		}
	}

	if !removed {
		return fmt.Errorf("learning with hash %s not found", hash)
	}

	return f.Save()
}

// Archive moves a learning to the archived section with an optional reason
func (f *File) Archive(hash, reason string) error {
	if f == nil {
		return fmt.Errorf("learnings file is nil")
	}

	var learning *Learning
	var fromSection string

	// Find and remove from confirmed
	for i := range f.confirmed {
		if f.confirmed[i].Hash == hash {
			learning = &f.confirmed[i]
			f.confirmed = append(f.confirmed[:i], f.confirmed[i+1:]...)
			fromSection = "confirmed"
			break
		}
	}

	// Find and remove from provisional if not in confirmed
	if learning == nil {
		for i := range f.provisional {
			if f.provisional[i].Hash == hash {
				learning = &f.provisional[i]
				f.provisional = append(f.provisional[:i], f.provisional[i+1:]...)
				fromSection = "provisional"
				break
			}
		}
	}

	if learning == nil {
		return fmt.Errorf("learning with hash %s not found", hash)
	}

	// Add reason to content if provided
	if reason != "" {
		learning.Content = fmt.Sprintf("%s\n\n*Archived from %s: %s*", learning.Content, fromSection, reason)
	} else {
		learning.Content = fmt.Sprintf("%s\n\n*Archived from %s*", learning.Content, fromSection)
	}

	if err := f.appendToArchiveFile(*learning); err != nil {
		return err
	}
	f.trackArchivedLearning(*learning)

	if f.autoSaveOff {
		return nil
	}
	return f.Save()
}

// Replace replaces one or more old learnings with a new consolidated learning
func (f *File) Replace(oldHashes []string, newContent, category string) error {
	if f == nil {
		return fmt.Errorf("learnings file is nil")
	}
	if len(oldHashes) == 0 {
		return fmt.Errorf("no old hashes provided")
	}

	// Collect old learning IDs for reference
	var oldBeadIDs []string
	for _, hash := range oldHashes {
		learning := f.GetByHash(hash)
		if learning != nil {
			oldBeadIDs = append(oldBeadIDs, learning.BeadID)
		}
	}

	// Remove all old learnings
	for _, hash := range oldHashes {
		// Try to remove, but don't fail if not found
		// (some might already be removed by previous iterations)
		for i := range f.confirmed {
			if f.confirmed[i].Hash == hash {
				f.confirmed = append(f.confirmed[:i], f.confirmed[i+1:]...)
				break
			}
		}
		for i := range f.provisional {
			if f.provisional[i].Hash == hash {
				f.provisional = append(f.provisional[:i], f.provisional[i+1:]...)
				break
			}
		}
		for i := range f.archived {
			if f.archived[i].Hash == hash {
				f.archived = append(f.archived[:i], f.archived[i+1:]...)
				break
			}
		}
	}

	// Create new learning
	hash := hashContent(newContent)
	newLearning := Learning{
		Date:     time.Now(),
		BeadID:   "retro", // Special bead ID for retro-created learnings
		Content:  newContent,
		Category: category,
		Hash:     hash,
	}

	// Add reference to replaced learnings
	if len(oldBeadIDs) > 0 {
		newLearning.RelatedTo = strings.Join(oldBeadIDs, ", ")
	}

	// Add to confirmed (replacement learnings are confirmed by default)
	f.confirmed = append(f.confirmed, newLearning)

	if f.autoSaveOff {
		return nil
	}
	return f.Save()
}

// ShouldSuggestRetro returns true if conditions suggest running a retro
func (f *File) ShouldSuggestRetro(lastRetro time.Time, failureRate float64) (bool, string) {
	if f == nil {
		return false, ""
	}
	confirmedCount, provisionalCount := f.Stats()

	if provisionalCount > 10 {
		return true, fmt.Sprintf("%d provisional learnings accumulated", provisionalCount)
	}

	if time.Since(lastRetro) > 7*24*time.Hour {
		return true, "more than 7 days since last retro"
	}

	if failureRate > 0.3 {
		return true, fmt.Sprintf("failure rate is %.0f%%", failureRate*100)
	}

	if confirmedCount+provisionalCount > 20 {
		return true, "learnings file is getting large"
	}

	return false, ""
}

func writeLearning(sb *strings.Builder, l Learning) {
	sb.WriteString(fmt.Sprintf("### %s | %s | %s\n",
		l.Date.Format("2006-01-02"),
		l.BeadID,
		l.Category,
	))
	if l.RelatedTo != "" {
		sb.WriteString(fmt.Sprintf("*Related to: %s*\n\n", l.RelatedTo))
	}
	sb.WriteString(l.Content)
	sb.WriteString("\n\n")
}
