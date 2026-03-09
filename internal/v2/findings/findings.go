package findings

// Severity describes how urgent a finding is.
type Severity string

const (
	// SeverityCritical signals a failure that forces a fail verdict.
	SeverityCritical Severity = "critical"
)

// Verdict describes the pass/fail outcome derived from findings.
type Verdict string

const (
	VerdictPass Verdict = "pass"
	VerdictFail Verdict = "fail"
)

// Finding captures structured details produced by a spec-level review.
type Finding struct {
	Severity      Severity `json:"severity"`
	Category      string   `json:"category"`
	Scope         string   `json:"scope"`
	Description   string   `json:"description"`
	AffectedFiles []string `json:"affected_files"`
}

// DeriveVerdict returns a fail verdict if any finding is critical.
func DeriveVerdict(findings []Finding) Verdict {
	for _, f := range findings {
		if f.Severity == SeverityCritical {
			return VerdictFail
		}
	}
	return VerdictPass
}
