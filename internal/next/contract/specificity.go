package contract

import "regexp"

// exportedIdentifierRegex matches single exported identifiers: start with uppercase, followed by alphanumeric/underscore
var exportedIdentifierRegex = regexp.MustCompile(`^[A-Z][a-zA-Z0-9_]*$`)

// SpecificityWarning describes a low-specificity pattern in a contract assertion.
type SpecificityWarning struct {
	ScenarioName string
	AssertionIdx int
	Pattern      string
	Path         string
	Reason       string
}

// ValidateContractSpecificity checks file_contains assertions for patterns
// that are likely to cause review↔contract contradiction thrash.
// Returns warnings for low-specificity patterns. An empty slice means all
// patterns are adequately specific.
func ValidateContractSpecificity(c ScenarioContract) []SpecificityWarning {
	var warnings []SpecificityWarning

	for _, scenario := range c.Scenarios {
		for assertionIdx, assertion := range scenario.Assertions {
			// Only check FileContains assertions; skip FileNotContains
			if assertion.FileContains != nil {
				pattern := assertion.FileContains.Pattern
				path := assertion.FileContains.Path

				// If pattern matches a single exported identifier, add a warning
				if exportedIdentifierRegex.MatchString(pattern) {
					warnings = append(warnings, SpecificityWarning{
						ScenarioName: scenario.Name,
						AssertionIdx: assertionIdx,
						Pattern:      pattern,
						Path:         path,
						Reason:       "single exported identifier — ambiguous if file contains multiple types",
					})
				}
			}
		}
	}

	return warnings
}
