package review

import "fmt"

// Severity represents the severity level of a review finding.
type Severity int

const (
	SeverityInfo       Severity = iota + 1
	SeveritySuggestion
	SeverityWarning
	SeverityError
)

// Rank returns an integer for ordering comparisons. Higher rank = more severe.
func (s Severity) Rank() int {
	return int(s)
}

// String returns the lowercase string representation.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeveritySuggestion:
		return "suggestion"
	case SeverityInfo:
		return "info"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// ParseSeverity converts a string to a Severity value.
func ParseSeverity(s string) (Severity, error) {
	switch s {
	case "error":
		return SeverityError, nil
	case "warning":
		return SeverityWarning, nil
	case "suggestion":
		return SeveritySuggestion, nil
	case "info":
		return SeverityInfo, nil
	default:
		return 0, fmt.Errorf("unknown severity: %q", s)
	}
}
