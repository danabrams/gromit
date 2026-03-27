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
	"github.com/danabrams/gromit/internal/next/testutil"
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

func newValidateWorkDir(t testing.TB) string {
	t.Helper()
	workDir := t.TempDir()
	testutil.WriteMinimalProjectFixtures(t, workDir)
	return workDir
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

	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: workDir}, nil, nil, nil)

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
		t.Fatal("expected FinalValidationPassed to be true when the validator reported success")
	}
}

func TestValidate_RegressionStillTriggersReplan(t *testing.T) {
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

	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: workDir}, nil, nil, nil)

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

func TestValidateStage_ResultPassFalseWithoutCheckFailures_Replans(t *testing.T) {
	workDir := newValidateWorkDir(t)

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          false,
			AlwaysRun:     validator.CheckResults{},
			ProjectChecks: validator.CheckResults{},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: workDir}, nil, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
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
	if rs.LastFinalValidation == nil || rs.LastFinalValidation.Pass {
		t.Fatalf("expected LastFinalValidation.Pass false, got %+v", rs.LastFinalValidation)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) == 0 {
		t.Fatal("expected failures to be non-empty")
	}
	if action.Context.Failures[0] != "validation failed" {
		t.Fatalf("unexpected failure message: %v", action.Context.Failures[0])
	}
}

func TestValidate_ContinueWhenAllBlockingFailuresBaselineExcluded(t *testing.T) {
	tmp := t.TempDir()
	testutil.WriteMinimalProjectFixtures(t, tmp)
	eventLog := runstore.NewEventLog(filepath.Join(tmp, "events.jsonl"))

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "baseline fail"},
				},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: tmp}, eventLog, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.BaselineFailures = map[string]string{"unit-tests": "baseline fail"}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true when all failures are baseline-excluded")
	}
	if rs.LastFinalValidation == nil || rs.LastFinalValidation.Pass {
		t.Fatalf("expected LastFinalValidation.Pass false, got %+v", rs.LastFinalValidation)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	found := false
	for _, ev := range events {
		if bfe, ok := ev.(*runstore.BaselineFailureExcludedEvent); ok {
			found = true
			if bfe.CheckName != "unit-tests" {
				t.Fatalf("baseline event check_name = %q, want unit-tests", bfe.CheckName)
			}
			if bfe.Output != "baseline fail" {
				t.Fatalf("baseline event output = %q, want baseline fail", bfe.Output)
			}
		}
	}
	if !found {
		t.Fatal("baseline_failure_excluded event not emitted")
	}
}

func TestValidate_MultipleBaselineExclusionsEmitMultipleEvents(t *testing.T) {
	tmp := t.TempDir()
	testutil.WriteMinimalProjectFixtures(t, tmp)
	eventLog := runstore.NewEventLog(filepath.Join(tmp, "events.jsonl"))

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "baseline output 1"},
					{Name: "lint", Pass: false, Output: "baseline output 2"},
				},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: tmp}, eventLog, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.BaselineFailures = map[string]string{
		"unit-tests": "baseline output 1",
		"lint":       "baseline output 2",
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	var exclusionEvents []*runstore.BaselineFailureExcludedEvent
	for _, ev := range events {
		if bfe, ok := ev.(*runstore.BaselineFailureExcludedEvent); ok {
			exclusionEvents = append(exclusionEvents, bfe)
		}
	}
	if len(exclusionEvents) != 2 {
		t.Fatalf("expected exactly 2 baseline_failure_excluded events, got %d", len(exclusionEvents))
	}
	checkNames := map[string]bool{}
	for _, ev := range exclusionEvents {
		checkNames[ev.CheckName] = true
	}
	if !checkNames["unit-tests"] {
		t.Error("expected baseline_failure_excluded event for unit-tests")
	}
	if !checkNames["lint"] {
		t.Error("expected baseline_failure_excluded event for lint")
	}
}

