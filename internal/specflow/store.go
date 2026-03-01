package specflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/frontmatter"
)

// NewSpecFrontmatterStore creates a SpecStore backed by the spec frontmatter files under .gromit/specs.
func NewSpecFrontmatterStore(gromitDir string) (SpecStore, error) {
	dir := strings.TrimSpace(gromitDir)
	if dir == "" {
		dir = ".gromit"
	}
	return &specFrontmatterStore{specsDir: filepath.Join(dir, "specs")}, nil
}

type specFrontmatterStore struct {
	specsDir string
}

func (s *specFrontmatterStore) Stage(_ context.Context, specID string) (Stage, error) {
	if s == nil {
		return "", fmt.Errorf("specfrontmatter store is nil")
	}
	fm, _, err := frontmatter.ReadFile(s.specPath(specID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrStageNotFound
		}
		return "", fmt.Errorf("reading spec frontmatter: %w", err)
	}
	raw, ok := fm["stage"]
	if !ok {
		return "", ErrStageNotFound
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("invalid stage value: %v", raw)
	}
	return Stage(value), nil
}

func (s *specFrontmatterStore) StoreStage(_ context.Context, specID string, stage Stage) error {
	if s == nil {
		return fmt.Errorf("specfrontmatter store is nil")
	}
	updates := map[string]interface{}{"stage": string(stage)}
	if err := frontmatter.UpdateFile(s.specPath(specID), updates); err != nil {
		return fmt.Errorf("updating spec frontmatter: %w", err)
	}
	return nil
}

func (s *specFrontmatterStore) specPath(specID string) string {
	return filepath.Join(s.specsDir, specID+".md")
}
