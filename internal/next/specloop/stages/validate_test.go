package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

type fakeValidator struct {
	result validator.FinalResult
	err    error
}

func (f *fakeValidator) RunFinal(ctx context.Context, alwaysRun []validator.Check, projectChecks []validator.Check, workDir string) (validator.FinalResult, error) {
	return f.result, f.err
}

type fakeContractEvaluator struct {
	failures []contract.ContractFailure
}

func (f *fakeContractEvaluator) Evaluate(_ context.Context, _ *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
	return f.failures, nil
}

// Verify ValidateStage satisfies the Stage interface.
var _ specloop.Stage = (*ValidateStage)(nil)

func TestValidateStage_AllPass_Continue(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: true,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "test", Pass: true}},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil, nil, nil)

	if stage.Name() != "validate" {
		t.Fatalf("expected name 'validate', got %q", stage.Name())
	}

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}
}

func TestValidateStage_Failure_ReplanFrom(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "test", Pass: false, Output: "FAIL"}},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 1
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be false")
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) == 0 {
		t.Fatal("expected failures to be non-empty")
	}
}

// TestValidateStage_MissingContractFile verifies that when EvidenceDir is set but
// scenario-contracts.yaml does not exist, the stage proceeds silently without error.
func TestValidateStage_MissingContractFile(t *testing.T) {
	dir := t.TempDir()

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{},
			ProjectChecks: validator.CheckResults{},
		},
	}
	evaluator := &fakeContractEvaluator{}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     "/tmp/work",
		EvidenceDir: dir, // dir exists but contains no scenario-contracts.yaml
	}, nil, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error when contract file missing: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue when contract file missing, got %v", action.Kind)
	}
}

// TestValidateStage_ContractFailures verifies that contract assertion failures are
// collected and reported with the format "contract:<scenario-name> — <assertion-type> failed: <details>".
func TestValidateStage_ContractFailures(t *testing.T) {
	dir := t.TempDir()

	// Write a minimal scenario-contracts.yaml to the evidence dir.
	contractYAML := `scenarios:
  - name: subtract-works
    assertions:
      - file_exists: result.txt
`
	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{},
			ProjectChecks: validator.CheckResults{},
		},
	}
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{ScenarioName: "subtract-works", AssertionType: "file_exists", Details: `file "result.txt" does not exist`},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     "/tmp/work",
		EvidenceDir: dir,
	}, nil, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom due to contract failure, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) == 0 {
		t.Fatal("expected failures to be non-empty")
	}
	want := `contract:subtract-works — file_exists failed: file "result.txt" does not exist`
	if action.Context.Failures[0] != want {
		t.Fatalf("expected failure %q, got %q", want, action.Context.Failures[0])
	}
}

// TestValidateStage_ContractAndShellFailures verifies that contract failures are collected
// first and then shell check failures are appended, both ending up in ReplanFrom failures.
func TestValidateStage_ContractAndShellFailures(t *testing.T) {
	dir := t.TempDir()

	contractYAML := `scenarios:
  - name: add-works
    assertions:
      - file_exists: out.txt
`
	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "test", Pass: false, Output: "FAIL"}},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{ScenarioName: "add-works", AssertionType: "file_exists", Details: `file "out.txt" does not exist`},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     "/tmp/work",
		EvidenceDir: dir,
	}, nil, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	// contract failure should be first
	if len(action.Context.Failures) < 2 {
		t.Fatalf("expected at least 2 failures (contract + shell), got %d", len(action.Context.Failures))
	}
	if !strings.HasPrefix(action.Context.Failures[0], "contract:") {
		t.Fatalf("expected first failure to be contract failure, got %q", action.Context.Failures[0])
	}
}

// TestFindSiblingFileWithPattern_FindsPatternInGoFile verifies that
// findSiblingFileWithPattern successfully finds a pattern in a sibling .go file.
func TestFindSiblingFileWithPattern_FindsPatternInGoFile(t *testing.T) {
	dir := t.TempDir()

	// Create two .go files in the same directory
	file1 := filepath.Join(dir, "file1.go")
	file2 := filepath.Join(dir, "file2.go")

	if err := os.WriteFile(file1, []byte("package main\nfunc foo() {}"), 0o644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("package main\nfunc bar() { myPattern }"), 0o644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	// Search for the pattern in sibling files
	foundPath := findSiblingFileWithPattern(dir, "file1.go", "myPattern", []string{".go"})

	if foundPath == "" {
		t.Fatal("expected to find pattern, but got empty path")
	}
	if foundPath != "file2.go" {
		t.Fatalf("expected to find file2.go, got %q", foundPath)
	}
}

// TestFindSiblingFileWithPattern_IgnoresNonGoFiles verifies that
// findSiblingFileWithPattern ignores non-.go files when extensions=[".go"].
func TestFindSiblingFileWithPattern_IgnoresNonGoFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a .go file without the pattern
	goFile := filepath.Join(dir, "code.go")
	if err := os.WriteFile(goFile, []byte("package main\nfunc hello() {}"), 0o644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	// Create non-.go files with the pattern
	txtFile := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("this file has mySearchPattern"), 0o644); err != nil {
		t.Fatalf("write txt file: %v", err)
	}

	yamlFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yamlFile, []byte("mySearchPattern: true"), 0o644); err != nil {
		t.Fatalf("write yaml file: %v", err)
	}

	// Search for the pattern that exists in non-.go files
	foundPath := findSiblingFileWithPattern(dir, "code.go", "mySearchPattern", []string{".go"})

	if foundPath != "" {
		t.Fatalf("expected empty path, got %q", foundPath)
	}
}