func TestValidate_ExcludesBaselineFailuresFromReplan(t *testing.T) {
	tmp := t.TempDir()
	testutil.WriteMinimalProjectFixtures(t, tmp)
	eventLog := runstore.NewEventLog(filepath.Join(tmp, "events.jsonl"))

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "baseline fail"},
					{Name: "integration-tests", Pass: false, Output: "new regression"},
				},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: tmp}, eventLog, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 1
	rs.BaselineFailures = map[string]string{"unit-tests": "baseline fail"}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) != 1 {
		t.Fatalf("expected 1 failure after excluding baseline, got %d: %v", len(action.Context.Failures), action.Context.Failures)
	}
	if strings.Contains(action.Context.Failures[0], "unit-tests") {
		t.Fatalf("baseline failure should be excluded, but found in context: %q", action.Context.Failures[0])
	}
	if !strings.Contains(action.Context.Failures[0], "integration-tests") {
		t.Fatalf("expected integration-tests failure to remain, got %q", action.Context.Failures[0])
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	found := false
	for _, ev := range events {
		if bfe, ok := ev.(*runstore.BaselineFailureExcludedEvent); ok {
			found = true
			if bfe.CheckName != "unit-tests" {
				t.Fatalf("baseline event check_name = %q, want unit-tests", bfe.CheckName)
			}
			if bfe.Output != "baseline fail" {
				t.Fatalf("baseline event output = %q, want baseline fail", bfe.Output)
			}
		}
	}
	if !found {
		t.Fatal("baseline_failure_excluded event not emitted")
	}
}

func TestValidateStage_UsesRunStateBaselineFailuresOnResume(t *testing.T) {
	tmp := t.TempDir()
	testutil.WriteMinimalProjectFixtures(t, tmp)
	eventLog := runstore.NewEventLog(filepath.Join(tmp, "events.jsonl"))

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "baseline fail"},
				},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: tmp}, eventLog, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 2
	rs.Resumed = true
	rs.BaselineFailures = map[string]string{"unit-tests": "baseline fail"}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true when all failures are baseline-excluded")
	}
	if got := rs.BaselineFailures["unit-tests"]; got != "baseline fail" {
		t.Fatalf("baseline failure output mutated: got %q, want baseline fail", got)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	found := false
	for _, ev := range events {
		if bfe, ok := ev.(*runstore.BaselineFailureExcludedEvent); ok {
			found = true
			if bfe.CheckName != "unit-tests" {
				t.Fatalf("baseline event check_name = %q, want unit-tests", bfe.CheckName)
			}
			if bfe.Output != "baseline fail" {
				t.Fatalf("baseline event output = %q, want baseline fail", bfe.Output)
			}
		}
	}
	if !found {
		t.Fatal("baseline_failure_excluded event not emitted")
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

	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     workDir,
		EvidenceDir: dir,
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
			{
				ScenarioName:  "subtract-works",
				AssertionType: "file_exists",
				Details:       `file "result.txt" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "result.txt",
				},
			},
		},
	}

	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     workDir,
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
			{
				ScenarioName:  "add-works",
				AssertionType: "file_exists",
				Details:       `file "out.txt" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "out.txt",
				},
			},
		},
	}

	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     workDir,
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

// TestDeferContractFailures_FileContainsDeferredWhenCovered verifies that
// file_contains failures are deferred when a pending task covers the file.
func TestDeferContractFailures_FileContainsDeferredWhenCovered(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "test-scenario",
			AssertionType: "file_contains",
			Details:       "pattern not found",
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "main.go",
					Pattern: "func main",
				},
			},
		},
	}
	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"main.go"},
		},
	}

	result := deferContractFailures(failures, tasks)
	remaining, deferred := result.remaining, result.deferred

	if len(deferred) != 1 {
		t.Fatalf("expected 1 deferred failure, got %d", len(deferred))
	}
	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining failures, got %d", len(remaining))
	}
	if deferred[0].ScenarioName != "test-scenario" {
		t.Fatalf("expected deferred scenario 'test-scenario', got %q", deferred[0].ScenarioName)
	}
}

// TestDeferContractFailures_FileExistsDeferredWhenCovered verifies that
// file_exists failures are deferred when a pending task covers the file.
func TestDeferContractFailures_FileExistsDeferredWhenCovered(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "another-scenario",
			AssertionType: "file_exists",
			Details:       "file does not exist",
			Assertion: contract.ContractAssertion{
				FileExists: "output.txt",
			},
		},
	}
	tasks := []runstore.Task{
		{
			TaskID:              "task-2",
			Status:              "pending",
			ExpectedTouchedArea: []string{"output.txt"},
		},
	}

	result := deferContractFailures(failures, tasks)
	remaining, deferred := result.remaining, result.deferred

	if len(deferred) != 1 {
		t.Fatalf("expected 1 deferred failure, got %d", len(deferred))
	}
	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining failures, got %d", len(remaining))
	}
	if deferred[0].Assertion.FileExists != "output.txt" {
		t.Fatalf("expected deferred file 'output.txt', got %q", deferred[0].Assertion.FileExists)
	}
}

