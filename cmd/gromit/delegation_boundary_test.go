package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// allowedCommandInternalImports captures the set of internal packages that
// the cmd/gromit command files currently import. Any new internal dependency
// must be explicitly added here so that this guard test can fail and prompt a
// design review before business logic resurfaces in the CLI layer.
var allowedCommandInternalImports = map[string]struct{}{
	"github.com/danabrams/gromit/internal/agent":            {},
	"github.com/danabrams/gromit/internal/backlog":          {},
	"github.com/danabrams/gromit/internal/bead":             {},
	"github.com/danabrams/gromit/internal/benchmark":        {},
	"github.com/danabrams/gromit/internal/claude":           {},
	"github.com/danabrams/gromit/internal/config":           {},
	"github.com/danabrams/gromit/internal/experiment":       {},
	"github.com/danabrams/gromit/internal/frontmatter":      {},
	"github.com/danabrams/gromit/internal/learnings":        {},
	"github.com/danabrams/gromit/internal/logger":           {},
	"github.com/danabrams/gromit/internal/integrationqueue": {},
	"github.com/danabrams/gromit/internal/pipeline":         {},
	"github.com/danabrams/gromit/internal/procutil":         {},
	"github.com/danabrams/gromit/internal/prompt":           {},
	"github.com/danabrams/gromit/internal/provider":         {},
	"github.com/danabrams/gromit/internal/queue":            {},
	"github.com/danabrams/gromit/internal/retro":            {},
	"github.com/danabrams/gromit/internal/review":           {},
	"github.com/danabrams/gromit/internal/runbook":          {},
	"github.com/danabrams/gromit/internal/runner":           {},
	"github.com/danabrams/gromit/internal/runner/specmerge": {},
	"github.com/danabrams/gromit/internal/scope":            {},
	"github.com/danabrams/gromit/internal/specgate":         {},
	"github.com/danabrams/gromit/internal/state":            {},
	"github.com/danabrams/gromit/internal/tracker":          {},
	"github.com/danabrams/gromit/internal/tui":              {},
	"github.com/danabrams/gromit/internal/visionmetrics":    {},
	"github.com/danabrams/gromit/internal/worktree":         {},
}

func assertCommandFilesOnlyImportAllowedInternalPackages(t *testing.T) {
	t.Helper()
	violations, err := findForbiddenCommandInternalImports()
	if err != nil {
		t.Fatalf("collect command imports: %v", err)
	}
	if len(violations) == 0 {
		return
	}
	var details []string
	for _, path := range sortedFilePaths(violations) {
		imports := violations[path]
		if len(imports) == 0 {
			continue
		}
		sort.Strings(imports)
		details = append(details, fmt.Sprintf("%s imports %s", path, strings.Join(imports, ", ")))
	}
	if len(details) > 0 {
		t.Fatalf("forbidden internal imports found:\n%s", strings.Join(details, "\n"))
	}
}

func findForbiddenCommandInternalImports() (map[string][]string, error) {
	files, err := commandSourceFiles()
	if err != nil {
		return nil, err
	}
	down := map[string][]string{}
	fs := token.NewFileSet()
	for _, path := range files {
		f, err := parser.ParseFile(fs, path, nil, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, imp := range f.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("unquote import in %s: %w", path, err)
			}
			if !strings.HasPrefix(importPath, "github.com/danabrams/gromit/internal/") {
				continue
			}
			if _, ok := allowedCommandInternalImports[importPath]; !ok {
				down[path] = append(down[path], importPath)
			}
		}
	}
	return down, nil
}

func commandSourceFiles() ([]string, error) {
	dir, err := commandSourceDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files, nil
}

func commandSourceDir() (string, error) {
	candidates := []string{"cmd/gromit", "."}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		if candidate == "." {
			if _, err := os.Stat(filepath.Join(candidate, "cli_contract_test.go")); err != nil {
				continue
			}
		}
		return candidate, nil
	}
	return "", fmt.Errorf("cmd/gromit directory not found")
}

func sortedFilePaths(m map[string][]string) []string {
	paths := make([]string, 0, len(m))
	for path := range m {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
