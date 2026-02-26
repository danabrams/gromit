package display

import "time"

// FormatRun renders the current run state for display.
func FormatRun(status *RunStatus) string {
	if status == nil {
		return "Run: not running"
	}
	return ""
}

// FormatHealth renders health information for display.
func FormatHealth(lastRetro time.Time, iterationsSinceReview int) string {
	return "Health:\n"
}
