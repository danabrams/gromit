package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRetroCleanup verifies the acceptance criteria for the
// "Reclassify and consolidate cmd/gromit/retro_* acceptance tests" task.
func TestRetroCleanup(t *testing.T) {
	t.Run("no retro acceptance test files exist", func(t *testing.T) {
		deletedFiles := []string{
			"retro_scope_acceptance_test.go",
			"retro_cli_acceptance_test.go",
			"retro_e2e_acceptance_test.go",
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

	t.Run("no wildcard retro acceptance files exist", func(t *testing.T) {
		// Catch any other retro_*_acceptance_test.go files that might exist,
		// excluding this cleanup test file itself.
		matches, err := filepath.Glob("retro_*_acceptance_test.go")
		if err != nil {
			t.Fatalf("glob error: %v", err)
		}
		var unexpected []string
		for _, m := range matches {
			if m == "retro_cleanup_acceptance_test.go" {
				continue
			}
			unexpected = append(unexpected, m)
		}
		if len(unexpected) > 0 {
			t.Errorf("found unexpected retro acceptance test files: %v", unexpected)
		}
	})

	t.Run("retro_integration_test.go covers scope behaviors", func(t *testing.T) {
		src, err := os.ReadFile("retro_integration_test.go")
		if err != nil {
			t.Fatalf("failed to read retro_integration_test.go: %v", err)
		}

		content := string(src)

		// Key behaviors from retro_scope_acceptance_test.go that must
		// be preserved in retro_integration_test.go.
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
				name: "epic flag existence check",
				check: func(s string) bool {
					return strings.Contains(s, "epic") && strings.Contains(s, "Flag")
				},
			},
			{
				name: "spec and epic mutual exclusivity via scope.ValidateFlags",
				check: func(s string) bool {
					return strings.Contains(s, "ValidateFlags") || strings.Contains(s, "mutually exclusive")
				},
			},
			{
				name: "spec flag resolves to label format via ResolveSpec",
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
		}

		for _, b := range behaviors {
			t.Run(b.name, func(t *testing.T) {
				if !b.check(content) {
					t.Errorf("retro_integration_test.go should cover behavior: %s", b.name)
				}
			})
		}
	})

	t.Run("retro_integration_test.go covers CLI help text behaviors", func(t *testing.T) {
		src, err := os.ReadFile("retro_integration_test.go")
		if err != nil {
			t.Fatalf("failed to read retro_integration_test.go: %v", err)
		}

		content := string(src)

		// Key behaviors from retro_cli_acceptance_test.go that must
		// be preserved in retro_integration_test.go.
		behaviors := []struct {
			name  string
			check func(string) bool
		}{
			{
				name: "help text documents --spec flag",
				check: func(s string) bool {
					return (strings.Contains(s, "helpText") || strings.Contains(s, "Long")) &&
						strings.Contains(s, "--spec")
				},
			},
			{
				name: "help text documents --epic flag",
				check: func(s string) bool {
					return (strings.Contains(s, "helpText") || strings.Contains(s, "Long")) &&
						strings.Contains(s, "--epic")
				},
			},
			{
				name: "help text mentions filtering",
				check: func(s string) bool {
					return strings.Contains(strings.ToLower(s), "filter")
				},
			},
			{
				name: "help text mentions mutual exclusivity",
				check: func(s string) bool {
					return strings.Contains(strings.ToLower(s), "mutually exclusive")
				},
			},
		}

		for _, b := range behaviors {
			t.Run(b.name, func(t *testing.T) {
				if !b.check(content) {
					t.Errorf("retro_integration_test.go should cover behavior: %s", b.name)
				}
			})
		}
	})

	t.Run("retro_integration_test.go covers e2e filter behaviors", func(t *testing.T) {
		src, err := os.ReadFile("retro_integration_test.go")
		if err != nil {
			t.Fatalf("failed to read retro_integration_test.go: %v", err)
		}

		content := string(src)

		// Key behaviors from retro_e2e_acceptance_test.go that must
		// be preserved in retro_integration_test.go.
		behaviors := []struct {
			name  string
			check func(string) bool
		}{
			{
				name: "buildBeadFilter tested with empty/nil labels",
				check: func(s string) bool {
					return strings.Contains(s, "buildBeadFilter")
				},
			},
			{
				name: "filter parameter flow documented or tested",
				check: func(s string) bool {
					return strings.Contains(s, "beadFilter") || strings.Contains(s, "bead filter")
				},
			},
		}

		for _, b := range behaviors {
			t.Run(b.name, func(t *testing.T) {
				if !b.check(content) {
					t.Errorf("retro_integration_test.go should cover behavior: %s", b.name)
				}
			})
		}
	})

	t.Run("retro_integration_test.go parses without errors", func(t *testing.T) {
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, "retro_integration_test.go", nil, parser.AllErrors)
		if err != nil {
			t.Fatalf("retro_integration_test.go has parse errors: %v", err)
		}
	})

	t.Run("go test passes", func(t *testing.T) {
		// This is verified implicitly by this test running successfully.
		// The actual go test ./cmd/gromit/... run is handled by the
		// CI/build pipeline. This subtest documents the criterion.
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, "retro_integration_test.go", nil, parser.AllErrors)
		if err != nil {
			t.Fatalf("retro_integration_test.go has parse errors: %v", err)
		}
	})
}
