package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunnerCleanup verifies the acceptance criteria for the
// "Reclassify and consolidate internal/runner/ acceptance tests" task.
func TestRunnerCleanup(t *testing.T) {
	t.Run("no runner acceptance test files exist", func(t *testing.T) {
		deletedFiles := []string{
			"runner_label_filtering_acceptance_test.go",
			"bead_client_interface_acceptance_test.go",
			"interfaces_label_methods_acceptance_test.go",
			"bead_client_label_methods_acceptance_test.go",
			"label_filtering_acceptance_test.go",
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

	t.Run("no wildcard acceptance files exist", func(t *testing.T) {
		matches, err := filepath.Glob("*_acceptance_test.go")
		if err != nil {
			t.Fatalf("glob error: %v", err)
		}
		var unexpected []string
		for _, m := range matches {
			// Allow this cleanup test file itself
			if m == "runner_cleanup_acceptance_test.go" {
				continue
			}
			unexpected = append(unexpected, m)
		}
		if len(unexpected) > 0 {
			t.Errorf("found unexpected acceptance test files in internal/runner/: %v", unexpected)
		}
	})

	t.Run("setupLabelFilterTest helper exists", func(t *testing.T) {
		// The helper should be in one of the label-related test files.
		candidates := []string{
			"label_filter_test.go",
			"runner_labels_test.go",
		}

		var found bool
		for _, filename := range candidates {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
			if err != nil {
				continue // File might not exist or have parse errors
			}

			for _, decl := range node.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if fn.Name.Name == "setupLabelFilterTest" {
					found = true

					// Verify it accepts *testing.T
					if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
						t.Error("setupLabelFilterTest should accept *testing.T parameter")
					}

					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			t.Error("setupLabelFilterTest helper function not found in any runner test file")
		}
	})

	t.Run("label filtering tests are table-driven", func(t *testing.T) {
		// The consolidated label filtering tests should use table-driven
		// test pattern. Check the candidate files for this pattern.
		candidates := []string{
			"label_filter_test.go",
			"runner_labels_test.go",
		}

		var foundTableDriven bool
		for _, filename := range candidates {
			src, err := os.ReadFile(filename)
			if err != nil {
				continue
			}

			content := string(src)

			// Table-driven tests use []struct{ with t.Run() inside a range loop
			hasTable := strings.Contains(content, "[]struct{") || strings.Contains(content, "[]struct {")
			hasSubtests := strings.Contains(content, "t.Run(")
			hasLabelFiltering := strings.Contains(content, "labelFilter") || strings.Contains(content, "LabelFilter") ||
				strings.Contains(content, "ReadyWithLabel")

			if hasTable && hasSubtests && hasLabelFiltering {
				foundTableDriven = true
				break
			}
		}

		if !foundTableDriven {
			t.Error("label filtering tests should use table-driven test pattern ([]struct{} + t.Run)")
		}
	})

	t.Run("label filtering tests are under 400 lines", func(t *testing.T) {
		// After consolidation, the label filtering test file should be concise.
		// Check both candidate destinations.
		candidates := []string{
			"label_filter_test.go",
			"runner_labels_test.go",
		}

		for _, filename := range candidates {
			src, err := os.ReadFile(filename)
			if err != nil {
				continue
			}

			lineCount := strings.Count(string(src), "\n") + 1
			if lineCount > 400 {
				t.Errorf("%s is %d lines, expected under 400 lines after table-driven consolidation", filename, lineCount)
			}
		}
	})

	t.Run("merged tests cover key label filtering behaviors", func(t *testing.T) {
		// Read all test files that might contain the consolidated tests
		candidates := []string{
			"label_filter_test.go",
			"runner_labels_test.go",
		}

		var allContent strings.Builder
		for _, filename := range candidates {
			src, err := os.ReadFile(filename)
			if err != nil {
				continue
			}
			allContent.Write(src)
		}

		content := allContent.String()

		// Key behaviors that must be preserved from the 5 deleted acceptance files
		behaviors := []struct {
			name  string
			check func(string) bool
		}{
			{
				name: "no-label-filter uses Ready()",
				check: func(s string) bool {
					return strings.Contains(s, "NoFilter") || strings.Contains(s, "NoLabels") ||
						(strings.Contains(s, "Ready()") && strings.Contains(s, "no label"))
				},
			},
			{
				name: "single label filter uses ReadyWithLabel",
				check: func(s string) bool {
					return strings.Contains(s, "SingleLabel") || strings.Contains(s, "single label")
				},
			},
			{
				name: "multiple label filters collect candidates",
				check: func(s string) bool {
					return strings.Contains(s, "MultipleLabel") || strings.Contains(s, "multiple label")
				},
			},
			{
				name: "priority ordering across labels",
				check: func(s string) bool {
					return strings.Contains(s, "Priority") || strings.Contains(s, "priority")
				},
			},
			{
				name: "deduplication of same bead from multiple labels",
				check: func(s string) bool {
					return strings.Contains(s, "Dedup") || strings.Contains(s, "dedup") ||
						strings.Contains(s, "same bead")
				},
			},
			{
				name: "no matching beads returns nil",
				check: func(s string) bool {
					return strings.Contains(s, "NoMatch") || strings.Contains(s, "NoneHave") ||
						strings.Contains(s, "AllLabelsEmpty") || strings.Contains(s, "no beads") ||
						strings.Contains(s, "none have beads")
				},
			},
		}

		for _, b := range behaviors {
			t.Run(b.name, func(t *testing.T) {
				if !b.check(content) {
					t.Errorf("consolidated test files should cover behavior: %s", b.name)
				}
			})
		}
	})

	t.Run("test files parse without errors", func(t *testing.T) {
		// Verify that key test files in the package parse correctly.
		// This catches syntax errors that might be introduced during consolidation.
		candidates := []string{
			"label_filter_test.go",
			"runner_labels_test.go",
		}

		for _, filename := range candidates {
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				continue // File doesn't exist, that's fine
			}

			fset := token.NewFileSet()
			_, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
			if err != nil {
				t.Errorf("%s has parse errors: %v", filename, err)
			}
		}
	})
}
