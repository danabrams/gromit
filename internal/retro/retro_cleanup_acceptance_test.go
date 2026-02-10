package retro

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestRetroHashEvictionCleanup verifies the acceptance criteria for the
// "Clean up internal/retro/filtered_hash_eviction_acceptance_test.go" task.
func TestRetroHashEvictionCleanup(t *testing.T) {
	const targetFile = "filtered_hash_eviction_acceptance_test.go"

	t.Run("no t.Skip tests remain", func(t *testing.T) {
		src, err := os.ReadFile(targetFile)
		if err != nil {
			t.Fatalf("cannot read %s: %v", targetFile, err)
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, targetFile, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse error in %s: %v", targetFile, err)
		}

		// Walk the AST looking for t.Skip calls inside test functions
		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if sel.Sel.Name == "Skip" || sel.Sel.Name == "Skipf" {
					t.Errorf("test %s contains t.Skip() — skipped tests should be deleted", fn.Name.Name)
				}
				return true
			})
		}
	})

	t.Run("skipped tests are deleted", func(t *testing.T) {
		src, err := os.ReadFile(targetFile)
		if err != nil {
			t.Fatalf("cannot read %s: %v", targetFile, err)
		}

		content := string(src)

		deletedTests := []string{
			"TestFilteredHashEviction_SingleSaveWhenBothAddAndPrune",
			"TestFilteredHashEviction_HandlesArchivedLearnings",
		}
		for _, name := range deletedTests {
			if strings.Contains(content, name) {
				t.Errorf("test %s should have been deleted (was skipped)", name)
			}
		}
	})

	t.Run("setup helper extracted for remaining tests", func(t *testing.T) {
		// Look for a setup helper function in the acceptance test file or
		// co-located test files in the retro package.
		candidates := []string{
			targetFile,
			"retro_test.go",
		}

		var found bool
		for _, filename := range candidates {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
			if err != nil {
				continue
			}

			for _, decl := range node.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				// Look for a helper function that sets up hash eviction test state.
				// It should start with "setup" and accept *testing.T.
				name := fn.Name.Name
				if strings.HasPrefix(name, "setup") && strings.Contains(strings.ToLower(name), "hash") ||
					strings.HasPrefix(name, "setup") && strings.Contains(strings.ToLower(name), "eviction") ||
					strings.HasPrefix(name, "setup") && strings.Contains(strings.ToLower(name), "filtered") {
					// Verify it accepts *testing.T
					if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}

		if !found {
			t.Error("setup helper function (e.g., setupFilteredHashEvictionTest) not found — " +
				"remaining tests should share a common setup helper")
		}
	})

	t.Run("build tag present", func(t *testing.T) {
		src, err := os.ReadFile(targetFile)
		if err != nil {
			t.Fatalf("cannot read %s: %v", targetFile, err)
		}

		content := string(src)
		if !strings.HasPrefix(content, "//go:build acceptance") {
			t.Error("file should have //go:build acceptance tag as the first line")
		}
	})

	t.Run("remaining tests preserved", func(t *testing.T) {
		src, err := os.ReadFile(targetFile)
		if err != nil {
			t.Fatalf("cannot read %s: %v", targetFile, err)
		}

		content := string(src)

		// These tests should still exist (they are non-skipped and test real behavior)
		preservedTests := []string{
			"TestFilteredHashEviction_RemovesStaleHashesAfterRetroRun",
			"TestFilteredHashEviction_NoSaveWhenNoChanges",
			"TestFilteredHashEviction_HandlesEmptyProvisionalLearnings",
		}
		for _, name := range preservedTests {
			if !strings.Contains(content, name) {
				t.Errorf("test %s should be preserved — it tests real behavior", name)
			}
		}
	})

	t.Run("file parses without errors", func(t *testing.T) {
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, targetFile, nil, parser.AllErrors)
		if err != nil {
			t.Errorf("%s has parse errors: %v", targetFile, err)
		}
	})
}
