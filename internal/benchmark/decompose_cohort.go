package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	stdstrings "strings"

	"github.com/danabrams/gromit/internal/bead"
)

const requiredCohortSize = 5

// SpecBeadLister exposes the subset of bead client behavior needed for cohort selection.
type SpecBeadLister interface {
	ListWithLabel(ctx context.Context, label string) ([]*bead.Bead, error)
}

// DecomposeCohortSelector selects eligible specs for the decompose benchmark.
type DecomposeCohortSelector struct {
	store    SpecBeadLister
	plansDir string
}

// NewDecomposeCohortSelector creates a selector that evaluates specs in the provided plans directory.
func NewDecomposeCohortSelector(store SpecBeadLister, plansDir string) *DecomposeCohortSelector {
	return &DecomposeCohortSelector{store: store, plansDir: plansDir}
}

// Select returns the top eligible specs ordered by closed bead count descending with a name tie-break.
func (s *DecomposeCohortSelector) Select(ctx context.Context, candidates []string) ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("selector is nil")
	}
	if s.store == nil {
		return nil, fmt.Errorf("spec bead store is nil")
	}

	var specNames []string
	if len(candidates) > 0 {
		specNames = append([]string(nil), candidates...)
	} else {
		specs, err := listPlanSpecs(s.planDir())
		if err != nil {
			return nil, err
		}
		specNames = specs
	}

	metrics := make([]specMetrics, 0, len(specNames))
	for _, spec := range specNames {
		spec = stdstrings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if err := ensurePlanFile(s.planDir(), spec); err != nil {
			return nil, err
		}
		closed, open, err := s.countBeads(ctx, spec)
		if err != nil {
			return nil, err
		}
		if closed >= 1 && open == 0 {
			metrics = append(metrics, specMetrics{name: spec, closed: closed})
		}
	}

	if len(metrics) < requiredCohortSize {
		return nil, fmt.Errorf("insufficient eligible specs: got %d, require %d", len(metrics), requiredCohortSize)
	}

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].closed != metrics[j].closed {
			return metrics[i].closed > metrics[j].closed
		}
		return metrics[i].name < metrics[j].name
	})

	selected := make([]string, 0, requiredCohortSize)
	for i := 0; i < requiredCohortSize; i++ {
		selected = append(selected, metrics[i].name)
	}
	return selected, nil
}

// ValidateOverrides ensures an explicit spec list satisfies the eligibility checks.
func (s *DecomposeCohortSelector) ValidateOverrides(ctx context.Context, overrides []string) error {
	if s == nil {
		return fmt.Errorf("selector is nil")
	}
	if s.store == nil {
		return fmt.Errorf("spec bead store is nil")
	}

	if len(overrides) != requiredCohortSize {
		return fmt.Errorf("--specs must include exactly %d spec ids", requiredCohortSize)
	}

	seen := make(map[string]struct{}, len(overrides))
	for _, raw := range overrides {
		spec := stdstrings.TrimSpace(raw)
		if spec == "" {
			return fmt.Errorf("spec ids cannot be empty")
		}
		if _, dup := seen[spec]; dup {
			return fmt.Errorf("--specs must not include duplicate spec ids")
		}

		if err := ensurePlanFile(s.planDir(), spec); err != nil {
			return err
		}
		closed, open, err := s.countBeads(ctx, spec)
		if err != nil {
			return err
		}
		if open > 0 {
			return fmt.Errorf("spec %q has %d unclosed beads", spec, open)
		}
		if closed == 0 {
			return fmt.Errorf("spec %q must have at least one closed bead", spec)
		}
		seen[spec] = struct{}{}
	}

	return nil
}

func (s *DecomposeCohortSelector) planDir() string {
	if s.plansDir != "" {
		return s.plansDir
	}
	return ".gromit/plans"
}

func ensurePlanFile(plansDir, spec string) error {
	if spec == "" {
		return fmt.Errorf("spec name is empty")
	}
	path := filepath.Join(plansDir, spec+".md")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("plan file for spec %q not found", spec)
		}
		return fmt.Errorf("stat plan file for spec %q: %w", spec, err)
	}
	if info.IsDir() {
		return fmt.Errorf("plan path for spec %q is a directory", spec)
	}
	return nil
}

func (s *DecomposeCohortSelector) countBeads(ctx context.Context, spec string) (closed, open int, err error) {
	beads, err := s.store.ListWithLabel(ctx, "spec:"+spec)
	if err != nil {
		return 0, 0, fmt.Errorf("list beads for spec %q: %w", spec, err)
	}
	for _, bead := range beads {
		if bead == nil {
			continue
		}
		status := stdstrings.TrimSpace(stdstrings.ToLower(bead.Status))
		if status == "closed" {
			closed++
		} else {
			open++
		}
	}
	return closed, open, nil
}

func listPlanSpecs(plansDir string) ([]string, error) {
	dir := plansDir
	if dir == "" {
		dir = ".gromit/plans"
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read plans dir %q: %w", dir, err)
	}
	specs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if stdstrings.EqualFold(filepath.Ext(name), ".md") {
			spec := stdstrings.TrimSuffix(name, filepath.Ext(name))
			spec = stdstrings.TrimSpace(spec)
			if spec != "" {
				specs = append(specs, spec)
			}
		}
	}
	sort.Strings(specs)
	return specs, nil
}

type specMetrics struct {
	name   string
	closed int
}