// TestDeferContractFailures_NotDeferredWhenNoPendingTaskCovers verifies that
// failures are not deferred when no pending task covers the file.
func TestDeferContractFailures_NotDeferredWhenNoPendingTaskCovers(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "uncovered-scenario",
			AssertionType: "file_exists",
			Details:       "file does not exist",
			Assertion: contract.ContractAssertion{
				FileExists: "uncovered.txt",
			},
		},
	}
	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"other.txt"},
		},
	}

	result := deferContractFailures(failures, tasks)
	remaining, deferred := result.remaining, result.deferred

	if len(deferred) != 0 {
		t.Fatalf("expected 0 deferred failures, got %d", len(deferred))
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining failure, got %d", len(remaining))
	}
	if remaining[0].ScenarioName != "uncovered-scenario" {
		t.Fatalf("expected remaining scenario 'uncovered-scenario', got %q", remaining[0].ScenarioName)
	}
}

// TestDeferContractFailures_NotDeferredForDoneTask verifies that failures
// are not deferred by tasks with status="done".
func TestDeferContractFailures_NotDeferredForDoneTask(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "test-scenario",
			AssertionType: "file_contains",
			Details:       "pattern not found",
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "impl.go",
					Pattern: "func process",
				},
			},
		},
	}
	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "done", // Task is done, not pending
			ExpectedTouchedArea: []string{"impl.go"},
		},
	}

	result := deferContractFailures(failures, tasks)
	remaining, deferred := result.remaining, result.deferred

	if len(deferred) != 0 {
		t.Fatalf("expected 0 deferred failures (done task should not trigger deferral), got %d", len(deferred))
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining failure, got %d", len(remaining))
	}
}

// TestDeferContractFailures_NonDeferrableAssertionTypes verifies that assertion types
// other than file_contains and file_exists are never deferred.
func TestDeferContractFailures_NonDeferrableAssertionTypes(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "scenario-1",
			AssertionType: "file_not_contains",
			Details:       "pattern found when not expected",
			Assertion: contract.ContractAssertion{
				FileNotContains: &contract.FileContainsAssertion{
					Path:    "bad.go",
					Pattern: "deprecated",
				},
			},
		},
		{
			ScenarioName:  "scenario-2",
			AssertionType: "file_not_modified",
			Details:       "file was modified",
			Assertion: contract.ContractAssertion{
				FileNotModified: "config.yaml",
			},
		},
		{
			ScenarioName:  "scenario-3",
			AssertionType: "file_not_exists",
			Details:       "file should not exist",
			Assertion: contract.ContractAssertion{
				FileNotExists: "temp.txt",
			},
		},
	}
	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"bad.go", "config.yaml", "temp.txt"},
		},
	}

	result := deferContractFailures(failures, tasks)
	remaining, deferred := result.remaining, result.deferred

	// All non-deferrable types should remain
	if len(deferred) != 0 {
		t.Fatalf("expected 0 deferred failures (all are non-deferrable types), got %d", len(deferred))
	}
	if len(remaining) != 3 {
		t.Fatalf("expected 3 remaining failures, got %d", len(remaining))
	}
	// Verify the specific assertion types in remaining
	assertionTypes := make(map[string]bool)
	for _, f := range remaining {
		assertionTypes[f.AssertionType] = true
	}
	if !assertionTypes["file_not_contains"] || !assertionTypes["file_not_modified"] || !assertionTypes["file_not_exists"] {
		t.Fatalf("expected remaining failures to have file_not_contains, file_not_modified, and file_not_exists types")
	}
}

