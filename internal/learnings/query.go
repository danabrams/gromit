package learnings

import (
	"strings"
	"time"
)

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
	return f.queryCounts()
}

// queryCounts returns the confirmed and provisional counts (nil-safe).
func (f *File) queryCounts() (int, int) {
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
