package spec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/frontmatter"
)

// Spec describes a v2 run loop spec file and its metadata.
type Spec struct {
	ID                    string
	Path                  string
	DependsOn             []string
	Accepted              bool
	ArchitectureDirection string
	TestStrategy          string
	Body                  string
}

// ReadySpec describes a spec whose dependencies are satisfied and is not yet accepted.
type ReadySpec struct {
	ID   string
	Path string
}

// SpecDependenciesError reports that dependencies are blocking execution.
type SpecDependenciesError struct {
	SpecID   string
	Blocking []string
}

// Error implements error.
func (e *SpecDependenciesError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("spec %s blocked by dependencies: %s", e.SpecID, strings.Join(e.Blocking, ", "))
}

// BlockingIDs returns a copy of the blocking dependency IDs reported by the error.
func (e *SpecDependenciesError) BlockingIDs() []string {
	if e == nil {
		return nil
	}
	out := make([]string, len(e.Blocking))
	copy(out, e.Blocking)
	return out
}

// Load parses spec metadata from the given file path.
func Load(path string) (*Spec, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("spec path required")
	}
	front, body, err := frontmatter.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec %s: %w", path, err)
	}
	specID := parseID(front, path)
	if specID == "" {
		specID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	return &Spec{
		ID:                    specID,
		Path:                  path,
		DependsOn:             parseDependencies(front),
		Accepted:              parseAccepted(front["accepted"]),
		ArchitectureDirection: extractSection(body, "Architecture Direction"),
		TestStrategy:          extractSection(body, "Test Strategy"),
		Body:                  strings.TrimSpace(body),
	}, nil
}

// CheckDependencies ensures all dependencies have been accepted before running.
func (s *Spec) CheckDependencies(specsDir string) error {
	if s == nil {
		return fmt.Errorf("spec is nil")
	}
	if strings.TrimSpace(specsDir) == "" {
		return fmt.Errorf("specs dir required")
	}

	blockers := make([]string, 0, len(s.DependsOn))
	seen := make(map[string]struct{})
	for _, dep := range s.DependsOn {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if _, ok := seen[dep]; ok {
			continue
		}
		seen[dep] = struct{}{}

		depPath := filepath.Join(specsDir, dep+".md")
		depSpec, err := Load(depPath)
		if err != nil {
			blockers = append(blockers, dep)
			continue
		}
		if !depSpec.Accepted {
			blockers = append(blockers, dep)
		}
	}

	if len(blockers) == 0 {
		return nil
	}

	sort.Strings(blockers)
	return &SpecDependenciesError{SpecID: s.ID, Blocking: blockers}
}

// ListReady returns specs whose dependencies are satisfied and are not yet accepted.
func ListReady(specsDir string) ([]ReadySpec, error) {
	if strings.TrimSpace(specsDir) == "" {
		return nil, fmt.Errorf("specs dir required")
	}
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, fmt.Errorf("reading specs dir: %w", err)
	}

	ready := make([]ReadySpec, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			continue
		}

		path := filepath.Join(specsDir, entry.Name())
		specEntry, err := Load(path)
		if err != nil {
			return nil, err
		}
		if specEntry.Accepted {
			continue
		}

		if err := specEntry.CheckDependencies(specsDir); err != nil {
			var depErr *SpecDependenciesError
			if errors.As(err, &depErr) {
				continue
			}
			return nil, err
		}

		ready = append(ready, ReadySpec{
			ID:   specEntry.ID,
			Path: path,
		})
	}

	sort.Slice(ready, func(i, j int) bool {
		return ready[i].ID < ready[j].ID
	})

	return ready, nil
}

func parseDependencies(front map[string]interface{}) []string {
	if front == nil {
		return nil
	}
	deps := parseDepends(front["dependencies"])
	deps = append(deps, parseDepends(front["depends_on"])...)
	return deps
}

func parseID(front map[string]interface{}, path string) string {
	if front == nil {
		return ""
	}
	if raw, ok := front["id"].(string); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}

func parseDepends(raw interface{}) []string {
	if raw == nil {
		return nil
	}
	var deps []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		deps = append(deps, value)
	}

	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	case []string:
		for _, item := range v {
			add(item)
		}
	case string:
		add(v)
	}

	return deps
}

func parseAccepted(raw interface{}) bool {
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		lower := strings.TrimSpace(strings.ToLower(v))
		return lower == "true" || lower == "yes" || lower == "1"
	}
	return false
}

func extractSection(body, heading string) string {
	if heading == "" {
		return ""
	}
	target := strings.ToLower("## " + heading)
	lines := strings.Split(body, "\n")
	var builder strings.Builder
	inSection := false

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if !inSection {
			if strings.ToLower(line) == target {
				inSection = true
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			break
		}
		builder.WriteString(rawLine)
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}
