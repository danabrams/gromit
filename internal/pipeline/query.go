package pipeline

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/frontmatter"
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

// ListUndecomposedPlans returns plan names whose frontmatter does not declare decomposition.
func ListUndecomposedPlans(plansDir string) ([]string, error) {
	planFiles, err := ListMarkdownFiles(plansDir)
	if err != nil {
		return nil, fmt.Errorf("listing plans: %w", err)
	}

	undecomposed := make([]string, 0, len(planFiles))

	for _, plan := range planFiles {
		name := strings.TrimSuffix(filepath.Base(plan), ".md")

		fm, _, err := frontmatter.ReadFile(plan)
		if err != nil {
			return nil, fmt.Errorf("reading plan frontmatter %s: %w", name, err)
		}

		decomposed, ok := fm["decomposed"].(bool)
		if !ok || !decomposed {
			undecomposed = append(undecomposed, name)
		}
	}

	sort.Strings(undecomposed)
	return undecomposed, nil
}
