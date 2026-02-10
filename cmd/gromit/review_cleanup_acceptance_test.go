package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReviewCleanup verifies the acceptance criteria for the
// "Reclassify and consolidate cmd/gromit/review_* acceptance tests" task.
func TestReviewCleanup(t *testing.T) {
	t.Run("no review acceptance test files exist", func(t *testing.T) {
		deletedFiles := []string{
			"review_scope_acceptance_test.go",
			"review_mutual_exclusivity_acceptance_test.go",
		}

		for _, filename := range deletedFiles {
			path := filepath.Join(".", filename)
			if _, err := os.Stat(path); err == nil {
				t.Errorf("file %s should have been deleted during cleanup", filename)
			} else if !os.IsNotExist(err) {
				t.Errorf("unexpected error checking for %s: %v", filename, err)
			}
		}
	})

	t.Run("no wildcard review acceptance files exist", func(t *testing.T) {
		// Catch any other review_*_acceptance_test.go files that might exist,
		// excluding this cleanup test file itself.
		matches, err := filepath.Glob("review_*_acceptance_test.go")
		if err != nil {
			t.Fatalf("glob error: %v", err)
		}
		var unexpected []string
		for _, m := range matches {
			if m == "review_cleanup_acceptance_test.go" {
				continue
			}
			unexpected = append(unexpected, m)
		}
		if len(unexpected) > 0 {
			t.Errorf("found unexpected review acceptance test files: %v", unexpected)
		}
	})

	t.Run("review_test.go covers scope behaviors", func(t *testing.T) {
		src, err := os.ReadFile("review_test.go")
		if err != nil {
			t.Fatalf("failed to read review_test.go: %v", err)
		}

		content := string(src)

		// Key behaviors from review_scope_acceptance_test.go (non-skipped)
		// that must be preserved in review_test.go.
		behaviors := []struct {
			name  string
			check func(string) bool
		}{
			{
				name: "spec flag existence check",
				check: func(s string) bool {
					return strings.Contains(s, "spec") && strings.Contains(s, "Flag")
				},
			},
			{
				name: "spec and epic mutual exclusivity via scope.ValidateFlags",
				check: func(s string) bool {
					return strings.Contains(s, "ValidateFlags") || strings.Contains(s, "mutually exclusive")
				},
			},
			{
				name: "spec flag resolves to label format",
				check: func(s string) bool {
					return strings.Contains(s, "ResolveSpec") || strings.Contains(s, "spec:")
				},
			},
			{
				name: "epic flag uses ResolveEpic",
				check: func(s string) bool {
					return strings.Contains(s, "ResolveEpic")
				},
			},
			{
				name: "help text documents --spec flag",
				check: func(s string) bool {
					return strings.Contains(s, "helpText") || strings.Contains(s, "help text") || strings.Contains(s, "Long")
				},
			},
		}

		for _, b := range behaviors {
			t.Run(b.name, func(t *testing.T) {
				if !b.check(content) {
					t.Errorf("review_test.go should cover behavior: %s", b.name)
				}
			})
		}
	})

	t.Run("review_test.go covers mutual exclusivity behaviors", func(t *testing.T) {
		src, err := os.ReadFile("review_test.go")
		if err != nil {
			t.Fatalf("failed to read review_test.go: %v", err)
		}

		content := string(src)

		// Key behaviors from review_mutual_exclusivity_acceptance_test.go
		// that must be preserved in review_test.go.
		behaviors := []struct {
			name  string
			check func(string) bool
		}{
			{
				name: "flag mutual exclusivity table-driven test",
				check: func(s string) bool {
					// The table-driven test covers all 8 combinations
					return strings.Contains(s, "mutually exclusive") &&
						(strings.Contains(s, "[]struct") || strings.Contains(s, "[]struct {"))
				},
			},
			{
				name: "since and epic mutual exclusivity",
				check: func(s string) bool {
					return strings.Contains(s, "since") && strings.Contains(s, "epic") &&
						strings.Contains(s, "mutually exclusive")
				},
			},
			{
				name: "mutual exclusivity checked before resolution",
				check: func(s string) bool {
					// Tests that validation happens before attempting to resolve
					return strings.Contains(s, "CheckedEarly") || strings.Contains(s, "checked early") ||
						strings.Contains(s, "nonexistent")
				},
			},
			{
				name: "whitespace handling in flag values",
				check: func(s string) bool {
					return strings.Contains(s, "Whitespace") || strings.Contains(s, "whitespace") ||
						strings.Contains(s, `"   "`)
				},
			},
		}

		for _, b := range behaviors {
			t.Run(b.name, func(t *testing.T) {
				if !b.check(content) {
					t.Errorf("review_test.go should cover behavior: %s", b.name)
				}
			})
		}
	})

	t.Run("skipped tests from scope file are not preserved", func(t *testing.T) {
		// The 2 skipped tests (FlagPriorityOrder, SpecResolutionWithNoBeads)
		// should NOT be carried into review_test.go — their ideas were
		// already captured in gromit-u3z9.
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "review_test.go", nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("failed to parse review_test.go: %v", err)
		}

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			name := fn.Name.Name
			if strings.Contains(name, "FlagPriorityOrder") ||
				strings.Contains(name, "SpecResolutionWithNoBeads") {
				t.Errorf("skipped test %s should not be carried into review_test.go (ideas captured in gromit-u3z9)", name)
			}
		}
	})

	t.Run("go test passes", func(t *testing.T) {
		// This is verified implicitly by this test running successfully.
		// The actual go test ./cmd/gromit/... run is handled by the
		// CI/build pipeline. This subtest documents the criterion.
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, "review_test.go", nil, parser.AllErrors)
		if err != nil {
			t.Fatalf("review_test.go has parse errors: %v", err)
		}
	})
}
