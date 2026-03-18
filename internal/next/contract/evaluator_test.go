package contract

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo creates a git repo in dir, commits a file, and returns the dir.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	// Create an initial commit so HEAD exists.
	run("git", "commit", "--allow-empty", "-m", "init")
	return dir
}

func TestEvaluator_NilContract(t *testing.T) {
	ev := &DefaultContractEvaluator{}
	failures, err := ev.Evaluate(context.Background(), nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures for nil contract, got %v", failures)
	}
}

func TestEvaluator_EmptyAssertions(t *testing.T) {
	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{Name: "empty", Assertions: []ContractAssertion{}},
		},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
}

func TestEvaluator_FileExists_Pass(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "foo.txt")
	os.WriteFile(f, []byte("hello"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileExists: "foo.txt"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
}

func TestEvaluator_FileExists_Fail(t *testing.T) {
	dir := t.TempDir()
	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileExists: "missing.txt"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %v", failures)
	}
	if failures[0].AssertionType != "file_exists" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
	if failures[0].ScenarioName != "s" {
		t.Errorf("wrong scenario name: %s", failures[0].ScenarioName)
	}
}

func TestEvaluator_FileContains_Pass(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "a.txt", Pattern: "hello"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
}

func TestEvaluator_FileContains_PatternMissing(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "a.txt", Pattern: "goodbye"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %v", failures)
	}
	if failures[0].AssertionType != "file_contains" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

func TestEvaluator_FileContains_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "no.txt", Pattern: "x"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure for nonexistent file, got %v", failures)
	}
	if failures[0].AssertionType != "file_contains" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

func TestEvaluator_FileNotExists_Pass(t *testing.T) {
	dir := t.TempDir()
	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotExists: "absent.txt"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
}

func TestEvaluator_FileNotExists_Fail(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "present.txt"), []byte("x"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotExists: "present.txt"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %v", failures)
	}
	if failures[0].AssertionType != "file_not_exists" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

func TestEvaluator_FileNotExists_PermissionDenied(t *testing.T) {
	dir := t.TempDir()
	// Create a subdirectory and revoke read permissions
	subdir := filepath.Join(dir, "restricted")
	os.Mkdir(subdir, 0755)
	os.Chmod(subdir, 0000)
	defer os.Chmod(subdir, 0755) // Restore for cleanup

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotExists: "restricted/file.txt"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure for permission denied, got %v", failures)
	}
	if failures[0].AssertionType != "file_not_exists" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
	// Check that the error message indicates "cannot stat" rather than "exists"
	if !strings.Contains(failures[0].Details, "cannot stat") {
		t.Errorf("expected 'cannot stat' in details, got: %s", failures[0].Details)
	}
}

func TestEvaluator_FileNotContains_Pass(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotContains: &FileContainsAssertion{Path: "b.txt", Pattern: "goodbye"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
}

func TestEvaluator_FileNotContains_Fail(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello world"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotContains: &FileContainsAssertion{Path: "b.txt", Pattern: "hello"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %v", failures)
	}
	if failures[0].AssertionType != "file_not_contains" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

func TestEvaluator_FileNotContains_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotContains: &FileContainsAssertion{Path: "no.txt", Pattern: "x"}},
			},
		}},
	}
	// A nonexistent file trivially does not contain the pattern — pass.
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures for nonexistent file in file_not_contains, got %v", failures)
	}
}

func TestEvaluator_FileNotModified_Pass(t *testing.T) {
	dir := initGitRepo(t)
	// Write a file and commit it — it is clean, not modified.
	p := filepath.Join(dir, "clean.go")
	os.WriteFile(p, []byte("package main"), 0644)
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Run()
	}
	run("git", "add", "clean.go")
	run("git", "commit", "-m", "add clean.go")

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotModified: "clean.go"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures for unmodified file, got %v", failures)
	}
}

func TestEvaluator_FileNotModified_Fail(t *testing.T) {
	dir := initGitRepo(t)
	p := filepath.Join(dir, "dirty.go")
	os.WriteFile(p, []byte("package main"), 0644)
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Run()
	}
	run("git", "add", "dirty.go")
	run("git", "commit", "-m", "add dirty.go")
	// Modify the file after commit.
	os.WriteFile(p, []byte("package main\n// changed"), 0644)
	run("git", "add", "dirty.go")

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotModified: "dirty.go"},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure for modified file, got %v", failures)
	}
	if failures[0].AssertionType != "file_not_modified" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

