package specreview

import "strings"

// SpecReviewFinding represents a single finding returned by a spec-level review.
type SpecReviewFinding struct {
    Verdict string
}

func verdictFromFindings(findings []SpecReviewFinding) string {
    for _, finding := range findings {
        if strings.EqualFold(strings.TrimSpace(finding.Verdict), "issue") {
            return "issue"
        }
    }
    return "pass"
}
