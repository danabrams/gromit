package logger

import (
	"fmt"
	"path/filepath"
	"sort"
)

// ReadSiblingTouchedPackages returns the deterministic union of touched packages
// from completed sibling iterations that share the same spec context and/or are
// explicitly listed as parent-epic siblings.
func ReadSiblingTouchedPackages(logsDir, currentBeadID, specID string, parentSiblingBeadIDs []string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	siblingIDs := make(map[string]struct{}, len(parentSiblingBeadIDs))
	for _, beadID := range parentSiblingBeadIDs {
		if beadID == "" || beadID == currentBeadID {
			continue
		}
		siblingIDs[beadID] = struct{}{}
	}

	packages := make(map[string]struct{})
	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue // Skip unreadable files
		}
		for _, entry := range entries {
			if !entry.Success || entry.BeadID == "" || entry.BeadID == currentBeadID {
				continue
			}

			_, sameParentContext := siblingIDs[entry.BeadID]
			sameSpecContext := specID != "" && entry.SpecID == specID
			if !sameParentContext && !sameSpecContext {
				continue
			}

			for _, pkg := range entry.TouchedPackages {
				if pkg == "" {
					continue
				}
				packages[pkg] = struct{}{}
			}
		}
	}

	return sortedPackageList(packages), nil
}

// ReadSiblingTouchedPackagesBySpec returns touched packages from completed sibling
// iterations with the same non-empty spec context.
func ReadSiblingTouchedPackagesBySpec(logsDir, currentBeadID, specID string) ([]string, error) {
	return ReadSiblingTouchedPackages(logsDir, currentBeadID, specID, nil)
}

// ReadSiblingTouchedPackagesByParent returns touched packages from completed
// sibling iterations whose bead IDs are provided by the parent-epic context.
func ReadSiblingTouchedPackagesByParent(logsDir, currentBeadID string, siblingBeadIDs []string) ([]string, error) {
	return ReadSiblingTouchedPackages(logsDir, currentBeadID, "", siblingBeadIDs)
}

func sortedPackageList(packages map[string]struct{}) []string {
	if len(packages) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(packages))
	for pkg := range packages {
		result = append(result, pkg)
	}
	sort.Strings(result)
	return result
}