// TestDeferContractFailures_MultipleTasksFirstWins verifies that when multiple
// pending tasks cover the same file, the first one in slice order is recorded.
func TestDeferContractFailures_MultipleTasksFirstWins(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "multi-task-scenario",
			AssertionType: "file_exists",
			Details:       "file does not exist",
			Assertion: contract.ContractAssertion{
				FileExists: "shared.go",
			},
		},
	}
	tasks := []runstore.Task{
		{
			TaskID:              "task-1", // First task wins
			Status:              "pending",
			ExpectedTouchedArea: []string{"shared.go", "other1.go"},
		},
		{
			TaskID:              "task-2", // Also covers shared.go, but task-1 wins
			Status:              "pending",
			ExpectedTouchedArea: []string{"shared.go", "other2.go"},
		},
		{
			TaskID:              "task-3",
			Status:              "pending",
			ExpectedTouchedArea: []string{"other3.go"},
		},
	}

	result := deferContractFailures(failures, tasks)
	remaining, deferred := result.remaining, result.deferred

	if len(deferred) != 1 {
		t.Fatalf("expected 1 deferred failure, got %d", len(deferred))
	}
	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining failures, got %d", len(remaining))
	}
	// Verify that task-1 is recorded via the returned taskIDByFile map
	taskID := result.taskIDByFile["shared.go"]
	if taskID != "task-1" {
		t.Fatalf("expected task-1 to cover shared.go first, got %q", taskID)
	}
}

// TestDeferContractFailures_MixOfDeferrableAndNonDeferrable verifies that when
// given a mix of deferrable and non-deferrable failures, only the covered deferrable
// ones are removed and put in deferred, while non-deferrable and uncovered failures remain.
func TestDeferContractFailures_MixOfDeferrableAndNonDeferrable(t *testing.T) {
	failures := []contract.ContractFailure{
		// Deferrable and covered
		{
			ScenarioName:  "scenario-1",
			AssertionType: "file_contains",
			Details:       "pattern not found",
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "handler.go",
					Pattern: "Handle",
				},
			},
		},
		// Deferrable but not covered
		{
			ScenarioName:  "scenario-2",
			AssertionType: "file_exists",
			Details:       "file does not exist",
			Assertion: contract.ContractAssertion{
				FileExists: "uncovered_result.txt",
			},
		},
		// Non-deferrable (file_not_contains)
		{
			ScenarioName:  "scenario-3",
			AssertionType: "file_not_contains",
			Details:       "pattern found",
			Assertion: contract.ContractAssertion{
				FileNotContains: &contract.FileContainsAssertion{
					Path:    "handler.go",
					Pattern: "deprecated",
				},
			},
		},
		// Deferrable and covered
		{
			ScenarioName:  "scenario-4",
			AssertionType: "file_exists",
			Details:       "file does not exist",
			Assertion: contract.ContractAssertion{
				FileExists: "result.txt",
			},
		},
		// Non-deferrable (file_not_modified)
		{
			ScenarioName:  "scenario-5",
			AssertionType: "file_not_modified",
			Details:       "file was modified",
			Assertion: contract.ContractAssertion{
				FileNotModified: "config.yaml",
			},
		},
	}
	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"handler.go", "result.txt"},
		},
	}

	result := deferContractFailures(failures, tasks)
	remaining, deferred := result.remaining, result.deferred

	// Should have 2 deferred: handler.go (file_contains) and result.txt (file_exists)
	if len(deferred) != 2 {
		t.Fatalf("expected 2 deferred failures, got %d", len(deferred))
	}

	// Should have 3 remaining: uncovered_result.txt (deferrable but uncovered),
	// file_not_contains (non-deferrable), file_not_modified (non-deferrable)
	if len(remaining) != 3 {
		t.Fatalf("expected 3 remaining failures, got %d", len(remaining))
	}

	// Verify deferred contains the correct scenarios
	deferredScenarios := make(map[string]bool)
	for _, f := range deferred {
		deferredScenarios[f.ScenarioName] = true
	}
	if !deferredScenarios["scenario-1"] || !deferredScenarios["scenario-4"] {
		t.Fatalf("expected deferred to have scenario-1 and scenario-4")
	}

	// Verify remaining contains the correct scenarios
	remainingScenarios := make(map[string]bool)
	for _, f := range remaining {
		remainingScenarios[f.ScenarioName] = true
	}
	if !remainingScenarios["scenario-2"] || !remainingScenarios["scenario-3"] || !remainingScenarios["scenario-5"] {
		t.Fatalf("expected remaining to have scenario-2, scenario-3, and scenario-5")
	}
}

