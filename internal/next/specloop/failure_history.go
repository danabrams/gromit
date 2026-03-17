package specloop

import (
	"fmt"
	"strings"
)

// ExtractTestFailureKeys extracts test function names from '--- FAIL: TestFunctionName' patterns in failure strings
func ExtractTestFailureKeys(failures []string) []string {
	var keys []string
	for _, failure := range failures {
		if strings.HasPrefix(failure, "--- FAIL: ") {
			// Extract the part after "--- FAIL: "
			remaining := strings.TrimPrefix(failure, "--- FAIL: ")
			// The test name is the first word (up to space or end of string)
			parts := strings.Fields(remaining)
			if len(parts) > 0 {
				keys = append(keys, parts[0])
			}
		}
	}
	return keys
}

// ExtractContractFailureKeys extracts 'contract:<scenario-name>' keys by splitting on ' — ' and taking the first segment
func ExtractContractFailureKeys(failures []string) []string {
	var keys []string
	for _, failure := range failures {
		if strings.HasPrefix(failure, "contract:") {
			// Split on ' — ' and take the first segment
			parts := strings.Split(failure, " — ")
			if len(parts) > 0 {
				key := strings.TrimSpace(parts[0])
				if key != "" {
					keys = append(keys, key)
				}
			}
		}
	}
	return keys
}

// UpdateFailureHistory increments count for keys present, resets to zero for keys not present (then deletes zero entries)
func UpdateFailureHistory(history map[string]int, currentKeys []string) {
	// Create a map of current keys for quick lookup
	currentKeySet := make(map[string]bool)
	for _, key := range currentKeys {
		currentKeySet[key] = true
	}

	// Increment counts for keys that are present in currentKeys
	for key := range currentKeySet {
		history[key]++
	}

	// Reset to zero (and mark for deletion) for keys not present
	keysToDelete := []string{}
	for key := range history {
		if !currentKeySet[key] {
			history[key] = 0
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Delete zero entries
	for _, key := range keysToDelete {
		delete(history, key)
	}
}

// AnnotateWithPersistentHints appends persistent failure hints to failures for keys with count >= threshold
func AnnotateWithPersistentHints(failures []string, history map[string]int, threshold int) []string {
	var annotated []string

	for _, failure := range failures {
		annotated = append(annotated, failure)

		var key string
		var found bool

		// Check if this is a test failure
		if strings.HasPrefix(failure, "--- FAIL: ") {
			parts := strings.Fields(strings.TrimPrefix(failure, "--- FAIL: "))
			if len(parts) > 0 {
				key = parts[0]
				found = true
			}
		} else if strings.HasPrefix(failure, "contract:") {
			// Check if this is a contract failure
			parts := strings.Split(failure, " — ")
			if len(parts) > 0 {
				key = strings.TrimSpace(parts[0])
				found = true
			}
		}

		// If we found a key and it has met or exceeded the threshold, add a hint
		if found && history[key] >= threshold {
			hint := fmt.Sprintf("persistent-failure: %s has failed %d consecutive cycles — may indicate a bad test specification rather than an implementation bug",
				key, history[key])
			annotated = append(annotated, hint)
		}
	}

	return annotated
}
