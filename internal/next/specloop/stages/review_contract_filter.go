package stages

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/review"
	"gopkg.in/yaml.v3"
)

// contractRequirement is a (path, pattern) tuple from a file_contains assertion.
type contractRequirement struct {
	path    string
	pattern string
}

// filterContractContradictions removes review findings that contradict contract
// assertions. A contradiction exists when a review finding suggests removing a
// pattern from a file, but a contract asserts that pattern must exist in that
// file. Suppressing such findings prevents infinite replan loops where the
// reviewer and contracts give opposing instructions.
//
// Returns the filtered findings and the count of suppressed findings.
func filterContractContradictions(findings []review.Finding, evidenceDir string) ([]review.Finding, int) {
	if evidenceDir == "" {
		return findings, 0
	}

	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		return findings, 0
	}

	var sc contract.ScenarioContract
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return findings, 0
	}

	required := extractContractRequirements(sc)
	if len(required) == 0 {
		return findings, 0
	}

	var filtered []review.Finding
	suppressed := 0
	for _, f := range findings {
		if isContradicted(f, required) {
			suppressed++
			continue
		}
		filtered = append(filtered, f)
	}
	if filtered == nil {
		filtered = []review.Finding{}
	}
	return filtered, suppressed
}

// extractContractRequirements builds a list of (path, pattern) tuples from
// all file_contains assertions in the contract.
func extractContractRequirements(sc contract.ScenarioContract) []contractRequirement {
	var required []contractRequirement
	for _, scenario := range sc.Scenarios {
		for _, a := range scenario.Assertions {
			if a.FileContains != nil {
				required = append(required, contractRequirement{
					path:    a.FileContains.Path,
					pattern: a.FileContains.Pattern,
				})
			}
		}
	}
	return required
}

// isContradicted returns true if a review finding's suggested fix would remove
// something that a contract requires to exist.
func isContradicted(f review.Finding, required []contractRequirement) bool {
	fix := strings.ToLower(f.SuggestedFix)
	if fix == "" {
		return false
	}

	// Only check findings whose fix suggests removal.
	isRemoval := strings.Contains(fix, "remove") ||
		strings.Contains(fix, "delete") ||
		strings.Contains(fix, "drop")
	if !isRemoval {
		return false
	}

	for _, r := range required {
		// The finding's file must match the contract's path.
		if !pathMatches(f.File, r.path) {
			continue
		}
		// The contract's required pattern must appear in the finding description
		// or suggested fix — indicating the finding wants to remove the
		// contract-protected content.
		if strings.Contains(f.Description, r.pattern) || strings.Contains(f.SuggestedFix, r.pattern) {
			return true
		}
	}
	return false
}

// pathMatches returns true if the finding file and contract path refer to the
// same file. Handles both exact match and suffix match (finding might use a
// shorter path like "types.go" vs contract "internal/next/reviewdistiller/types.go").
func pathMatches(findingFile, contractPath string) bool {
	if findingFile == contractPath {
		return true
	}
	return strings.HasSuffix(contractPath, "/"+findingFile) ||
		strings.HasSuffix(findingFile, "/"+contractPath) ||
		filepath.Base(findingFile) == filepath.Base(contractPath)
}
