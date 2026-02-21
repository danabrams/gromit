package runner

import (
	"bufio"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunnerSplitVerificationReclassified_LineBudgets verifies that the primary
// runner source files stay within their line-count budgets after subpackage
// extractions. This is the unit-level reclassification of the acceptance-level
// line-budget verification that checked runner.go and process.go size.
func TestRunnerSplitVerificationReclassified_LineBudgets(t *testing.T) {
	budgets := []struct {
		file  string
		limit int
	}{
		{"runner.go", 600},
		{"process.go", 700},
	}

	for _, b := range budgets {
		t.Run(b.file, func(t *testing.T) {
			f, err := os.Open(filepath.Join(b.file))
			if err != nil {
				t.Fatalf("open %s: %v", b.file, err)
			}
			defer func() { _ = f.Close() }()

			lines := 0
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				lines++
			}
			if scanErr := scanner.Err(); scanErr != nil {
				t.Fatalf("scan %s: %v", b.file, scanErr)
			}

			if lines > b.limit {
				t.Errorf("%s has %d lines, want <= %d — "+
					"further subpackage extraction should reduce this",
					b.file, lines, b.limit)
			}
		})
	}
}

// TestRunnerSplitVerificationReclassified_ImportIsolation verifies that the
// runner subpackages (escalation, validation, methodology, reviewpkg) do not
// import the parent runner facade package. This prevents circular dependencies
// and keeps the subpackages independently testable. This is the unit-level
// reclassification of the acceptance-level import isolation check.
func TestRunnerSplitVerificationReclassified_ImportIsolation(t *testing.T) {
	// Sub-packages that must NOT import the parent runner facade.
	subPackageDirs := []string{
		filepath.Join("escalation"),
		filepath.Join("validation"),
		filepath.Join("methodology"),
		filepath.Join("reviewpkg"),
	}

	// The import path prefix that must not appear in these sub-packages.
	const forbiddenImport = "github.com/danabrams/gromit/internal/runner\""

	for _, dir := range subPackageDirs {
		t.Run(dir, func(t *testing.T) {
			fset := token.NewFileSet()
			pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
				return !strings.HasSuffix(fi.Name(), "_test.go")
			}, 0)
			if err != nil {
				t.Fatalf("parse dir %s: %v", dir, err)
			}
			for _, pkg := range pkgs {
				for fileName, file := range pkg.Files {
					for _, imp := range file.Imports {
						path := imp.Path.Value
						if strings.Contains(path, forbiddenImport) ||
							path == "\"github.com/danabrams/gromit/internal/runner\"" {
							t.Errorf("%s imports the runner facade package %s — "+
								"subpackages must not import their parent facade to "+
								"preserve import isolation",
								fileName, path)
						}
					}
				}
			}
		})
	}
}
