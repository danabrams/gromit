package contract

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- file_exists ---

func TestScenario_FileExists_Pass(t *testing.T) {
	// Seed: temp dir with an existing file.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "calc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calc", "calc.go"), []byte("package calc"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Invoke: evaluate file_exists assertion.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "file-exists-pass",
			Assertions: []ContractAssertion{
				{FileExists: "calc/calc.go"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: passes.
	if len(failures) != 0 {
		t.Errorf("expected 0 failures, got %d: %v", len(failures), failures)
	}
}

func TestScenario_FileExists_Fail(t *testing.T) {
	// Seed: empty temp dir (file missing).
	dir := t.TempDir()

	// Invoke: evaluate file_exists assertion for missing file.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "file-exists-fail",
			Assertions: []ContractAssertion{
				{FileExists: "calc/calc.go"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: fails with correct message.
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "file_exists" {
		t.Errorf("expected assertion type file_exists, got %q", failures[0].AssertionType)
	}
	if failures[0].ScenarioName != "file-exists-fail" {
		t.Errorf("expected scenario name file-exists-fail, got %q", failures[0].ScenarioName)
	}
	if !strings.Contains(failures[0].Details, "does not exist") {
		t.Errorf("expected 'does not exist' in details, got %q", failures[0].Details)
	}
}

// --- file_not_exists ---

func TestScenario_FileNotExists_ExistingFile_Fails(t *testing.T) {
	// Seed: temp dir with an existing file.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "legacy.go"), []byte("package legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Invoke: file_not_exists on existing file.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "not-exists-on-existing",
			Assertions: []ContractAssertion{
				{FileNotExists: "legacy.go"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: fails.
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "file_not_exists" {
		t.Errorf("expected assertion type file_not_exists, got %q", failures[0].AssertionType)
	}
	if !strings.Contains(failures[0].Details, "exists but should not") {
		t.Errorf("expected 'exists but should not' in details, got %q", failures[0].Details)
	}
}

func TestScenario_FileNotExists_MissingFile_Passes(t *testing.T) {
	// Seed: empty temp dir.
	dir := t.TempDir()

	// Invoke: file_not_exists on missing file.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "not-exists-on-missing",
			Assertions: []ContractAssertion{
				{FileNotExists: "legacy.go"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: passes.
	if len(failures) != 0 {
		t.Errorf("expected 0 failures, got %d: %v", len(failures), failures)
	}
}

// --- file_contains / file_not_contains ---

func TestScenario_FileContains_SubstringPresent_Passes(t *testing.T) {
	// Seed: file containing "func Add".
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Invoke: file_contains with substring present.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "contains-present",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "calc.go", Pattern: "func Add"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: passes.
	if len(failures) != 0 {
		t.Errorf("expected 0 failures, got %d: %v", len(failures), failures)
	}
}

func TestScenario_FileContains_SubstringMissing_Fails(t *testing.T) {
	// Seed: file containing "func Add" but not "func Subtract".
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Invoke: file_contains with missing substring.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "contains-missing",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "calc.go", Pattern: "func Subtract"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: fails with "pattern X not found in Y" format.
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "file_contains" {
		t.Errorf("expected assertion type file_contains, got %q", failures[0].AssertionType)
	}
	if !strings.Contains(failures[0].Details, "pattern") || !strings.Contains(failures[0].Details, "not found in") {
		t.Errorf("expected 'pattern X not found in Y' format, got %q", failures[0].Details)
	}
}

func TestScenario_FileNotContains_SubstringPresent_Fails(t *testing.T) {
	// Seed: file containing "go:deprecated".
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package calc\n\n//go:deprecated use NewCalc instead"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Invoke: file_not_contains with substring present.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "not-contains-present",
			Assertions: []ContractAssertion{
				{FileNotContains: &FileContainsAssertion{Path: "calc.go", Pattern: "go:deprecated"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: fails.
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "file_not_contains" {
		t.Errorf("expected assertion type file_not_contains, got %q", failures[0].AssertionType)
	}
}

func TestScenario_FileNotContains_SubstringAbsent_Passes(t *testing.T) {
	// Seed: file without "go:deprecated".
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Invoke: file_not_contains with absent substring.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "not-contains-absent",
			Assertions: []ContractAssertion{
				{FileNotContains: &FileContainsAssertion{Path: "calc.go", Pattern: "go:deprecated"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: passes.
	if len(failures) != 0 {
		t.Errorf("expected 0 failures, got %d: %v", len(failures), failures)
	}
}

// --- file_not_modified ---

// initScenarioGitRepo creates a git repo in a temp dir and returns the path.
func initScenarioGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	runGit("git", "init")
	runGit("git", "config", "user.email", "test@test.com")
	runGit("git", "config", "user.name", "Test")
	runGit("git", "commit", "--allow-empty", "-m", "init")
	return dir
}

func TestScenario_FileNotModified_Unchanged_Passes(t *testing.T) {
	// Seed: committed file, unchanged from HEAD.
	dir := initScenarioGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte("package config"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Run()
	}
	runGit("git", "add", "config.go")
	runGit("git", "commit", "-m", "add config.go")

	// Invoke: file_not_modified on unchanged file.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "not-modified-pass",
			Assertions: []ContractAssertion{
				{FileNotModified: "config.go"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: passes.
	if len(failures) != 0 {
		t.Errorf("expected 0 failures for unmodified file, got %d: %v", len(failures), failures)
	}
}

func TestScenario_FileNotModified_Modified_Fails(t *testing.T) {
	// Seed: committed file, then modified after commit.
	dir := initScenarioGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte("package config"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Run()
	}
	runGit("git", "add", "config.go")
	runGit("git", "commit", "-m", "add config.go")

	// Modify after commit.
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte("package config\n// changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("git", "add", "config.go")

	// Invoke: file_not_modified on modified file.
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "not-modified-fail",
			Assertions: []ContractAssertion{
				{FileNotModified: "config.go"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: fails.
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure for modified file, got %d: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "file_not_modified" {
		t.Errorf("expected assertion type file_not_modified, got %q", failures[0].AssertionType)
	}
	if !strings.Contains(failures[0].Details, "has been modified") {
		t.Errorf("expected 'has been modified' in details, got %q", failures[0].Details)
	}
}

// --- Multiple assertions, partial failure (no short-circuit) ---

func TestScenario_MultipleAssertions_PartialFailure_NoShortCircuit(t *testing.T) {
	// Seed: calc.go exists with func Add but not func Multiply.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "calc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calc", "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Invoke: evaluate 2 assertions -- file_exists (pass) and file_contains (fail).
	ev := &DefaultContractEvaluator{}
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "calculator-module-exists",
			Assertions: []ContractAssertion{
				{FileExists: "calc/calc.go"},
				{FileContains: &FileContainsAssertion{Path: "calc/calc.go", Pattern: "func Multiply"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), sc, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: both checked (no short-circuit), only the failure returned.
	if len(failures) != 1 {
		t.Fatalf("expected exactly 1 failure, got %d: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "file_contains" {
		t.Errorf("expected file_contains failure, got %q", failures[0].AssertionType)
	}
	if failures[0].ScenarioName != "calculator-module-exists" {
		t.Errorf("expected scenario name calculator-module-exists, got %q", failures[0].ScenarioName)
	}
}
