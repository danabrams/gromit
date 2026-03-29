package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAugmentWithTestAssertions_NoOp_WhenScenarioContractNil(t *testing.T) {
	err := AugmentWithTestAssertions(nil, ".")
	if err != nil {
		t.Fatalf("expected nil error for nil contract, got %v", err)
	}
}

func TestAugmentWithTestAssertions_NoOp_WhenScenarioContractEmpty(t *testing.T) {
	err := AugmentWithTestAssertions(&ScenarioContract{}, ".")
	if err != nil {
		t.Fatalf("expected nil error for empty contract, got %v", err)
	}
}

func TestAugmentWithTestAssertions_NoOp_WhenNoScenarioTestFiles(t *testing.T) {
	workDir := t.TempDir()
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{Name: "some scenario"},
		},
	}
	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("expected nil error when no test files exist, got %v", err)
	}
}

func TestAugmentWithTestAssertions_InjectsGoTestPassAssertions(t *testing.T) {
	workDir := t.TempDir()

	// Create scenario test file with TestScenario_Foo function
	testCode := `package test

import "testing"

func TestScenario_Foo(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(workDir, "example_scenario_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write scenario test: %v", err)
	}

	// Create contract with scenario named "foo"
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "foo",
				Assertions: []ContractAssertion{
					{FileContains: &FileContainsAssertion{Path: "file.txt", Pattern: "pattern"}},
				},
			},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// Verify: file_contains should be dropped, go_test_pass should be injected
	if len(sc.Scenarios[0].Assertions) != 1 {
		t.Fatalf("expected 1 assertion after augmentation, got %d", len(sc.Scenarios[0].Assertions))
	}

	assertion := sc.Scenarios[0].Assertions[0]
	if assertion.GoTestPass == nil {
		t.Fatalf("expected GoTestPass assertion, got %+v", assertion)
	}
	if assertion.GoTestPass.TestName != "TestScenario_Foo" {
		t.Fatalf("expected TestName=TestScenario_Foo, got %s", assertion.GoTestPass.TestName)
	}
}

func TestAugmentWithTestAssertions_DropsContentAssertionsWhenGoTestPassExists(t *testing.T) {
	workDir := t.TempDir()

	// File pattern requires "_scenario_" in the name
	testCode := `package test
import "testing"
func TestScenario_Drop(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(workDir, "example_scenario_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "drop",
				Assertions: []ContractAssertion{
					{FileContains: &FileContainsAssertion{Path: "file.txt", Pattern: "pattern"}},
					{FileNotContains: &FileContainsAssertion{Path: "other.txt", Pattern: "not-this"}},
				},
			},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// Both content assertions should be dropped, replaced by go_test_pass
	if len(sc.Scenarios[0].Assertions) != 1 {
		t.Fatalf("expected 1 assertion after augmentation, got %d", len(sc.Scenarios[0].Assertions))
	}
	if sc.Scenarios[0].Assertions[0].GoTestPass == nil {
		t.Fatalf("expected GoTestPass assertion")
	}
}

func TestAugmentWithTestAssertions_PreservesStructuralAssertions(t *testing.T) {
	workDir := t.TempDir()

	// Create scenario test file
	testCode := `package test
import "testing"
func TestScenario_Bar(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(workDir, "example_scenario_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	// Create contract with mixed assertions
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "bar",
				Assertions: []ContractAssertion{
					{FileExists: "must_exist.txt"},
					{FileContains: &FileContainsAssertion{Path: "file.txt", Pattern: "drop me"}},
					{FileNotModified: "stable.txt"},
				},
			},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// Verify: file_exists and file_not_modified preserved, file_contains dropped, go_test_pass added
	if len(sc.Scenarios[0].Assertions) != 3 {
		t.Fatalf("expected 3 assertions (2 structural + 1 go_test_pass), got %d", len(sc.Scenarios[0].Assertions))
	}

	if sc.Scenarios[0].Assertions[0].FileExists == "" {
		t.Fatalf("expected FileExists preserved at index 0")
	}
	if sc.Scenarios[0].Assertions[1].FileNotModified == "" {
		t.Fatalf("expected FileNotModified preserved at index 1")
	}
	if sc.Scenarios[0].Assertions[2].GoTestPass == nil {
		t.Fatalf("expected GoTestPass injected at index 2")
	}
}

func TestAugmentWithTestAssertions_MatchesScenarioByNormalizedName(t *testing.T) {
	workDir := t.TempDir()

	// Create test with normalized name matching scenario
	testCode := `package test
import "testing"
func TestScenario_Build_And_Run(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(workDir, "example_scenario_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	// Scenario name has different spacing/punctuation
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "build-and-run",
				Assertions: []ContractAssertion{
					{FileContains: &FileContainsAssertion{Path: "file.txt", Pattern: "pattern"}},
				},
			},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// Verify match succeeded
	if len(sc.Scenarios[0].Assertions) != 1 || sc.Scenarios[0].Assertions[0].GoTestPass == nil {
		t.Fatalf("expected test to match scenario by normalized name")
	}
}

func TestAugmentWithTestAssertions_SkipsNonScenarioTestFiles(t *testing.T) {
	workDir := t.TempDir()

	// Create non-scenario test file (won't match "_scenario_" pattern)
	testCode := `package test
import "testing"
func TestScenario_Something(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(workDir, "regular_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "something",
				Assertions: []ContractAssertion{
					{FileContains: &FileContainsAssertion{Path: "file.txt", Pattern: "pattern"}},
				},
			},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// Verify: no augmentation occurred (file must contain "_scenario_" pattern)
	if len(sc.Scenarios[0].Assertions) != 1 || sc.Scenarios[0].Assertions[0].FileContains == nil {
		t.Fatalf("expected no augmentation when test file doesn't match _scenario_ pattern")
	}
}

func TestAugmentWithTestAssertions_SkipsNonTestScenarioFunctions(t *testing.T) {
	workDir := t.TempDir()

	// Create scenario test file with non-TestScenario function
	testCode := `package test
import "testing"
func TestHelper_Foo(t *testing.T) {}
func TestScenario_Bar(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(workDir, "example_scenario_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{Name: "helper"},
			{Name: "bar"},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// Verify: only bar matched, helper didn't
	if len(sc.Scenarios[0].Assertions) != 0 {
		t.Fatalf("expected no assertions for 'helper' scenario")
	}
	if len(sc.Scenarios[1].Assertions) != 1 || sc.Scenarios[1].Assertions[0].GoTestPass == nil {
		t.Fatalf("expected go_test_pass for 'bar' scenario")
	}
}

func TestAugmentWithTestAssertions_SortsByPackageAndTestName(t *testing.T) {
	workDir := t.TempDir()

	// Create multiple test files in different packages
	pkg1Dir := filepath.Join(workDir, "pkg1")
	pkg2Dir := filepath.Join(workDir, "pkg2")
	if err := os.MkdirAll(pkg1Dir, 0o755); err != nil {
		t.Fatalf("mkdir pkg1: %v", err)
	}
	if err := os.MkdirAll(pkg2Dir, 0o755); err != nil {
		t.Fatalf("mkdir pkg2: %v", err)
	}

	testCode1 := `package pkg1
import "testing"
func TestScenario_Foo(t *testing.T) {}
`
	testCode2 := `package pkg2
import "testing"
func TestScenario_Foo(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(pkg2Dir, "a_scenario_test.go"), []byte(testCode2), 0o644); err != nil {
		t.Fatalf("write pkg2 test: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg1Dir, "b_scenario_test.go"), []byte(testCode1), 0o644); err != nil {
		t.Fatalf("write pkg1 test: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{Name: "foo"},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// Verify: assertions sorted by package then test name
	assertions := sc.Scenarios[0].Assertions
	if len(assertions) != 2 {
		t.Fatalf("expected 2 go_test_pass assertions, got %d", len(assertions))
	}

	// Verify package patterns include .../ suffix for recursive matching
	if !strings.Contains(assertions[0].GoTestPass.Pkg, "/...") {
		t.Fatalf("expected package pattern to include /..., got %s", assertions[0].GoTestPass.Pkg)
	}

	// pkg1 should come before pkg2 lexicographically
	if assertions[0].GoTestPass.Pkg >= assertions[1].GoTestPass.Pkg {
		t.Fatalf("expected pkg1 before pkg2, got %s and %s", assertions[0].GoTestPass.Pkg, assertions[1].GoTestPass.Pkg)
	}
}

func TestAugmentWithTestAssertions_ErrorsOnNonExistentWorkDir(t *testing.T) {
	nonExistentDir := filepath.Join(t.TempDir(), "nonexistent")
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{Name: "test"}},
	}

	// WalkDir errors on non-existent directory
	err := AugmentWithTestAssertions(sc, nonExistentDir)
	if err == nil {
		t.Fatalf("expected error for non-existent workdir, got nil")
	}
}

func TestAugmentWithTestAssertions_SkipsScenarioTestsOutsideWorkDir(t *testing.T) {
	workDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create test file outside workDir
	testCode := `package test
import "testing"
func TestScenario_Foo(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(outsideDir, "scenario_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{Name: "foo"}},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// No augmentation should occur
	if len(sc.Scenarios[0].Assertions) != 0 {
		t.Fatalf("expected no assertions when test is outside workdir")
	}
}

func TestAugmentWithTestAssertions_DropsFileNotContains(t *testing.T) {
	workDir := t.TempDir()

	testCode := `package test
import "testing"
func TestScenario_Baz(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(workDir, "example_scenario_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "baz",
				Assertions: []ContractAssertion{
					{FileNotContains: &FileContainsAssertion{Path: "file.txt", Pattern: "drop me"}},
				},
			},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// file_not_contains should be dropped when go_test_pass exists
	if len(sc.Scenarios[0].Assertions) != 1 || sc.Scenarios[0].Assertions[0].GoTestPass == nil {
		t.Fatalf("expected file_not_contains to be dropped and replaced with go_test_pass")
	}
}

func TestAugmentWithTestAssertions_MultipleTestsPerScenario(t *testing.T) {
	workDir := t.TempDir()

	// Must use names that match the "_scenario_" pattern
	testCode := `package test
import "testing"
func TestScenario_Multi(t *testing.T) {}
func TestScenario_Multi_Integration(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(workDir, "example_scenario_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "multi",
				Assertions: []ContractAssertion{
					{FileContains: &FileContainsAssertion{Path: "file.txt", Pattern: "pattern"}},
				},
			},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// Both tests should be included and sorted by test name
	if len(sc.Scenarios[0].Assertions) != 2 {
		t.Fatalf("expected 2 go_test_pass assertions, got %d", len(sc.Scenarios[0].Assertions))
	}

	if sc.Scenarios[0].Assertions[0].GoTestPass.TestName != "TestScenario_Multi" {
		t.Fatalf("expected first test to be TestScenario_Multi")
	}
	if sc.Scenarios[0].Assertions[1].GoTestPass.TestName != "TestScenario_Multi_Integration" {
		t.Fatalf("expected second test to be TestScenario_Multi_Integration")
	}
}

func TestAugmentWithTestAssertions_SkipsAmbiguousBestMatch(t *testing.T) {
	workDir := t.TempDir()

	// Create a test that matches multiple scenarios equally well
	testCode := `package test
import "testing"
func TestScenario_Foo(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(workDir, "example_scenario_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	// Two scenarios with equal normalized match to the test: "foo" and "foo_something"
	// Test "Foo" has highest match to both when scored, causing a tie
	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "foo",
				Assertions: []ContractAssertion{
					{FileContains: &FileContainsAssertion{Path: "file1.txt", Pattern: "pattern1"}},
				},
			},
			{
				Name: "foo_other",
				Assertions: []ContractAssertion{
					{FileContains: &FileContainsAssertion{Path: "file2.txt", Pattern: "pattern2"}},
				},
			},
		},
	}

	err := AugmentWithTestAssertions(sc, workDir)
	if err != nil {
		t.Fatalf("augment failed: %v", err)
	}

	// When there's a tie, neither scenario should be augmented (skip ambiguous match)
	// Both should retain their original assertions
	if len(sc.Scenarios[0].Assertions) != 1 || sc.Scenarios[0].Assertions[0].FileContains == nil {
		t.Fatalf("expected first scenario to retain original file_contains assertion when match is ambiguous")
	}
	if len(sc.Scenarios[1].Assertions) != 1 || sc.Scenarios[1].Assertions[0].FileContains == nil {
		t.Fatalf("expected second scenario to retain original file_contains assertion when match is ambiguous")
	}
}
