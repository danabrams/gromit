package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
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

// ActiveBeadClient defines the subset of bead client functionality needed by ListActiveBeads.
type ActiveBeadClient interface {
	List(ctx context.Context) ([]*bead.Bead, error)
	ListByStatus(ctx context.Context, status string) ([]*bead.Bead, error)
}

// ListActiveBeads returns open and in-progress beads, filtering duplicates and invalid entries.
func ListActiveBeads(ctx context.Context, client ActiveBeadClient) ([]*bead.Bead, error) {
	if client == nil {
		return nil, fmt.Errorf("active bead client is nil")
	}

	open, err := client.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing active beads: %w", err)
	}

	inProgress, err := client.ListByStatus(ctx, "in_progress")
	if err != nil {
		return nil, fmt.Errorf("listing active beads: %w", err)
	}

	combined := append(open, inProgress...)
	result := make([]*bead.Bead, 0, len(combined))
	seen := make(map[string]struct{}, len(combined))

	for _, b := range combined {
		if b == nil {
			continue
		}

		id := strings.TrimSpace(b.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, b)
	}

	if len(result) == 0 {
		return []*bead.Bead{}, nil
	}
	return result, nil
}