// TestValidateStage_AllContractsDeferredNoPriorFailures verifies that when all contract
// failures are deferred and no shell check failures exist, validation passes with Continue action.
func TestValidateStage_AllContractsDeferredNoPriorFailures(t *testing.T) {
	dir := t.TempDir()

	contractYAML := `scenarios:
  - name: scenario-deferred
    assertions:
      - file_exists: output.txt
`
	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{Results: []validator.CheckResult{{Name: "test", Pass: true}}},
			ProjectChecks: validator.CheckResults{Results: []validator.CheckResult{{Name: "lint", Pass: true}}},
		},
	}
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "scenario-deferred",
				AssertionType: "file_exists",
				Details:       `file "output.txt" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "output.txt",
				},
			},
		},
	}

	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     workDir,
		EvidenceDir: dir,
	}, nil, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"output.txt"},
		},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All contract failures deferred + no shell failures = Continue
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue when all contracts deferred and no shell failures, got %v", action.Kind)
	}

	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true when all failures deferred")
	}

	if len(rs.LastContractFailures) != 0 {
		t.Fatalf("expected LastContractFailures to be empty (all deferred), got %d", len(rs.LastContractFailures))
	}
}

// TestValidateStage_SomeContractsDeferredSomeNotTriggersReplan verifies that when
// some failures are deferred and others are not, only non-deferred failures trigger
// a ReplanFrom action.
func TestValidateStage_SomeContractsDeferredSomeNotTriggersReplan(t *testing.T) {
	dir := t.TempDir()

	contractYAML := `scenarios:
  - name: scenario-deferred
    assertions:
      - file_exists: output.txt
  - name: scenario-not-deferred
    assertions:
      - file_exists: uncovered.txt
