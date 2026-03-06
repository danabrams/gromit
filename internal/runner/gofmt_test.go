package runner

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestGofmtCompliance(t *testing.T) {
	files := []string{
		"spc_auto_triage.go",
		"specmerge/pr_summary.go",
	}

	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Helper()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			formatted, err := format.Source(src)
			if err != nil {
				t.Fatalf("formatting %s: %v", path, err)
			}

			if !bytes.Equal(src, formatted) {
				t.Fatalf("%s is not gofmt-compliant", path)
			}
		})
	}
}

func TestRunnerCoreStdLibImportsCompact(t *testing.T) {
	files := []string{
		"runner.go",
		"orchestrator.go",
		"constructor.go",
	}
	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Helper()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			violations, err := detectStdLibImportBlankLines(path, src)
			if err != nil {
				t.Fatalf("parsing imports in %s: %v", path, err)
			}

			if len(violations) > 0 {
				t.Fatalf("stdlib imports separated by blank lines in %s:\n%s", path, strings.Join(violations, "\n"))
			}
		})
	}
}

func detectStdLibImportBlankLines(filePath string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	type entry struct {
		path  string
		start int
		end   int
		isStd bool
	}

	var entries []entry
	for _, spec := range file.Imports {
		start := int(fset.Position(spec.Pos()).Offset)
		end := int(fset.Position(spec.End()).Offset)
		pathValue, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			pathValue = spec.Path.Value
		}
		entries = append(entries, entry{
			path:  pathValue,
			start: start,
			end:   end,
			isStd: isStdImport(pathValue),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].start < entries[j].start
	})

	var violations []string
	for i := 1; i < len(entries); i++ {
		prev := entries[i-1]
		curr := entries[i]
		if !prev.isStd || !curr.isStd || prev.end >= curr.start {
			continue
		}
		between := src[prev.end:curr.start]
		if hasBlankLine(between) {
			violations = append(violations, fmt.Sprintf("%s vs %s", prev.path, curr.path))
		}
	}
	return violations, nil
}

func isStdImport(path string) bool {
	return path != "" && !strings.Contains(path, ".")
}

func hasBlankLine(data []byte) bool {
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	for i := 0; i < len(normalized); i++ {
		if normalized[i] != '\n' {
			continue
		}
		j := i + 1
		for j < len(normalized) && (normalized[j] == ' ' || normalized[j] == '\t' || normalized[j] == '\r') {
			j++
		}
		if j < len(normalized) && normalized[j] == '\n' {
			return true
		}
	}
	return false
}