func TestEvaluator_MultipleAssertions_PartialFailure_NoShortCircuit(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("content"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileExists: "exists.txt"},   // pass
				{FileExists: "missing1.txt"}, // fail
				{FileExists: "missing2.txt"}, // fail — must still be checked
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 2 {
		t.Errorf("expected 2 failures (no short-circuit), got %d: %v", len(failures), failures)
	}
}

func TestEvaluator_MultipleScenarios(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{Name: "pass-scenario", Assertions: []ContractAssertion{
				{FileExists: "a.txt"},
			}},
			{Name: "fail-scenario", Assertions: []ContractAssertion{
				{FileExists: "b.txt"},
			}},
		},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %v", failures)
	}
	if failures[0].ScenarioName != "fail-scenario" {
		t.Errorf("wrong scenario name: %s", failures[0].ScenarioName)
	}
}

func TestEvaluator_FileContains_WhitespaceNormalized_Pass(t *testing.T) {
	dir := t.TempDir()
	// File has tab-separated fields (like gofmt output)
	os.WriteFile(filepath.Join(dir, "types.go"), []byte("type State struct {\n\tScenarioTestsWritten\tbool\n}"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "types.go", Pattern: "ScenarioTestsWritten bool"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures (whitespace-normalized match), got %v", failures)
	}
}

func TestEvaluator_FileContains_WhitespaceNormalized_TabsAndSpaces(t *testing.T) {
	dir := t.TempDir()
	// File has mixed tabs and spaces
	os.WriteFile(filepath.Join(dir, "mixed.go"), []byte("func  \t doThing(a \t\t int,  b   string)"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "mixed.go", Pattern: "doThing(a int, b string)"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures (whitespace-normalized match with mixed tabs/spaces), got %v", failures)
	}
}

func TestEvaluator_FileNotContains_WhitespaceNormalized_Fail(t *testing.T) {
	dir := t.TempDir()
	// File has tab-separated fields (like gofmt output)
	os.WriteFile(filepath.Join(dir, "types.go"), []byte("type State struct {\n\tScenarioTestsWritten\tbool\n}"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotContains: &FileContainsAssertion{Path: "types.go", Pattern: "ScenarioTestsWritten bool"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure (whitespace-normalized match should detect pattern), got %d: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "file_not_contains" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

func TestEvaluator_FileContains_WhitespaceNormalized_StillFailsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "types.go"), []byte("type State struct {\n\tFooBar\tint\n}"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "types.go", Pattern: "BazQux string"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure (pattern truly absent), got %d: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "file_contains" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

// --- Regex fallback tests (RED phase) ---

func TestEvaluator_FileContains_RegexPattern_ChainIDs(t *testing.T) {
	dir := t.TempDir()
	// Simulate gofmt-style field declaration with extra whitespace
	content := "type Config struct {\n\tChainIDs        []string `json:\"chain_ids\"`\n}"
	os.WriteFile(filepath.Join(dir, "config.go"), []byte(content), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "config.go", Pattern: `ChainIDs.*\[\]string`}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected regex pattern to match, got failures: %v", failures)
	}
}

func TestEvaluator_FileContains_RegexPattern_FuncSignature(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "foo.go"), []byte("package foo\n\nfunc NewFoo(bar string) *Foo {\n\treturn nil\n}\n"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "foo.go", Pattern: `func \w+\(`}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected regex pattern to match func signature, got failures: %v", failures)
	}
}

func TestEvaluator_FileNotContains_RegexPattern_Fail(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("TODO: urgent fix needed here"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotContains: &FileContainsAssertion{Path: "notes.txt", Pattern: `TODO.*urgent`}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure (regex matched TODO.*urgent), got %d: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "file_not_contains" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

func TestEvaluator_FileContains_InvalidRegex_FallsBackToLiteral(t *testing.T) {
	dir := t.TempDir()
	// The literal pattern "[unclosed" appears verbatim in the file
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("some [unclosed bracket"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "a.txt", Pattern: "[unclosed"}},
			},
		}},
	}
	// Must not panic; invalid regex falls back to literal match which succeeds
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected literal fallback to match, got failures: %v", failures)
	}
}

func TestEvaluator_FileContains_LiteralPatternStillWorks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "a.txt", Pattern: "hello"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected literal pattern to still work, got failures: %v", failures)
	}
}

// --- End regex fallback tests ---

func TestEvaluator_ZeroValueAssertion(t *testing.T) {
	dir := t.TempDir()
	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{}, // zero-value assertion with no fields set
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure for zero-value assertion, got %d failures: %v", len(failures), failures)
	}
	if failures[0].AssertionType != "invalid_assertion" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
	if !strings.Contains(failures[0].Details, "no fields set") {
		t.Errorf("expected 'no fields set' in details, got: %s", failures[0].Details)
	}
}