`
	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{Results: []validator.CheckResult{{Name: "test", Pass: true}}},
			ProjectChecks: validator.CheckResults{Results: []validator.CheckResult{{Name: "lint", Pass: true}}},
		},
	}
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "scenario-deferred",
				AssertionType: "file_exists",
				Details:       `file "output.txt" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "output.txt",
				},
			},
			{
				ScenarioName:  "scenario-not-deferred",
				AssertionType: "file_exists",
				Details:       `file "uncovered.txt" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "uncovered.txt",
				},
			},
		},
	}

	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     workDir,
		EvidenceDir: dir,
	}, nil, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 1
	rs.Tasks = []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"output.txt"}, // Only covers output.txt, not uncovered.txt
		},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// One deferred + one not = ReplanFrom with only non-deferred in context
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom when some failures not deferred, got %v", action.Kind)
	}

	if action.Context == nil || len(action.Context.Failures) == 0 {
		t.Fatal("expected FailureContext with failures")
	}

	// Should only have the non-deferred failure
	if len(action.Context.Failures) != 1 {
		t.Fatalf("expected 1 failure (only non-deferred), got %d: %v", len(action.Context.Failures), action.Context.Failures)
	}

	if !strings.Contains(action.Context.Failures[0], "scenario-not-deferred") {
		t.Fatalf("expected failure to reference scenario-not-deferred, got %q", action.Context.Failures[0])
	}

	if !strings.Contains(action.Context.Failures[0], "uncovered.txt") {
		t.Fatalf("expected failure to reference uncovered.txt, got %q", action.Context.Failures[0])
	}
}

// TestValidateStage_DeferredFailuresExcludedFromLastContractFailures verifies that
// deferred failures do not appear in rs.LastContractFailures.
func TestValidateStage_DeferredFailuresExcludedFromLastContractFailures(t *testing.T) {
	dir := t.TempDir()

	contractYAML := `scenarios:
  - name: deferred-scenario
    assertions:
      - file_exists: deferred.txt
  - name: not-deferred-scenario
    assertions:
      - file_exists: not-deferred.txt
`
	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{Results: []validator.CheckResult{{Name: "test", Pass: true}}},
			ProjectChecks: validator.CheckResults{Results: []validator.CheckResult{{Name: "lint", Pass: true}}},
		},
	}
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "deferred-scenario",
				AssertionType: "file_exists",
				Details:       `file "deferred.txt" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "deferred.txt",
				},
			},
			{
				ScenarioName:  "not-deferred-scenario",
				AssertionType: "file_exists",
				Details:       `file "not-deferred.txt" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "not-deferred.txt",
				},
			},
		},
	}

	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     workDir,
		EvidenceDir: dir,
	}, nil, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"deferred.txt"}, // Only covers deferred.txt
		},
	}

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// LastContractFailures should only contain the not-deferred failure
	if len(rs.LastContractFailures) != 1 {
		t.Fatalf("expected 1 failure in LastContractFailures, got %d: %v", len(rs.LastContractFailures), rs.LastContractFailures)
	}

	if !strings.Contains(rs.LastContractFailures[0], "not-deferred-scenario") {
		t.Fatalf("expected LastContractFailures to reference not-deferred-scenario, got %q", rs.LastContractFailures[0])
	}

	if strings.Contains(rs.LastContractFailures[0], "contract:deferred-scenario") {
		t.Fatalf("expected LastContractFailures to NOT contain contract:deferred-scenario, got %q", rs.LastContractFailures[0])
	}
}

// TestValidateStage_ContractDeferredEventEmitted verifies that contract_deferred events
// are emitted with correct fields: scenario_name, file_path, pattern, task_id.
func TestValidateStage_ContractDeferredEventEmitted(t *testing.T) {
	dir := t.TempDir()
	eventLogPath := filepath.Join(dir, "events.jsonl")

	contractYAML := `scenarios:
  - name: test-file-exists
    assertions:
      - file_exists: result.txt
  - name: test-file-contains
    assertions:
      - file_contains:
          path: handler.go
          pattern: "func Handle"
`
	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{Results: []validator.CheckResult{{Name: "test", Pass: true}}},
			ProjectChecks: validator.CheckResults{Results: []validator.CheckResult{{Name: "lint", Pass: true}}},
		},
	}
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "test-file-exists",
				AssertionType: "file_exists",
				Details:       "file does not exist",
				Assertion: contract.ContractAssertion{
					FileExists: "result.txt",
				},
			},
			{
				ScenarioName:  "test-file-contains",
				AssertionType: "file_contains",
				Details:       "pattern not found",
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "handler.go",
						Pattern: "func Handle",
					},
				},
			},
		},
	}

	eventLog := runstore.NewEventLog(eventLogPath)
	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     workDir,
		EvidenceDir: dir,
	}, eventLog, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{
			TaskID:              "task-101",
			Status:              "pending",
			ExpectedTouchedArea: []string{"result.txt", "handler.go"},
		},
	}

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read events from log
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("failed to read event log: %v", err)
	}

	// Find contract_deferred events
	var deferredEvents []*runstore.ContractDeferredEvent
	for _, ev := range events {
		if ce, ok := ev.(*runstore.ContractDeferredEvent); ok {
			deferredEvents = append(deferredEvents, ce)
		}
	}

	if len(deferredEvents) != 2 {
		t.Fatalf("expected 2 contract_deferred events, got %d", len(deferredEvents))
	}

	// Verify events have correct fields
	eventsByScenario := make(map[string]*runstore.ContractDeferredEvent)
	for _, ev := range deferredEvents {
		eventsByScenario[ev.ScenarioName] = ev
	}

	// Check file_exists event
	if ev, ok := eventsByScenario["test-file-exists"]; !ok {
		t.Fatal("expected contract_deferred event for test-file-exists")
	} else {
		if ev.ScenarioName != "test-file-exists" {
			t.Fatalf("expected ScenarioName 'test-file-exists', got %q", ev.ScenarioName)
		}
		if ev.FilePath != "result.txt" {
			t.Fatalf("expected FilePath 'result.txt', got %q", ev.FilePath)
		}
		if ev.Pattern != "" {
			t.Fatalf("expected Pattern '' (empty for file_exists), got %q", ev.Pattern)
		}
		if ev.TaskID != "task-101" {
			t.Fatalf("expected TaskID 'task-101', got %q", ev.TaskID)
		}
	}

	// Check file_contains event
	if ev, ok := eventsByScenario["test-file-contains"]; !ok {
		t.Fatal("expected contract_deferred event for test-file-contains")
	} else {
		if ev.ScenarioName != "test-file-contains" {
			t.Fatalf("expected ScenarioName 'test-file-contains', got %q", ev.ScenarioName)
		}
		if ev.FilePath != "handler.go" {
			t.Fatalf("expected FilePath 'handler.go', got %q", ev.FilePath)
		}
		if ev.Pattern != "func Handle" {
			t.Fatalf("expected Pattern 'func Handle', got %q", ev.Pattern)
		}
		if ev.TaskID != "task-101" {
			t.Fatalf("expected TaskID 'task-101', got %q", ev.TaskID)
		}
	}
}

// TestValidateStage_ContractDeferralChainOrder verifies that the processing chain order
// is: defer → self-correct → re-evaluate → re-defer → format-to-failures.
// This is an integration test that verifies the full flow works correctly together.
func TestValidateStage_ContractDeferralChainOrder(t *testing.T) {
	dir := t.TempDir()

	// Write a contract with a file_contains failure that will be deferred
	contractYAML := `scenarios:
  - name: scenario-with-deferred-and-uncorrectable
    assertions:
      - file_contains:
          path: original.go
          pattern: "func MainLogic"
`
	contractPath := filepath.Join(dir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{Results: []validator.CheckResult{{Name: "test", Pass: true}}},
			ProjectChecks: validator.CheckResults{Results: []validator.CheckResult{{Name: "lint", Pass: true}}},
		},
	}
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "scenario-with-deferred-and-uncorrectable",
				AssertionType: "file_contains",
				Details:       `pattern "func MainLogic" not found in "original.go"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "original.go",
						Pattern: "func MainLogic",
					},
				},
			},
		},
	}

	workDir := newValidateWorkDir(t)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:          workDir,
		EvidenceDir:      dir,
		SearchExtensions: []string{".go"},
	}, nil, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{
			TaskID:              "task-pending",
			Status:              "pending",
			ExpectedTouchedArea: []string{"original.go"}, // Covers the file, should defer
		},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All contract failures should be deferred by the deferral logic
	// Since there are no shell failures, this should Continue
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue when all contract failures deferred and no shell failures, got %v", action.Kind)
	}

	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}

	// LastContractFailures should be empty (all deferred)
	if len(rs.LastContractFailures) != 0 {
		t.Fatalf("expected LastContractFailures to be empty after deferral, got %d: %v", len(rs.LastContractFailures), rs.LastContractFailures)
	}
}

