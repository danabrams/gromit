package specflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/danabrams/gromit/internal/frontmatter"
)

// NewSpecFrontmatterStore creates a SpecStore backed by the spec frontmatter files under .gromit/specs.
func NewSpecFrontmatterStore(gromitDir string) (SpecStore, error) {
	dir := strings.TrimSpace(gromitDir)
	if dir == "" {
		dir = ".gromit"
	}
	return newSpecFrontmatterStore(filepath.Join(dir, "specs")), nil
}

type specFrontmatterStore struct {
	specsDir   string
	mu         sync.Mutex
	readFile   func(string) (map[string]interface{}, string, error)
	updateFile func(string, map[string]interface{}) error
}

func newSpecFrontmatterStore(specsDir string) *specFrontmatterStore {
	return &specFrontmatterStore{
		specsDir:   specsDir,
		readFile:   frontmatter.ReadFile,
		updateFile: frontmatter.UpdateFile,
	}
}

var ErrMalformedSpecFrontmatter = errors.New("spec frontmatter is malformed")

func (s *specFrontmatterStore) Stage(_ context.Context, specID string) (Stage, error) {
	if s == nil {
		return "", fmt.Errorf("specfrontmatter store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fm, _, err := s.readSpecFile(s.specPath(specID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrStageNotFound
		}
		return "", fmt.Errorf("%w: %w", ErrMalformedSpecFrontmatter, err)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	updates := map[string]interface{}{"stage": string(stage)}
	if err := s.updateSpecFile(s.specPath(specID), updates); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformedSpecFrontmatter, err)
	}
	return nil
}

func (s *specFrontmatterStore) specPath(specID string) string {
	return filepath.Join(s.specsDir, specID+".md")
}

func (s *specFrontmatterStore) readSpecFile(path string) (map[string]interface{}, string, error) {
	if s != nil && s.readFile != nil {
		return s.readFile(path)
	}
	return frontmatter.ReadFile(path)
}

func (s *specFrontmatterStore) updateSpecFile(path string, updates map[string]interface{}) error {
	if s != nil && s.updateFile != nil {
		return s.updateFile(path, updates)
	}
	return frontmatter.UpdateFile(path, updates)
}
