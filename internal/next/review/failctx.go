package review

import "fmt"

// BuildFailureStrings converts blocking findings from a RunResult into
// human-readable strings for planner FailureContext consumption.
func BuildFailureStrings(result RunResult) []string {
	return ReviewFailuresToStrings(result.BlockingFindings)
}

// ReviewFailuresToStrings converts a slice of Finding to human-readable
// failure strings for FailureContext.Failures. This is the review-side
// counterpart to AcceptanceFailuresToStrings in the acceptor package.
func ReviewFailuresToStrings(findings []Finding) []string {
	var strs []string
	for _, f := range findings {
		s := fmt.Sprintf("review:%s:%s:%s:%d — %s",
			f.Facet, f.Severity, f.File, f.Line, f.Description)
		if f.SuggestedFix != "" {
			s += fmt.Sprintf(" (suggested fix: %s)", f.SuggestedFix)
		}
		strs = append(strs, s)
	}
	return strs
}
