package doctrine

import (
	"errors"
	"os"
)

// MergedDoctrine loads rules from global and local directories and merges them
// with local-wins semantics: for matching IDs, the local rule takes precedence.
// If a local rule has Status="superseded", it masks (excludes) the corresponding
// global rule from the result.
func MergedDoctrine(globalDir, localDir string) ([]Rule, error) {
	// Load global rules
	globalStore := &FSStore{Dir: globalDir}
	globalDoctrine, err := globalStore.Load()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		globalDoctrine = Doctrine{Rules: []Rule{}}
	}

	// Load local rules
	localStore := &FSStore{Dir: localDir}
	localDoctrine, err := localStore.Load()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		localDoctrine = Doctrine{Rules: []Rule{}}
	}

	// Build a map of local rules by ID
	localByID := make(map[string]Rule)
	maskedIDs := make(map[string]bool) // IDs to exclude due to superseded local rules
	for _, rule := range localDoctrine.Rules {
		localByID[rule.ID] = rule
		if rule.Status == "superseded" {
			maskedIDs[rule.ID] = true
		}
	}

	// Start with local rules (excluding superseded ones)
	result := []Rule{}
	for _, rule := range localDoctrine.Rules {
		if rule.Status != "superseded" {
			result = append(result, rule)
		}
	}

	// Add global rules that don't conflict with local rules
	for _, globalRule := range globalDoctrine.Rules {
		if _, hasLocal := localByID[globalRule.ID]; !hasLocal && !maskedIDs[globalRule.ID] {
			result = append(result, globalRule)
		}
	}

	return result, nil
}
