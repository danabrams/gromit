package contract

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidateContract checks each ContractAssertion in the contract against the known
// assertion vocabulary. Each assertion must have exactly one field set; zero fields
// or multiple fields are vocabulary violations. Returns a slice of error strings
// describing any violations found.
func ValidateContract(c ScenarioContract) []string {
	var errs []string
	for _, scenario := range c.Scenarios {
		for i, a := range scenario.Assertions {
			count := 0
			if a.FileExists != "" {
				count++
			}
			if a.FileContains != nil {
				count++
			}
			if a.FileNotModified != "" {
				count++
			}
			if a.FileNotExists != "" {
				count++
			}
			if a.FileNotContains != nil {
				count++
			}
			if count != 1 {
				errs = append(errs, fmt.Sprintf("scenario %q assertion %d: expected exactly 1 field set, got %d", scenario.Name, i, count))
			}
		}
	}
	return errs
}

// ParseContractYAML extracts YAML from raw LLM output and unmarshals it into a ScenarioContract.
// It strips markdown YAML fences and plain code fences before parsing.
func ParseContractYAML(output string) (ScenarioContract, error) {
	extracted := extractYAML(output)
	var c ScenarioContract
	if err := yaml.Unmarshal([]byte(extracted), &c); err != nil {
		return ScenarioContract{}, fmt.Errorf("parse contract YAML: %w", err)
	}
	return c, nil
}

// extractYAML strips YAML code fences from LLM output.
func extractYAML(output string) string {
	if idx := strings.Index(output, "```yaml"); idx >= 0 {
		start := idx + len("```yaml")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			return strings.TrimSpace(output[start : start+end])
		}
	}
	if idx := strings.Index(output, "```"); idx >= 0 {
		start := idx + len("```")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			return strings.TrimSpace(output[start : start+end])
		}
	}
	return strings.TrimSpace(output)
}
