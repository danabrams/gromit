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

// TestExploreCleanup_NoAcceptanceTestFilesExist verifies that all 5
// explore_*_acceptance_test.go files have been deleted as part of the cleanup.
func TestExploreCleanup_NoAcceptanceTestFilesExist(t *testing.T) {
	deletedFiles := []string{
		"explore_acceptance_test.go",
		"explore_prompt_acceptance_test.go",
		"explore_session_acceptance_test.go",
		"explore_command_acceptance_test.go",
		"explore_integration_acceptance_test.go",
	}

	for _, filename := range deletedFiles {
		path := filepath.Join(".", filename)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("file %s should have been deleted during cleanup", filename)
		} else if !os.IsNotExist(err) {
			t.Errorf("unexpected error checking for %s: %v", filename, err)
		}
	}
}

// TestExploreCleanup_SetupHelperExists verifies that explore_test.go contains
// a setupExploreTest helper function that returns config and gromitDir.
func TestExploreCleanup_SetupHelperExists(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "explore_test.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse explore_test.go: %v", err)
	}

	var found bool
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == "setupExploreTest" {
			found = true

			// Verify it accepts *testing.T
			if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
				t.Error("setupExploreTest should accept *testing.T parameter")
			}

			// Verify it returns at least two values (config + gromitDir)
			if fn.Type.Results == nil || len(fn.Type.Results.List) < 2 {
				t.Error("setupExploreTest should return at least two values (config, gromitDir)")
			}

			break
		}
	}

	if !found {
		t.Error("setupExploreTest helper function not found in explore_test.go")
	}
}

// TestExploreCleanup_BuildExplorePromptTestsAreTableDriven verifies that
// buildExplorePrompt tests in explore_test.go use table-driven test pattern.
func TestExploreCleanup_BuildExplorePromptTestsAreTableDriven(t *testing.T) {
	src, err := os.ReadFile("explore_test.go")
	if err != nil {
		t.Fatalf("failed to read explore_test.go: %v", err)
	}

	content := string(src)

	// Verify there's a table-driven test for buildExplorePrompt.
	// Table-driven tests in Go use []struct{ with t.Run() inside a range loop.
	hasBuildPromptTest := strings.Contains(content, "buildExplorePrompt") &&
		strings.Contains(content, "t.Run(")

	if !hasBuildPromptTest {
		t.Error("explore_test.go should contain a table-driven test for buildExplorePrompt using t.Run()")
	}

	// Verify the table-driven pattern: struct slice + range loop
	hasTablePattern := strings.Contains(content, "[]struct{") || strings.Contains(content, "[]struct {")
	if !hasTablePattern {
		t.Error("explore_test.go should use table-driven test pattern ([]struct{...})")
	}

	// Count separate test functions that test buildExplorePrompt.
	// After cleanup, there should be at most 2 top-level test functions
	// (consolidated into table-driven), not the 7+ that existed across
	// the acceptance test files.
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "explore_test.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse explore_test.go: %v", err)
	}

	buildPromptTestCount := 0
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fn.Name.Name
		if strings.HasPrefix(name, "Test") &&
			(strings.Contains(name, "BuildPrompt") ||
				strings.Contains(name, "BuildsPrompt") ||
				(strings.Contains(name, "Prompt") && strings.Contains(name, "Build"))) {
			buildPromptTestCount++
		}
	}

	if buildPromptTestCount > 2 {
		t.Errorf("expected at most 2 top-level test functions for buildExplorePrompt (table-driven consolidation), got %d", buildPromptTestCount)
	}
}

// TestExploreCleanup_MergedTestsCoverKeyBehaviors verifies that explore_test.go
// contains tests covering the key behaviors from the deleted acceptance files.
func TestExploreCleanup_MergedTestsCoverKeyBehaviors(t *testing.T) {
	src, err := os.ReadFile("explore_test.go")
	if err != nil {
		t.Fatalf("failed to read explore_test.go: %v", err)
	}

	content := string(src)

	// The key behaviors that must still be tested after merging from the
	// deleted acceptance files. These are verified by checking that the
	// source code references the relevant concepts.
	behaviors := []struct {
		name  string
		check string
	}{
		{
			name:  "buildExplorePrompt is tested",
			check: "buildExplorePrompt",
		},
		{
			name:  "prompt includes project context (CLAUDE.md)",
			check: "CLAUDE.md",
		},
		{
			name:  "prompt handles missing files gracefully",
			check: "missing",
		},
	}

	for _, b := range behaviors {
		t.Run(b.name, func(t *testing.T) {
			if !strings.Contains(content, b.check) {
				t.Errorf("explore_test.go should contain tests covering %q (looking for %q)", b.name, b.check)
			}
		})
	}
}
