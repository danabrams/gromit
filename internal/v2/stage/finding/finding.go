package finding

// Severity represents the impact level of a finding.
type Severity string

const (
	SeverityCritical   Severity = "critical"
	SeverityWarning    Severity = "warning"
	SeveritySuggestion Severity = "suggestion"
)

// Finding captures a post-review issue discovered during the stage.
type Finding struct {
	Severity      Severity
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
