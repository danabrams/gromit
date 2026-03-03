package validation

// StaleFixSnapshot captures metadata for a validation retry attempt.
type StaleFixSnapshot struct {
	ChangedFiles   []string
	ErrorCategories []string
}

// StaleFixDetection reports whether the current attempt repeats the previous one.
type StaleFixDetection struct {
	// StaleFixDetected is true when neither the changed files nor error categories
	// differ from the previous attempt.
	StaleFixDetected bool

	// ChangedFilesMatch indicates whether the file set matches the previous attempt.
	ChangedFilesMatch bool

	// ErrorCategoriesMatch indicates whether the error categories match the previous attempt.
	ErrorCategoriesMatch bool
}

// StaleFixDetector compares snapshots across retry attempts.
type StaleFixDetector struct {
	previous *StaleFixSnapshot
}

// NewStaleFixDetector creates a detector for retry progress tracking.
func NewStaleFixDetector() *StaleFixDetector {
	return &StaleFixDetector{}
}

// RecordAttempt registers a new snapshot and reports whether it repeats the prior one.
func (d *StaleFixDetector) RecordAttempt(snapshot StaleFixSnapshot) StaleFixDetection {
	detection := StaleFixDetection{}
	if d.previous != nil {
		detection.ChangedFilesMatch = stringSlicesEqual(snapshot.ChangedFiles, d.previous.ChangedFiles)
		detection.ErrorCategoriesMatch = stringSlicesEqual(snapshot.ErrorCategories, d.previous.ErrorCategories)
		detection.StaleFixDetected = detection.ChangedFilesMatch && detection.ErrorCategoriesMatch
	}
	d.previous = &StaleFixSnapshot{
		ChangedFiles:   cloneStringSlice(snapshot.ChangedFiles),
		ErrorCategories: cloneStringSlice(snapshot.ErrorCategories),
	}
	return detection
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func stringSlicesEqual(a, b []string) bool {
	countsA := make(map[string]int, len(a))
	for _, entry := range a {
		countsA[entry]++
	}
	countsB := make(map[string]int, len(b))
	for _, entry := range b {
		countsB[entry]++
	}
	if len(countsA) != len(countsB) {
		return false
	}
	for key, countA := range countsA {
		if countB, ok := countsB[key]; !ok || countA != countB {
			return false
		}
	}
	return true
}