func TestEvaluator_FileContains_RegexPattern_Pass(t *testing.T) {
	dir := t.TempDir()
	// File with struct field definitions similar to those in the codebase
	os.WriteFile(filepath.Join(dir, "types.go"), []byte("type State struct {\n\tChainIDs []string\n}"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "types.go", Pattern: "ChainIDs.*\\[\\]string"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures for regex pattern match, got %v", failures)
	}
}

func TestEvaluator_FileContains_RegexWithEscapedChars_Pass(t *testing.T) {
	dir := t.TempDir()
	// File with function call containing len()
	os.WriteFile(filepath.Join(dir, "logic.go"), []byte("if len(e.LastError) > 2000 { e.LastError = e.LastError[:2000] }"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "logic.go", Pattern: "len\\(.*LastError.*\\) > 2000"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures for regex with escaped chars, got %v", failures)
	}
}

func TestEvaluator_FileContains_RegexPattern_Fail(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "types.go"), []byte("type State struct {\n\tOtherField bool\n}"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "types.go", Pattern: "ChainIDs.*\\[\\]string"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure for missing regex pattern, got %v", failures)
	}
	if failures[0].AssertionType != "file_contains" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

func TestEvaluator_FileNotContains_RegexPattern_Pass(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "types.go"), []byte("type State struct {\n\tOtherField bool\n}"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotContains: &FileContainsAssertion{Path: "types.go", Pattern: "ChainIDs.*\\[\\]string"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures (pattern should not be found), got %v", failures)
	}
}

func TestEvaluator_FileNotContains_RegexPattern_Fail_ChainIDs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "types.go"), []byte("type State struct {\n\tChainIDs []string\n}"), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "s",
			Assertions: []ContractAssertion{
				{FileNotContains: &FileContainsAssertion{Path: "types.go", Pattern: "ChainIDs.*\\[\\]string"}},
			},
		}},
	}
	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure (regex pattern found but should not be), got %v", failures)
	}
	if failures[0].AssertionType != "file_not_contains" {
		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
	}
}

func TestEvaluator_FileContains_ActualPatternsFromSpec(t *testing.T) {
	dir := t.TempDir()

	// Create files with actual struct definitions matching the spec patterns
	os.WriteFile(filepath.Join(dir, "runstore_types.go"), []byte(`
type State struct {
	ChainIDs []string
	ConsecutiveFails int
	LastError string
}
`), 0644)

	os.WriteFile(filepath.Join(dir, "planner_types.go"), []byte(`
type Task struct {
	Fixes []string
}
`), 0644)

	os.WriteFile(filepath.Join(dir, "execpolicy_policy.go"), []byte(`
type Policy struct {
	ErrorContextThreshold int
	ModelEscalationThreshold int
	Escalation EscalationConfig
}
`), 0644)

	os.WriteFile(filepath.Join(dir, "lineage.go"), []byte(`
if len(entry.LastError) > 2000 {
	entry.LastError = entry.LastError[:2000]
}
`), 0644)

	ev := &DefaultContractEvaluator{}
	contract := ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "spec-patterns",
			Assertions: []ContractAssertion{
				{FileContains: &FileContainsAssertion{Path: "runstore_types.go", Pattern: "ChainIDs.*\\[\\]string"}},
				{FileContains: &FileContainsAssertion{Path: "runstore_types.go", Pattern: "ConsecutiveFails.*int"}},
				{FileContains: &FileContainsAssertion{Path: "runstore_types.go", Pattern: "LastError.*string"}},
				{FileContains: &FileContainsAssertion{Path: "planner_types.go", Pattern: "Fixes.*\\[\\]string"}},
				{FileContains: &FileContainsAssertion{Path: "execpolicy_policy.go", Pattern: "ErrorContextThreshold.*int"}},
				{FileContains: &FileContainsAssertion{Path: "execpolicy_policy.go", Pattern: "ModelEscalationThreshold.*int"}},
				{FileContains: &FileContainsAssertion{Path: "execpolicy_policy.go", Pattern: "Escalation.*EscalationConfig"}},
				{FileContains: &FileContainsAssertion{Path: "lineage.go", Pattern: "len\\(.*LastError.*\\) > 2000"}},
				{FileContains: &FileContainsAssertion{Path: "lineage.go", Pattern: "LastError.*\\[:\\d+\\]"}},
			},
		}},
	}

	failures, err := ev.Evaluate(context.Background(), &contract, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures for spec patterns, got %d failures: %v", len(failures), failures)
	}
}
