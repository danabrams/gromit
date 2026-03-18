package contract

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// runtimeArtifacts is the set of pipeline runtime artifact filenames that live
// in the run store, not the code worktree. Initialized once at package load.
var runtimeArtifacts = map[string]bool{
	"run.json":              true,
	"execution-policy.json": true,
	"tasks.json":            true,
	"events.jsonl":          true,
	"spec-packet.md":        true,
	"spec.md":               true,
}

// isRuntimeArtifact checks if a path references a pipeline runtime artifact that
// lives in the run store, not the code worktree. These files should never be
// referenced in contract assertions.
func isRuntimeArtifact(path string) bool {
	// Check bare filename
	if runtimeArtifacts[path] {
		return true
	}
	// Check with .gromit-next/ prefix
	for f := range runtimeArtifacts {
		if path == ".gromit-next/"+f {
			return true
		}
	}
	return false
}

// ValidateContract checks each ContractAssertion in the contract against the known
// assertion vocabulary. Each assertion must have exactly one field set; zero fields
// or multiple fields are vocabulary violations. Additionally, assertion paths must
// not reference pipeline runtime artifacts that live in the run store.
// Returns a slice of error strings describing any violations found.
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
				continue
			}

			// Check paths don't reference runtime artifacts
			var path string
			switch {
			case a.FileExists != "":
				path = a.FileExists
			case a.FileNotExists != "":
				path = a.FileNotExists
			case a.FileContains != nil:
				path = a.FileContains.Path
			case a.FileNotContains != nil:
				path = a.FileNotContains.Path
			case a.FileNotModified != "":
				path = a.FileNotModified
			}

			if path != "" && isRuntimeArtifact(path) {
				errs = append(errs, fmt.Sprintf("scenario %q assertion %d: path %q references a runtime artifact that lives in the run store, not the code worktree. Use source code paths only (e.g., internal/, cmd/, etc.)", scenario.Name, i, path))
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
