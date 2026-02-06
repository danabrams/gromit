package learnings

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

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
	path       string
	confirmed  []Learning
	provisional []Learning
	archived   []Learning
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

// NewFile creates a new learnings file manager
func NewFile(dir string) *File {
	return &File{
		path:        filepath.Join(dir, "LEARNINGS.md"),
		confirmed:   []Learning{},
		provisional: []Learning{},
		archived:    []Learning{},
	}
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
	f.normalizeNilFields()
	return nil
}

// Add adds a new learning, checking for duplicates
func (f *File) Add(beadID, content, category string) (*Learning, error) {
	if f == nil {
		return nil, fmt.Errorf("learnings file is nil")
	}
	hash := hashContent(content)

	// Check for exact duplicate
	for _, l := range f.confirmed {
		if l.Hash == hash {
			return nil, nil // Exact duplicate, skip
		}
	}
	for _, l := range f.provisional {
		if l.Hash == hash {
			return nil, nil // Exact duplicate, skip
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
	sb.WriteString("Accumulated operational knowledge from Ralph iterations.\n")
	sb.WriteString("This file is automatically updated. Review periodically with `ralph retro`.\n\n")

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

	// Write archived learnings
	sb.WriteString("---\n\n## Archived\n\n")
	sb.WriteString("*No longer relevant or superseded.*\n\n")
	if len(f.archived) == 0 {
		sb.WriteString("*No archived learnings.*\n\n")
	} else {
		for _, l := range f.archived {
			writeLearning(&sb, l)
		}
	}

	return os.WriteFile(f.path, []byte(sb.String()), 0644)
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

	// Add to archived
	f.archived = append(f.archived, *learning)

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

func hashContent(content string) string {
	// Normalize: lowercase, remove extra whitespace
	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")

	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:8]) // First 8 bytes is enough
}

// similarity calculates a simple trigram-based similarity score
func similarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))

	if a == b {
		return 1.0
	}

	trigramsA := trigrams(a)
	trigramsB := trigrams(b)

	if len(trigramsA) == 0 || len(trigramsB) == 0 {
		return 0.0
	}

	// Count matching trigrams
	matches := 0
	for t := range trigramsA {
		if trigramsB[t] {
			matches++
		}
	}

	// Jaccard similarity
	union := len(trigramsA) + len(trigramsB) - matches
	if union == 0 {
		return 0.0
	}
	return float64(matches) / float64(union)
}

func trigrams(s string) map[string]bool {
	result := make(map[string]bool)
	if len(s) < 3 {
		return result
	}
	for i := 0; i <= len(s)-3; i++ {
		result[s[i:i+3]] = true
	}
	return result
}

// parseLearnings parses the LEARNINGS.md file content
func parseLearnings(content string) (confirmed, provisional, archived []Learning) {
	// Simple parser - looks for ### headers in Confirmed/Provisional/Archived sections
	lines := strings.Split(content, "\n")

	inConfirmed := false
	inProvisional := false
	inArchived := false
	var current *Learning
	var contentLines []string

	saveCurrent := func() {
		if current != nil && len(contentLines) > 0 {
			current.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
			current.Hash = hashContent(current.Content)
			if inConfirmed {
				confirmed = append(confirmed, *current)
			} else if inProvisional {
				provisional = append(provisional, *current)
			} else if inArchived {
				archived = append(archived, *current)
			}
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## Confirmed") {
			saveCurrent()
			current = nil
			contentLines = nil
			inConfirmed = true
			inProvisional = false
			inArchived = false
			continue
		}
		if strings.HasPrefix(line, "## Provisional") {
			saveCurrent()
			current = nil
			contentLines = nil
			inConfirmed = false
			inProvisional = true
			inArchived = false
			continue
		}
		if strings.HasPrefix(line, "## Archived") {
			saveCurrent()
			current = nil
			contentLines = nil
			inConfirmed = false
			inProvisional = false
			inArchived = true
			continue
		}

		if strings.HasPrefix(line, "### ") {
			// Save previous learning
			saveCurrent()

			// Parse header: ### 2026-02-05 | bead-id | category
			parts := strings.Split(strings.TrimPrefix(line, "### "), " | ")
			current = &Learning{}
			contentLines = nil

			if len(parts) >= 1 {
				current.Date, _ = time.Parse("2006-01-02", strings.TrimSpace(parts[0]))
			}
			if len(parts) >= 2 {
				current.BeadID = strings.TrimSpace(parts[1])
			}
			if len(parts) >= 3 {
				current.Category = strings.TrimSpace(parts[2])
			}
			continue
		}

		if current != nil {
			// Check for related-to line
			if strings.HasPrefix(line, "*Related to:") {
				re := regexp.MustCompile(`\*Related to: (.+)\*`)
				if matches := re.FindStringSubmatch(line); len(matches) > 1 {
					current.RelatedTo = matches[1]
				}
				continue
			}
			// Skip section dividers
			if strings.TrimSpace(line) == "---" {
				continue
			}
			// Include everything else, including *Archived from* lines
			// (those are part of the content, not metadata like RelatedTo)
			contentLines = append(contentLines, line)
		}
	}

	// Don't forget the last learning
	saveCurrent()

	return confirmed, provisional, archived
}