// TestFindSiblingFileWithPattern_PatternNotFound verifies that
// findSiblingFileWithPattern returns empty when the pattern is not found.
func TestFindSiblingFileWithPattern_PatternNotFound(t *testing.T) {
	dir := t.TempDir()

	// Create multiple .go files without the pattern
	file1 := filepath.Join(dir, "file1.go")
	file2 := filepath.Join(dir, "file2.go")
	file3 := filepath.Join(dir, "file3.go")

	if err := os.WriteFile(file1, []byte("package main\nfunc foo() {}"), 0o644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("package main\nfunc bar() {}"), 0o644); err != nil {
		t.Fatalf("write file2: %v", err)
	}
	if err := os.WriteFile(file3, []byte("package main\nfunc baz() {}"), 0o644); err != nil {
		t.Fatalf("write file3: %v", err)
	}

	// Search for a pattern that doesn't exist in any .go file
	foundPath := findSiblingFileWithPattern(dir, "file1.go", "nonexistentPattern", []string{".go"})

	if foundPath != "" {
		t.Fatalf("expected empty path, got %q", foundPath)
	}
}

// TestFindSiblingFileWithPattern_SkipsNonGoFiles verifies that findSiblingFileWithPattern
// only returns .go files and ignores non-.go files (e.g., .txt, .md) even when their
// names match the pattern. This tests the ExtFilter behavior of the function.
func TestFindSiblingFileWithPattern_SkipsNonGoFiles(t *testing.T) {
	// Create a work directory structure
	workDir := t.TempDir()
	pkgDir := filepath.Join(workDir, "internal", "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a .go file without the pattern
	goFile := filepath.Join(pkgDir, "implementation.go")
	if err := os.WriteFile(goFile, []byte("package pkg\nfunc init() {}"), 0o644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	// Create non-.go files with the pattern - these should be ignored
	txtFile := filepath.Join(pkgDir, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("myTargetPattern found here"), 0o644); err != nil {
		t.Fatalf("write txt file: %v", err)
	}

	mdFile := filepath.Join(pkgDir, "README.md")
	if err := os.WriteFile(mdFile, []byte("# My Documentation\nmyTargetPattern is described here"), 0o644); err != nil {
		t.Fatalf("write md file: %v", err)
	}

	// Create another non-.go file with matching extension
	jsonFile := filepath.Join(pkgDir, "config.json")
	if err := os.WriteFile(jsonFile, []byte(`{"key": "myTargetPattern"}`), 0o644); err != nil {
		t.Fatalf("write json file: %v", err)
	}

	// Search for the pattern that exists in non-.go files but not in .go files
	// Should return empty since pattern is not in any .go file
	foundPath := findSiblingFileWithPattern(workDir, "internal/pkg/implementation.go", "myTargetPattern", []string{".go"})

	if foundPath != "" {
		t.Fatalf("expected empty path when pattern only in non-.go files, got %q", foundPath)
	}
}

// TestFindSiblingFileWithPattern_OnlyGoFilesReturned verifies that findSiblingFileWithPattern
// returns .go files when the pattern is found, and completely ignores non-.go files
// even when they have matching names.
func TestFindSiblingFileWithPattern_OnlyGoFilesReturned(t *testing.T) {
	workDir := t.TempDir()
	pkgDir := filepath.Join(workDir, "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a .go file without the pattern
	goFile1 := filepath.Join(pkgDir, "types.go")
	if err := os.WriteFile(goFile1, []byte("package pkg\ntype Handler struct{}"), 0o644); err != nil {
		t.Fatalf("write types.go: %v", err)
	}

	// Create a .go file with the pattern
	goFile2 := filepath.Join(pkgDir, "handler.go")
	if err := os.WriteFile(goFile2, []byte("package pkg\nfunc (h Handler) ServeRequest() { TargetFunc() }"), 0o644); err != nil {
		t.Fatalf("write handler.go: %v", err)
	}

	// Create non-.go files that also match the pattern name (but shouldn't be returned)
	txtFile := filepath.Join(pkgDir, "handler.txt")
	if err := os.WriteFile(txtFile, []byte("TargetFunc description"), 0o644); err != nil {
		t.Fatalf("write handler.txt: %v", err)
	}

	mdFile := filepath.Join(pkgDir, "types.md")
	if err := os.WriteFile(mdFile, []byte("TargetFunc API docs"), 0o644); err != nil {
		t.Fatalf("write types.md: %v", err)
	}

	// Search for the pattern
	foundPath := findSiblingFileWithPattern(workDir, "pkg/types.go", "TargetFunc", []string{".go"})

	// Should find handler.go (the .go file with the pattern)
	if foundPath == "" {
		t.Fatal("expected to find pattern in .go file")
	}

	// Verify it returned a .go file, not the .txt or .md files
	if !strings.HasSuffix(foundPath, ".go") {
		t.Fatalf("expected returned file to have .go extension, got %q", foundPath)
	}

	if strings.Contains(foundPath, ".txt") || strings.Contains(foundPath, ".md") {
		t.Fatalf("expected to return .go file only, got %q which contains non-.go extension", foundPath)
	}
}
