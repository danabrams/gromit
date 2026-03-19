package specloop

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	cannotReadRe      = regexp.MustCompile(`^cannot read "([^"]+)"`)
	fileNotExistRe    = regexp.MustCompile(`^file "([^"]+)" does not exist$`)
	patternNotFoundRe = regexp.MustCompile(`^pattern "([^"]+)" not found in "([^"]+)"$`)
)

// extractRootCause takes an error detail string and returns a grouping key
// used to deduplicate related errors.
//
// Supported patterns:
// - "cannot read \"<path>\"" -> returns "<path>"
// - "file \"<path>\" does not exist" -> returns "<path>"
// - "pattern \"<pattern>\" not found in \"<path>\"" -> returns "<path>:<pattern>"
// - Unrecognized patterns -> returns empty string
func extractRootCause(detail string) string {
	// Pattern 1: cannot read "<path>"
	if matches := cannotReadRe.FindStringSubmatch(detail); matches != nil {
		return matches[1]
	}

	// Pattern 2: file "<path>" does not exist
	if matches := fileNotExistRe.FindStringSubmatch(detail); matches != nil {
		return matches[1]
	}

	// Pattern 3: pattern "<pattern>" not found in "<path>"
	if matches := patternNotFoundRe.FindStringSubmatch(detail); matches != nil {
		path := matches[2]
		pattern := matches[1]
		return path + ":" + pattern
	}

	// Unknown pattern
	return ""
}

// extractScenarioName extracts the scenario name from a contract failure string.
// It removes the "contract:" prefix, splits on " — " (em dash), takes the first segment,
// and trims whitespace.
//
// Examples:
// - "contract:Happy path — file_exists failed: something" -> "Happy path"
// - "contract:No delimiter scenario" -> "No delimiter scenario"
// - "contract:" -> ""
func extractScenarioName(s string) string {
	// Remove "contract:" prefix
	if strings.HasPrefix(s, "contract:") {
		s = strings.TrimPrefix(s, "contract:")
	}

	// Split on " — " (em dash) and take first segment
	parts := strings.Split(s, " — ")
	scenario := parts[0]

	// Trim whitespace
	return strings.TrimSpace(scenario)
}

// extractDetailFromContract extracts the detail part from a contract failure.
// Format: "contract:<scenario> — <assertion> failed: <detail>"
func extractDetailFromContract(failure string) string {
	assertionIdx := strings.Index(failure, " — ")
	if assertionIdx == -1 {
		return ""
	}

	afterAssert := failure[assertionIdx+5:] // Skip " — " (em dash separator: 5 bytes in UTF-8)
	failedIdx := strings.Index(afterAssert, " failed: ")
	if failedIdx == -1 {
		return ""
	}

	return afterAssert[failedIdx+9:] // Skip " failed: "
}

// DeduplicateFailures takes a list of failure strings and deduplicates contract failures
// while preserving non-contract failures and persistent-failure hints.
//
// Contract failures (prefix 'contract:') are grouped by their root cause:
// - Single failure in a group: kept as-is
// - Multiple failures in a group: collapsed to 'N contract assertions failed: <description> (scenarios: ...)'
//
// Non-contract failures and persistent-failure hints are preserved as-is.
// Output order: non-contract failures first, then contract failures (singles and summaries).
func DeduplicateFailures(failures []string) []string {
	if len(failures) == 0 {
		return []string{}
	}

	var nonContractFailures []string
	contractFailures := make(map[string][]string) // groupKey -> []original failure string
	contractDetails := make(map[string]string)    // groupKey -> normalized description

	for _, failure := range failures {
		// Persistent-failure hints pass through unchanged
		if strings.HasPrefix(failure, "persistent-failure:") {
			nonContractFailures = append(nonContractFailures, failure)
			continue
		}

		// Non-contract failures pass through unchanged
		if !strings.HasPrefix(failure, "contract:") {
			nonContractFailures = append(nonContractFailures, failure)
			continue
		}

		// Process contract failures
		detail := extractDetailFromContract(failure)
		groupKey := extractRootCause(detail)

		// Unrecognized patterns (empty groupKey) are treated as ungroupable
		if groupKey == "" {
			nonContractFailures = append(nonContractFailures, failure)
			continue
		}

		contractFailures[groupKey] = append(contractFailures[groupKey], failure)

		// Store normalized description
		if _, exists := contractDetails[groupKey]; !exists {
			contractDetails[groupKey] = detail
		}
	}

	// Build output: non-contracts first, then contracts
	var result []string

	// Add non-contract failures first
	result = append(result, nonContractFailures...)

	// Add deduplicated contracts in the order they first appeared
	seen := make(map[string]bool)
	for _, failure := range failures {
		if !strings.HasPrefix(failure, "contract:") {
			continue
		}

		detail := extractDetailFromContract(failure)
		groupKey := extractRootCause(detail)

		// Skip unrecognized patterns (already added to nonContractFailures)
		if groupKey == "" {
			continue
		}

		if seen[groupKey] {
			continue
		}
		seen[groupKey] = true

		group := contractFailures[groupKey]
		if len(group) == 1 {
			result = append(result, group[0])
		} else {
			// Create summary
			scenarioSet := make(map[string]bool)
			var scenarios []string
			for _, g := range group {
				scenario := extractScenarioName(g)
				if scenario != "" && !scenarioSet[scenario] {
					scenarios = append(scenarios, scenario)
					scenarioSet[scenario] = true
				}
			}

			count := len(group)
			normalizedDesc := contractDetails[groupKey]
			// For missing-file groups (plain path, no ':'), use normalized format
			if !strings.Contains(groupKey, ":") {
				normalizedDesc = fmt.Sprintf(`file "%s" does not exist`, groupKey)
			}
			scenarioList := strings.Join(scenarios, ", ")
			summary := fmt.Sprintf("%d contract assertions failed: %s (scenarios: %s)", count, normalizedDesc, scenarioList)
			result = append(result, summary)
		}
	}

	return result
}
