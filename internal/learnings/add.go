package learnings

import (
	"fmt"
	"os"
)

func (f *File) processNewLearning(learning Learning) (*Learning, error) {
	if f == nil {
		return nil, fmt.Errorf("learnings file is nil")
	}

	// Apply filter if configured
	if f.filterFunc != nil {
		isGeneric, err := f.filterFunc(learning.Content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "learnings filter failed: %v\n", err)
			// Fall through to normal logic on filter error — don't block learning placement
		} else if isGeneric {
			archived := learning
			archived.Content = fmt.Sprintf("%s\n\n*Archived from new: filtered: generic engineering advice*", learning.Content)
			if err := f.appendToArchiveFile(archived); err != nil {
				return nil, err
			}
			f.trackArchivedLearning(archived)
			return nil, f.Save()
		}
	}

	// Check for fuzzy match in provisional (might promote to confirmed)
	for i, existing := range f.provisional {
		if similarity(existing.Content, learning.Content) > FuzzyMatchThreshold {
			// Similar learning exists - promote to confirmed
			f.provisional = append(f.provisional[:i], f.provisional[i+1:]...)
			learning.RelatedTo = existing.BeadID
			f.confirmed = append(f.confirmed, learning)
			return &learning, f.Save()
		}
	}

	// Check for fuzzy match in confirmed (mark as related)
	for _, existing := range f.confirmed {
		if similarity(existing.Content, learning.Content) > FuzzyMatchThreshold {
			learning.RelatedTo = existing.BeadID
			break
		}
	}

	// Add as provisional
	f.provisional = append(f.provisional, learning)
	return &learning, f.Save()
}
