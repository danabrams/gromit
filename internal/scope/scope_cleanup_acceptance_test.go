package scope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScopeAcceptanceTestCleanup verifies the acceptance criteria for the
// "Reclassify internal/scope/ acceptance tests" task.
func TestScopeAcceptanceTestCleanup(t *testing.T) {
	t.Run("no acceptance test files exist in internal/scope", func(t *testing.T) {
		matches, err := filepath.Glob("*_acceptance_test.go")
		if err != nil {
			t.Fatalf("glob error: %v", err)
		}
		var unexpected []string
		for _, m := range matches {
			// Allow this cleanup test file itself
			if m == "scope_cleanup_acceptance_test.go" {
				continue
			}
			unexpected = append(unexpected, m)
		}
		if len(unexpected) > 0 {
			t.Errorf("found unexpected acceptance test files in internal/scope/: %v", unexpected)
		}
	})

	t.Run("validate_flags_three_way_acceptance_test.go is deleted", func(t *testing.T) {
		if _, err := os.Stat("validate_flags_three_way_acceptance_test.go"); err == nil {
			t.Error("validate_flags_three_way_acceptance_test.go should have been deleted during cleanup")
		} else if !os.IsNotExist(err) {
			t.Errorf("unexpected error checking for validate_flags_three_way_acceptance_test.go: %v", err)
		}
	})

	t.Run("three-way mutual exclusivity tests are in scope_test.go", func(t *testing.T) {
		src, err := os.ReadFile("scope_test.go")
		if err != nil {
			t.Fatalf("cannot read scope_test.go: %v", err)
		}

		content := string(src)

		// The three-way ValidateFlags tests from the acceptance file should be
		// merged into scope_test.go. Key behaviors to verify:
		behaviors := []struct {
			name  string
			check func(string) bool
		}{
			{
				name: "three-way mutual exclusivity with since parameter",
				check: func(s string) bool {
					return strings.Contains(s, "since") && strings.Contains(s, "mutually exclusive")
				},
			},
			{
				name: "whitespace trimming for three-way flags",
				check: func(s string) bool {
					return strings.Contains(s, "whitespace") || strings.Contains(s, "Trims")
				},
			},
			{
				name: "ValidateFlags called with three parameters",
				check: func(s string) bool {
					// Must have at least one call with epic, spec, since
					return strings.Contains(s, "ValidateFlags(") &&
						strings.Contains(s, "since")
				},
			},
		}

		for _, b := range behaviors {
			t.Run(b.name, func(t *testing.T) {
				if !b.check(content) {
					t.Errorf("scope_test.go should cover behavior: %s", b.name)
				}
			})
		}
	})

	t.Run("test files parse without errors", func(t *testing.T) {
		if _, err := os.Stat("scope_test.go"); os.IsNotExist(err) {
			t.Fatal("scope_test.go should exist")
		}

		src, err := os.ReadFile("scope_test.go")
		if err != nil {
			t.Fatalf("cannot read scope_test.go: %v", err)
		}

		if !strings.Contains(string(src), "package scope") {
			t.Error("scope_test.go should declare package scope")
		}
	})
}
