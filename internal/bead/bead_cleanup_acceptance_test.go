package bead

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBeadAcceptanceTestCleanup verifies the acceptance criteria for the
// "Reclassify internal/bead/ acceptance tests" task.
func TestBeadAcceptanceTestCleanup(t *testing.T) {
	t.Run("no acceptance test files exist in internal/bead", func(t *testing.T) {
		matches, err := filepath.Glob("*_acceptance_test.go")
		if err != nil {
			t.Fatalf("glob error: %v", err)
		}
		var unexpected []string
		for _, m := range matches {
			// Allow this cleanup test file itself
			if m == "bead_cleanup_acceptance_test.go" {
				continue
			}
			unexpected = append(unexpected, m)
		}
		if len(unexpected) > 0 {
			t.Errorf("found unexpected acceptance test files in internal/bead/: %v", unexpected)
		}
	})

	t.Run("label_methods_acceptance_test.go is deleted", func(t *testing.T) {
		if _, err := os.Stat("label_methods_acceptance_test.go"); err == nil {
			t.Error("label_methods_acceptance_test.go should have been deleted during cleanup")
		} else if !os.IsNotExist(err) {
			t.Errorf("unexpected error checking for label_methods_acceptance_test.go: %v", err)
		}
	})

	t.Run("BD_AVAILABLE-gated contract tests are preserved", func(t *testing.T) {
		// The BD_AVAILABLE-gated contract tests from label_methods_acceptance_test.go
		// should have been moved into existing test files.
		// Check that contract test patterns exist somewhere in the bead test files.
		candidates := []string{
			"bead_test.go",
			"ready_with_label_test.go",
			"list_with_label_test.go",
			"label_integration_test.go",
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

		// Verify BD_AVAILABLE-gated contract tests are preserved
		if !strings.Contains(content, "BD_AVAILABLE") {
			t.Error("BD_AVAILABLE-gated contract tests should be preserved in bead test files")
		}
	})

	t.Run("ReadyWithLabel validation tests are preserved", func(t *testing.T) {
		candidates := []string{
			"bead_test.go",
			"ready_with_label_test.go",
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

		// Key behaviors from the acceptance file that must be preserved
		behaviors := []struct {
			name  string
			check func(string) bool
		}{
			{
				name: "ReadyWithLabel rejects empty labels",
				check: func(s string) bool {
					return strings.Contains(s, "ReadyWithLabel") && strings.Contains(s, "empty")
				},
			},
			{
				name: "ReadyWithLabel validates shell metacharacters",
				check: func(s string) bool {
					return strings.Contains(s, "ReadyWithLabel") &&
						(strings.Contains(s, "metacharacter") || strings.Contains(s, "invalid"))
				},
			},
		}

		for _, b := range behaviors {
			t.Run(b.name, func(t *testing.T) {
				if !b.check(content) {
					t.Errorf("test files should cover behavior: %s", b.name)
				}
			})
		}
	})

	t.Run("ListWithLabel validation tests are preserved", func(t *testing.T) {
		candidates := []string{
			"bead_test.go",
			"list_with_label_test.go",
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

		behaviors := []struct {
			name  string
			check func(string) bool
		}{
			{
				name: "ListWithLabel rejects empty labels",
				check: func(s string) bool {
					return strings.Contains(s, "ListWithLabel") && strings.Contains(s, "empty")
				},
			},
			{
				name: "ListWithLabel validates shell metacharacters",
				check: func(s string) bool {
					return strings.Contains(s, "ListWithLabel") &&
						(strings.Contains(s, "metacharacter") || strings.Contains(s, "invalid"))
				},
			},
		}

		for _, b := range behaviors {
			t.Run(b.name, func(t *testing.T) {
				if !b.check(content) {
					t.Errorf("test files should cover behavior: %s", b.name)
				}
			})
		}
	})

	t.Run("test files parse without errors", func(t *testing.T) {
		candidates := []string{
			"bead_test.go",
			"ready_with_label_test.go",
			"list_with_label_test.go",
			"label_integration_test.go",
		}

		for _, filename := range candidates {
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				continue
			}

			src, err := os.ReadFile(filename)
			if err != nil {
				t.Errorf("cannot read %s: %v", filename, err)
				continue
			}

			// Basic check: file should contain package declaration
			if !strings.Contains(string(src), "package bead") {
				t.Errorf("%s should declare package bead", filename)
			}
		}
	})
}
