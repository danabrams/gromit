package contract

// SpecScenario represents a single scenario parsed from the spec's Scenarios section.
// Extracted from spec markdown by matching "### Scenario:" headers and Given/When/Then blocks.
type SpecScenario struct {
	Name  string // Scenario title (from ### header, minus "Scenario: " prefix)
	Given string // Given block text
	When  string // When block text
	Then  string // Then block text
	Notes string // Notes block text (optional)
}

// ScenarioContract is the contract assertion file written by the WriteContracts stage.
type ScenarioContract struct {
	Scenarios []ScenarioAssertions `yaml:"scenarios"`
}

// ScenarioAssertions holds the assertions for a single scenario.
type ScenarioAssertions struct {
	Name       string              `yaml:"name"`
	Assertions []ContractAssertion `yaml:"assertions"`
}

// ContractAssertion is a typed subset of e2e.Assertion, only filesystem-checkable fields.
// This is a separate type from e2e.Assertion; do not import the e2e package.
// Single-key map — exactly one field must be set per assertion.
type ContractAssertion struct {
	FileExists      string                 `yaml:"file_exists,omitempty"`
	FileContains    *FileContainsAssertion `yaml:"file_contains,omitempty"`
	FileNotModified string                 `yaml:"file_not_modified,omitempty"`
	FileNotExists   string                 `yaml:"file_not_exists,omitempty"`
	FileNotContains *FileContainsAssertion `yaml:"file_not_contains,omitempty"`
}

// FileContainsAssertion holds path and pattern for file_contains / file_not_contains assertions.
type FileContainsAssertion struct {
	Path    string `yaml:"path"`
	Pattern string `yaml:"pattern"` // Literal substring, matched via strings.Contains
}

// ContractFailure represents a single failed contract assertion.
type ContractFailure struct {
	ScenarioName  string // e.g., "subtract-works"
	AssertionType string // e.g., "file_contains"
	Details       string // Human-readable failure description
}