func TestValidateStage_AutoFixRunsBeforeValidation(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteMinimalProjectFixtures(t, dir)
	// Create a file that auto-fix will modify
	targetFile := filepath.Join(dir, "fixme.txt")
	if err := os.WriteFile(targetFile, []byte("unfixed"), 0o644); err != nil {
		t.Fatal(err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{Results: []validator.CheckResult{{Name: "test", Pass: true}}},
			ProjectChecks: validator.CheckResults{},
		},
	}

	// Auto-fix command rewrites the file content
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: dir,
		AutoFix: []validator.Check{
			{Name: "fix-it", Command: "echo fixed > fixme.txt", Type: "format"},
		},
	}, nil, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Verify auto-fix ran: file should be modified
	got, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "fixed" {
		t.Fatalf("auto-fix did not run: file contains %q, want \"fixed\"", string(got))
	}
}

func TestValidateStage_AutoFixErrorDoesNotBlockValidation(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{Results: []validator.CheckResult{{Name: "test", Pass: true}}},
			ProjectChecks: validator.CheckResults{},
		},
	}

	// Auto-fix command that fails (nonexistent command)
	workDir := t.TempDir()
	testutil.WriteMinimalProjectFixtures(t, workDir)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: workDir,
		AutoFix: []validator.Check{
			{Name: "broken", Command: "false", Type: "format"},
		},
	}, nil, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("auto-fix failure should not propagate: %v", err)
	}
	// Validation should still continue despite auto-fix failure
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue despite auto-fix failure, got %v", action.Kind)
	}
}

func TestValidateStage_EmptyAutoFixSkipped(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{Results: []validator.CheckResult{{Name: "test", Pass: true}}},
			ProjectChecks: validator.CheckResults{},
		},
	}

	workDir := t.TempDir()
	testutil.WriteMinimalProjectFixtures(t, workDir)
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: workDir,
		AutoFix: []validator.Check{}, // empty — nothing to run
	}, nil, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
}

