package dep

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/specflow"
)

type SpecDependencyGate struct {
	specsDir string
}

// NewSpecDependencyGate creates a gate backed by the provided specs directory.
func NewSpecDependencyGate(specsDir string) (*SpecDependencyGate, error) {
	if strings.TrimSpace(specsDir) == "" {
		return nil, fmt.Errorf("specs dir required")
	}
	return &SpecDependencyGate{specsDir: specsDir}, nil
}

// EnsureSpecReady returns an error if dependencies for specID are not satisfied.
func (s *SpecDependencyGate) EnsureSpecReady(ctx context.Context, specID string) error {
	if s == nil {
		return fmt.Errorf("spec dependency gate is nil")
	}
	meta, err := s.loadSpec(specID)
	if err != nil {
		return err
	}
	blockers, _ := s.blockingDependencies(meta)
	if len(blockers) == 0 {
		return nil
	}
	return &SpecDependencyError{SpecID: meta.ID, Blocking: blockers}
}

// ListReady returns the spec IDs whose dependencies are satisfied and which are not marked done.
func (s *SpecDependencyGate) ListReady(ctx context.Context) ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("spec dependency gate is nil")
	}
	entries, err := os.ReadDir(s.specsDir)
	if err != nil {
		return nil, fmt.Errorf("reading specs dir: %w", err)
	}
	ready := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			continue
		}
		specID := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		meta, err := s.loadSpec(specID)
		if err != nil {
			return nil, err
		}
		blockers, _ := s.blockingDependencies(meta)
		if len(blockers) == 0 {
			ready = append(ready, meta.ID)
		}
	}
	sort.Strings(ready)
	return ready, nil
}

// SpecDependencyError signals that a spec is blocked by unfinished dependencies.
type SpecDependencyError struct {
	SpecID   string
	Blocking []string
}

// Error formats a descriptive message including the blocking spec IDs.
func (e *SpecDependencyError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("spec %s blocked by dependencies: %s", e.SpecID, strings.Join(e.Blocking, ", "))
}

// BlockingIDs returns a copy of the blocking spec IDs reported by the error.
func (e *SpecDependencyError) BlockingIDs() []string {
	if e == nil {
		return nil
	}
	out := make([]string, len(e.Blocking))
	copy(out, e.Blocking)
	return out
}

func (s *SpecDependencyGate) blockingDependencies(meta *specMetadata) ([]string, error) {
	if meta == nil || len(meta.DependsOn) == 0 {
		return nil, nil
	}
	blockers := make([]string, 0, len(meta.DependsOn))
	seen := map[string]struct{}{}
	for _, dep := range meta.DependsOn {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if _, ok := seen[dep]; ok {
			continue
		}
		seen[dep] = struct{}{}

		depMeta, err := s.loadSpec(dep)
		if err != nil {
			blockers = append(blockers, dep)
			continue
		}
		if depMeta.Stage != specflow.StageDone {
			blockers = append(blockers, depMeta.ID)
		}
	}
	sort.Strings(blockers)
	return blockers, nil
}

func (s *SpecDependencyGate) loadSpec(specID string) (*specMetadata, error) {
	path := filepath.Join(s.specsDir, specID+".md")
	fm, _, err := frontmatter.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec %s: %w", specID, err)
	}
	return parseSpecMetadata(specID, fm), nil
}
