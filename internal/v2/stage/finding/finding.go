package finding

// Severity represents the impact level of a finding.
type Severity string

const (
	SeverityCritical   Severity = "critical"
	SeverityWarning    Severity = "warning"
	SeveritySuggestion Severity = "suggestion"
)

// Category classifies the type of a finding.
type Category string

const (
	CategoryBug          Category = "bug"
	CategorySecurity     Category = "security"
	CategoryQuality      Category = "quality"
	CategoryTestGap      Category = "test_gap"
	CategoryArchitecture Category = "architecture"
	CategoryAcceptance   Category = "acceptance"
)

// Finding captures a post-review issue discovered during the stage.
type Finding struct {
	Title         string
	Severity      Severity
	Category      Category
	Scope         string
	Description   string
	AffectedFiles []string
}

// HasCritical reports whether at least one finding is marked critical.
func HasCritical(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// NormalizeNilFields ensures that slice fields are never nil.
func (f *Finding) NormalizeNilFields() {
	if f == nil {
		return
	}
	if f.AffectedFiles == nil {
		f.AffectedFiles = []string{}
	}
}
