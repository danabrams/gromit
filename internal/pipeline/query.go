package pipeline

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ListUnplannedSpecs returns spec names that do not have a corresponding plan file.
func ListUnplannedSpecs(specsDir, plansDir string) ([]string, error) {
	specFiles, err := ListMarkdownFiles(specsDir)
	if err != nil {
		return nil, fmt.Errorf("listing specs: %w", err)
	}

	planFiles, err := ListMarkdownFiles(plansDir)
	if err != nil {
		return nil, fmt.Errorf("listing plans: %w", err)
	}

	planNames := make(map[string]struct{}, len(planFiles))
	for _, plan := range planFiles {
		planNames[strings.TrimSuffix(filepath.Base(plan), ".md")] = struct{}{}
	}

	unplanned := make([]string, 0, len(specFiles))
	for _, spec := range specFiles {
		name := strings.TrimSuffix(filepath.Base(spec), ".md")
		if _, ok := planNames[name]; !ok {
			unplanned = append(unplanned, name)
		}
	}

	sort.Strings(unplanned)
	return unplanned, nil
}