// TestValidateStage_PipelineOrdering verifies the complete processing pipeline:
// raw failures → deferral (no-op when no tasks cover) → self-correction → formatted strings.
// This test confirms that:
// 1. Raw failures flow through deferral unchanged when no pending tasks cover the files
// 2. Failures are processed through self-correction
// 3. Final formatted output strings match the expected format: "contract:<scenario> — <type> failed: <details>"
func TestValidateStage_PipelineOrdering(t *testing.T) {
	dir := t.TempDir()

	// Create a contract with a file_contains failure that won't be deferred
	// (no pending tasks cover the file)
	contractYAML := `scenarios:
  - name: scenario-pipeline-test
    assertions:
      - file_contains:
          path: "handler.go"
          pattern: "func ProcessRequest"
  - name: scenario-undeferred-exists
    assertions:
      - file_exists: "config.yaml"
`
	contractPath := filepath.Join(dir, "scenario-contracts.yaml")
	if err := os.WriteFile(contractPath, []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass:          true,
			AlwaysRun:     validator.CheckResults{Results: []validator.CheckResult{{Name: "test", Pass: true}}},
			ProjectChecks: validator.CheckResults{Results: []validator.CheckResult{{Name: "lint", Pass: true}}},
		},
	}

	// Create raw failures that won't be deferred (no pending tasks cover these files)
	evaluator := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "scenario-pipeline-test",
				AssertionType: "file_contains",
				Details:       `pattern "func ProcessRequest" not found in "handler.go"`,
				Assertion: contract.ContractAssertion{
					FileContains: &contract.FileContainsAssertion{
						Path:    "handler.go",
						Pattern: "func ProcessRequest",
					},
				},
			},
			{
				ScenarioName:  "scenario-undeferred-exists",
				AssertionType: "file_exists",
				Details:       `file "config.yaml" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "config.yaml",
				},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:          newValidateWorkDir(t),
		EvidenceDir:      dir,
		SearchExtensions: []string{".go"},
	}, nil, evaluator, nil)

	// Create RunState with NO pending tasks that cover the failing files
	// This ensures deferral is a no-op (failures pass through unchanged)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Tasks = []runstore.Task{
		{
			TaskID:              "task-other",
			Status:              "pending",
			ExpectedTouchedArea: []string{"other_file.go"}, // Doesn't cover handler.go or config.yaml
		},
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify: Should replan due to non-deferred failures
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}

	// Verify: FailureContext contains the failures
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}

	failures := action.Context.Failures
	if len(failures) != 2 {
		t.Fatalf("expected 2 failures (both non-deferred), got %d: %v", len(failures), failures)
	}

	// Verify: LastContractFailures contains formatted strings with correct format
	// Format should be: "contract:<scenario> — <type> failed: <details>"
	if len(rs.LastContractFailures) != 2 {
		t.Fatalf("expected 2 formatted failures in LastContractFailures, got %d: %v", len(rs.LastContractFailures), rs.LastContractFailures)
	}

	// Expected formatted strings following the pipeline:
	// 1. Raw failures (from evaluator)
	// 2. Through deferral (no-op since no tasks cover these files)
	// 3. Through self-correction (no corrections possible)
	// 4. Formatted to strings with "contract:<scenario> — <type> failed: <details>"
	expectedFormats := map[string]bool{
		`contract:scenario-pipeline-test — file_contains failed: pattern "func ProcessRequest" not found in "handler.go"`: false,
		`contract:scenario-undeferred-exists — file_exists failed: file "config.yaml" does not exist`:                     false,
	}

	for _, failure := range rs.LastContractFailures {
		found := false
		for expected := range expectedFormats {
			if failure == expected {
				expectedFormats[expected] = true
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("formatted failure does not match expected format: %q\nExpected one of:\n%v", failure, expectedFormats)
		}
	}

	// Verify all expected formats were present
	for expected, found := range expectedFormats {
		if !found {
			t.Fatalf("expected failure not found in output: %q", expected)
		}
	}

	// Additional verification: Failures in action context should match LastContractFailures
	// (they come from the same source after processing through the pipeline)
	for i, failure := range failures {
		if failure != rs.LastContractFailures[i] {
			t.Fatalf("failure at index %d in action context does not match LastContractFailures: %q vs %q", i, failure, rs.LastContractFailures[i])
		}
	}

	// Verify format compliance: all failures start with "contract:" prefix
	for _, failure := range rs.LastContractFailures {
		if !strings.HasPrefix(failure, "contract:") {
			t.Fatalf("expected failure to have 'contract:' prefix, got: %q", failure)
		}
		if !strings.Contains(failure, " — ") {
			t.Fatalf("expected failure to contain ' — ' separator, got: %q", failure)
		}
		if !strings.Contains(failure, " failed: ") {
			t.Fatalf("expected failure to contain ' failed: ' text, got: %q", failure)
		}
	}
}
