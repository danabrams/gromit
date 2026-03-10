package review

// FindingSeverity describes how severe a review finding is.
type FindingSeverity string

const (
	SeverityCritical   FindingSeverity = "critical"
	SeverityWarning    FindingSeverity = "warning"
	SeveritySuggestion FindingSeverity = "suggestion"
)

// FindingCategory classifies a finding by its domain.
type FindingCategory string

const (
	CategoryBug          FindingCategory = "bug"
	CategorySecurity     FindingCategory = "security"
	CategoryQuality      FindingCategory = "quality"
	CategoryTestGap      FindingCategory = "test_gap"
	CategoryArchitecture FindingCategory = "architecture"
	CategoryAcceptance   FindingCategory = "acceptance"
)

// FindingScope describes whether the finding targets spec changes or general code.
type FindingScope string

const (
	ScopeSpec    FindingScope = "spec"
	ScopeGeneral FindingScope = "general"
)

// Finding captures a single review observation.
type Finding struct {
	Title         string
	Description   string
	Priority      int
	Severity      FindingSeverity
	Category      FindingCategory
	Scope         FindingScope
	AffectedFiles []string
	InScope       bool
}

// Verdict records whether the review findings pass or fail.
type Verdict string

const (
	VerdictPass Verdict = "pass"
	VerdictFail Verdict = "fail"
)

// ComputeVerdict returns a fail verdict whenever any findings exist.
func ComputeVerdict(findings []Finding) Verdict {
	if len(findings) > 0 {
		return VerdictFail
	}
	return VerdictPass
}
