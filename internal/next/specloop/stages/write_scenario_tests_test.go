package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// fakeScenarioTestWriter is a test double for the ScenarioTestWriter interface.
type fakeScenarioTestWriter struct {
	calls               int
	failAttempt         int   // -1 means never fail, N means fail on attempt N (0-indexed)
	failErr             error // error to return on failAttempt; nil means use generic error
	returnedPaths       []string
	returnedPathIndex   int
	compilableScenarios map[string]bool // scenarios that will compile
}

func (m *fakeScenarioTestWriter) WriteScenarioTest(
	ctx context.Context,
	scenario contract.SpecScenario,
	implFiles []string,
	workDir string,
	compileErrors string,
) (testFilePath string, err error) {
	defer func() { m.calls++ }()

	// Check if this attempt should fail
	if m.failAttempt >= 0 && m.calls == m.failAttempt {
		if m.failErr != nil {
			return "", m.failErr
		}
		return "", fmt.Errorf("mock writer simulated error on attempt %d", m.calls)
	}

	// Return a pre-prepared path if available
	if m.returnedPathIndex < len(m.returnedPaths) {
		path := m.returnedPaths[m.returnedPathIndex]
		m.returnedPathIndex++

		// Ensure the directory exists
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create test directory %s: %w", dir, err)
		}

		// Create a minimal Go test file that may or may not compile
		if m.compilableScenarios[scenario.Name] {
			// Compilable test file
			testCode := fmt.Sprintf(`package main

import "testing"

func TestScenario_%s(t *testing.T) {
	t.Log("scenario: %s")
}
`, escapeIdentifier(scenario.Name), scenario.Name)
			if err := os.WriteFile(path, []byte(testCode), 0o644); err != nil {
				return "", fmt.Errorf("write test file: %w", err)
			}
		} else {
			// Non-compilable test file (invalid syntax)
			testCode := fmt.Sprintf(`package main
func TestScenario_%s(t *testing.T { // Missing closing paren
}
`, escapeIdentifier(scenario.Name))
			if err := os.WriteFile(path, []byte(testCode), 0o644); err != nil {
				return "", fmt.Errorf("write test file: %w", err)
			}
		}
		return path, nil
	}

	// Fallback: return empty path (deliberate no-op as per implementation)
	return "", nil
}

func escapeIdentifier(s string) string {
	// Replace non-alphanumeric with underscore for valid Go identifiers
	result := ""
	for _, ch := range s {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			result += string(ch)
		} else {
			result += "_"
		}
	}
	return result
}

const specWithTwoScenarios = `# Test Spec

## Scenarios

### Scenario: scenario-one
**Given:** precondition one
**When:** action one
**Then:** outcome one

### Scenario: scenario-two
**When:** action two
**Then:** outcome two
`

const specWithoutScenarioTests = `# Test Spec

## Overview
No scenarios in this spec.
`

func makeWriteScenarioTestsRunState(t *testing.T) *runstore.RunState {
	t.Helper()
	rs := runstore.NewRunState("spec-scenario-test", "proj-scenario-test")
	return rs
}

func TestWriteScenarioTests_IdempotencyNoOp(t *testing.T) {
	// When ScenarioTestsWritten is true, returns Continue without calling writer
	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)
	rs.ScenarioTestsWritten = true

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	writer := &fakeScenarioTestWriter{failAttempt: -1}
	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: filepath.Join(tmp, "evidence"),
		Store:       nil,
		WorkDir:     tmp,
	}
	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if writer.calls != 0 {
		t.Fatalf("expected 0 writer calls for idempotent run, got %d", writer.calls)
	}
	if !rs.ScenarioTestsWritten {
		t.Fatal("expected ScenarioTestsWritten to remain true")
	}
}

func TestWriteScenarioTests_NoScenariosReturnsContinue(t *testing.T) {
	// When spec has no scenarios, returns Continue with no writer calls
	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithoutScenarioTests), 0o644); err != nil {
		t.Fatal(err)
	}

	writer := &fakeScenarioTestWriter{failAttempt: -1}
	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: filepath.Join(tmp, "evidence"),
		Store:       nil,
		WorkDir:     tmp,
	}
	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue for no-scenarios, got %v", action.Kind)
	}
	if writer.calls != 0 {
		t.Fatalf("expected 0 writer calls for no-scenarios, got %d", writer.calls)
	}
	if rs.ScenarioTestsWritten {
		t.Fatal("expected ScenarioTestsWritten to remain false for no-scenarios")
	}
}

func TestWriteScenarioTests_HappyPath(t *testing.T) {
	// Happy path: writes tests for 2 scenarios, sets flag, emits events, writes manifest
	// Use testdata subdirectory within the actual gromit project for compilation to work
	currentDir := os.Getenv("PWD")
	if currentDir == "" {
		currentDir = "."
	}

	testDataDir := filepath.Join(currentDir, "internal/next/specloop/stages/testdata")
	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(testDataDir)
	})

	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(testDataDir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Prepare test file paths
	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
	testFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")

	// Create writer that makes both scenarios compilable
	writer := &fakeScenarioTestWriter{
		failAttempt:   -1,
		returnedPaths: []string{testFile1, testFile2},
		compilableScenarios: map[string]bool{
			"scenario-one": true,
			"scenario-two": true,
		},
	}

	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       nil,
		WorkDir:     currentDir,
	}

	stage := NewWriteScenarioTestsStage(writer, cfg, nil, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		msg := "unknown reason"
		if action.Context != nil && len(action.Context.Failures) > 0 {
			// Print the full failure message
			for i, f := range action.Context.Failures {
				t.Logf("Failure %d: %s", i, f)
			}
			msg = action.Context.Failures[0]
		}
		t.Fatalf("expected Continue, got %v: %s", action.Kind, msg)
	}
	if !rs.ScenarioTestsWritten {
		t.Fatal("expected ScenarioTestsWritten=true after success")
	}

	// Check manifest was written
	manifestPath := filepath.Join(evidenceDir, "scenario-test-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}

	var manifest contract.ScenarioTestManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if len(manifest.Scenarios) != 2 {
		t.Fatalf("expected 2 scenarios in manifest, got %d", len(manifest.Scenarios))
	}

	// Check scenario entries
	scenarioMap := make(map[string]string)
	for _, entry := range manifest.Scenarios {
		scenarioMap[entry.Name] = entry.TestFile
	}

	if _, ok := scenarioMap["scenario-one"]; !ok {
		t.Fatal("expected scenario-one in manifest")
	}
	if _, ok := scenarioMap["scenario-two"]; !ok {
		t.Fatal("expected scenario-two in manifest")
	}

	// Check events were emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var testWrittenCount, completeCount int
	for _, ev := range events {
		if ev.EventType() == "scenario_tests_written" {
			testWrittenCount++
		} else if ev.EventType() == "scenario_tests_complete" {
			completeCount++
		}
	}

	if testWrittenCount != 2 {
		t.Fatalf("expected 2 scenario_tests_written events, got %d", testWrittenCount)
	}
	if completeCount != 1 {
		t.Fatalf("expected 1 scenario_tests_complete event, got %d", completeCount)
	}
}

func TestWriteScenarioTests_CompileFailureSelfRepair(t *testing.T) {
	// First attempt fails compilation, retry succeeds
	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")

	// Writer will be called multiple times — first fails, then succeeds
	writer := &fakeScenarioTestWriter{
		failAttempt:   -1,
		returnedPaths: []string{testFile1, testFile1}, // return same path twice
		compilableScenarios: map[string]bool{
			"scenario-one": true, // will compile on second attempt
		},
	}

	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       nil,
		WorkDir:     tmp,
	}

	stage := NewWriteScenarioTestsStage(writer, cfg, nil, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue after self-repair, got %v", action.Kind)
	}
	if !rs.ScenarioTestsWritten {
		t.Fatal("expected ScenarioTestsWritten=true after successful repair")
	}

	// Should have retried (at least 2 calls for first scenario)
	if writer.calls < 2 {
		t.Fatalf("expected at least 2 writer calls for retry, got %d", writer.calls)
	}
}

func TestWriteScenarioTests_CompileFailureExhausted(t *testing.T) {
	// All 3 attempts fail compilation, returns Blocked, flag not set
	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")

	writer := &fakeScenarioTestWriter{
		failAttempt:         -1,
		returnedPaths:       []string{testFile1, testFile1, testFile1}, // 3 attempts
		compilableScenarios: make(map[string]bool),                     // empty = never compilable
	}

	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       nil,
		WorkDir:     tmp,
	}

	stage := NewWriteScenarioTestsStage(writer, cfg, nil, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked after exhausted retries, got %v", action.Kind)
	}
	if rs.ScenarioTestsWritten {
		t.Fatal("expected ScenarioTestsWritten=false after failure")
	}

	// Should have exactly 3 writer calls (0, 1, 2)
	if writer.calls != 3 {
		t.Fatalf("expected 3 writer calls (maxRetries=2 means 3 attempts), got %d", writer.calls)
	}

	// Should have emitted a blocked event
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var blockedCount int
	for _, ev := range events {
		if ev.EventType() == "scenario_tests_blocked" {
			blockedCount++
		}
	}
	if blockedCount != 1 {
		t.Fatalf("expected 1 scenario_tests_blocked event, got %d", blockedCount)
	}
}

func TestWriteScenarioTests_BudgetExhausted(t *testing.T) {
	// Budget exceeded mid-iteration returns Blocked
	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")

	writer := &fakeScenarioTestWriter{
		failAttempt:   -1,
		returnedPaths: []string{testFile1},
		compilableScenarios: map[string]bool{
			"scenario-one": true,
		},
	}

	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// Create a budget that's already exceeded
	budgetLimits := execpolicy.Budgets{
		MaxRunCostUSD: 100.0,
		MaxSpecCycles: 5,
	}
	budget := specloop.NewBudget(budgetLimits)
	budget.AddCost(150.0) // Exceed the budget

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       nil,
		WorkDir:     tmp,
	}
	stage := NewWriteScenarioTestsStage(writer, cfg, budget, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked when budget exceeded, got %v", action.Kind)
	}
	if rs.ScenarioTestsWritten {
		t.Fatal("expected ScenarioTestsWritten=false when budget exhausted")
	}
	if writer.calls > 0 {
		t.Fatalf("expected 0 writer calls when budget already exceeded, got %d", writer.calls)
	}

	// Check for blocked event
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	var blockedCount int
	for _, ev := range events {
		if ev.EventType() == "scenario_tests_blocked" {
			blockedCount++
		}
	}
	if blockedCount != 1 {
		t.Fatalf("expected 1 scenario_tests_blocked event, got %d", blockedCount)
	}
}

func TestWriteScenarioTests_PartialRecovery(t *testing.T) {
	// Manifest exists with completed scenario, skips it on retry
	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use testdata directory for compilation to work
	currentDir := os.Getenv("PWD")
	if currentDir == "" {
		currentDir = "."
	}

	testDataDir := filepath.Join(currentDir, "internal/next/specloop/stages/testdata_partial")
	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(testDataDir)
	})

	evidenceDir := filepath.Join(testDataDir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Pre-write manifest with first scenario already done
	manifestPath := filepath.Join(evidenceDir, "scenario-test-manifest.json")
	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")

	// Write a valid test file for scenario-one
	testCode := `package main
import "testing"
func TestScenario_scenario_one(t *testing.T) {
	t.Log("scenario one")
}
`
	if err := os.WriteFile(testFile1, []byte(testCode), 0o644); err != nil {
		t.Fatal(err)
	}

	existingManifest := contract.ScenarioTestManifest{
		Scenarios: []contract.ScenarioTestEntry{
			{Name: "scenario-one", TestFile: testFile1},
		},
	}
	data, err := json.Marshal(existingManifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	testFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")

	writer := &fakeScenarioTestWriter{
		failAttempt:   -1,
		returnedPaths: []string{testFile2}, // Only one new file needed
		compilableScenarios: map[string]bool{
			"scenario-two": true,
		},
	}

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       nil,
		WorkDir:     currentDir,
	}

	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Should only have written one scenario (skipped the first)
	if writer.calls != 1 {
		t.Fatalf("expected 1 writer call (skipped first scenario), got %d", writer.calls)
	}

	// Final manifest should have both scenarios
	finalData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var finalManifest contract.ScenarioTestManifest
	if err := json.Unmarshal(finalData, &finalManifest); err != nil {
		t.Fatal(err)
	}

	if len(finalManifest.Scenarios) != 2 {
		t.Fatalf("expected 2 scenarios in final manifest, got %d", len(finalManifest.Scenarios))
	}
}

func TestWriteScenarioTests_SeparateFilesPerScenario(t *testing.T) {
	// Each scenario gets its own file
	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
	testFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")

	writer := &fakeScenarioTestWriter{
		failAttempt:   -1, // Never fail
		returnedPaths: []string{testFile1, testFile2},
		compilableScenarios: map[string]bool{
			"scenario-one": true,
			"scenario-two": true,
		},
	}

	// Use testdata directory for compilation to work
	currentDir := os.Getenv("PWD")
	if currentDir == "" {
		currentDir = "."
	}

	testDataDir := filepath.Join(currentDir, "internal/next/specloop/stages/testdata_separate")
	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(testDataDir)
	})

	evidenceDirFinal := filepath.Join(testDataDir, "evidence")
	if err := os.MkdirAll(evidenceDirFinal, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile1Final := filepath.Join(evidenceDirFinal, "scenario_one_test.go")
	testFile2Final := filepath.Join(evidenceDirFinal, "scenario_two_test.go")

	writer.returnedPaths = []string{testFile1Final, testFile2Final}

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDirFinal,
		Store:       nil,
		WorkDir:     currentDir,
	}

	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	// Check files are different
	if testFile1 == testFile2 {
		t.Fatal("test files should be different paths")
	}

	// Check manifest records both files
	manifestPath := filepath.Join(evidenceDirFinal, "scenario-test-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	var manifest contract.ScenarioTestManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}

	var files []string
	for _, entry := range manifest.Scenarios {
		files = append(files, entry.TestFile)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 test files, got %d", len(files))
	}
	if files[0] == files[1] {
		t.Fatal("test files should be unique paths")
	}

	// Verify files exist on disk
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("test file not found: %s: %v", f, err)
		}
	}
}

func TestWriteScenarioTests_Name(t *testing.T) {
	// Name() returns 'write_scenario_tests'
	tmp := t.TempDir()
	writer := &fakeScenarioTestWriter{}
	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    filepath.Join(tmp, "spec.md"),
		EvidenceDir: filepath.Join(tmp, "evidence"),
		Store:       nil,
		WorkDir:     tmp,
	}
	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	if name := stage.Name(); name != "write_scenario_tests" {
		t.Fatalf("expected Name() to return 'write_scenario_tests', got %q", name)
	}
}

func TestWriteScenarioTests_StaleNonCompilingFileDeleted(t *testing.T) {
	// Manifest entry pointing to non-compiling file causes delete+rewrite, not skip
	currentDir := os.Getenv("PWD")
	if currentDir == "" {
		currentDir = "."
	}

	testDataDir := filepath.Join(currentDir, "internal/next/specloop/stages/testdata_stale")
	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(testDataDir)
	})

	evidenceDir := filepath.Join(testDataDir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a stale non-compiling file for scenario-one
	staleFile := filepath.Join(evidenceDir, "scenario_one_stale_test.go")
	staleCode := `package main
func TestScenario_scenario_one(t *testing.T { // Missing closing paren
}
`
	if err := os.WriteFile(staleFile, []byte(staleCode), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-populate manifest pointing to the stale file
	manifestPath := filepath.Join(evidenceDir, "scenario-test-manifest.json")
	existingManifest := contract.ScenarioTestManifest{
		Scenarios: []contract.ScenarioTestEntry{
			{Name: "scenario-one", TestFile: staleFile},
		},
	}
	data, err := json.Marshal(existingManifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Writer returns a fresh compilable file for scenario-one, then one for scenario-two
	newFile1 := filepath.Join(evidenceDir, "scenario_one_new_test.go")
	newFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")

	writer := &fakeScenarioTestWriter{
		failAttempt:   -1,
		returnedPaths: []string{newFile1, newFile2},
		compilableScenarios: map[string]bool{
			"scenario-one": true,
			"scenario-two": true,
		},
	}

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       nil,
		WorkDir:     currentDir,
	}
	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		msg := "unknown reason"
		if action.Context != nil && len(action.Context.Failures) > 0 {
			msg = action.Context.Failures[0]
		}
		t.Fatalf("expected Continue, got %v: %s", action.Kind, msg)
	}
	if !rs.ScenarioTestsWritten {
		t.Fatal("expected ScenarioTestsWritten=true after successful rewrite")
	}

	// Writer must have been called (stale file was not skipped)
	if writer.calls == 0 {
		t.Fatal("expected writer to be called for stale scenario, got 0 calls")
	}

	// Stale file should have been removed from disk
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Fatal("expected stale file to be deleted from disk")
	}

	// Final manifest should contain scenario-one with the new file
	finalData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not found: %v", err)
	}
	var finalManifest contract.ScenarioTestManifest
	if err := json.Unmarshal(finalData, &finalManifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	scenarioMap := make(map[string]string)
	for _, entry := range finalManifest.Scenarios {
		scenarioMap[entry.Name] = entry.TestFile
	}
	if _, ok := scenarioMap["scenario-one"]; !ok {
		t.Fatal("expected scenario-one in final manifest")
	}
	if scenarioMap["scenario-one"] == staleFile {
		t.Fatal("expected scenario-one manifest entry to point to new file, not stale file")
	}
}

func TestWriteScenarioTests_IndependentOfContractArtifacts(t *testing.T) {
	// Stage runs normally even when no scenario-contracts.yaml exists in evidence dir
	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithoutScenarioTests), 0o644); err != nil {
		t.Fatal(err)
	}

	// Evidence dir exists but has no scenario-contracts.yaml
	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writer := &fakeScenarioTestWriter{failAttempt: -1}
	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		Store:       nil,
		WorkDir:     tmp,
	}
	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue without contract artifacts, got %v", action.Kind)
	}
}

func TestWriteScenarioTests_ParseErrorRetried(t *testing.T) {
	// Parse error on attempt 0 is treated as retryable; attempt 1 succeeds.
	// Use the actual project directory as WorkDir so that compilation works.
	currentDir := os.Getenv("PWD")
	if currentDir == "" {
		currentDir = "."
	}

	testDataDir := filepath.Join(currentDir, "internal/next/specloop/stages/testdata/parse-retry")
	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(testDataDir)
	})

	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := testDataDir
	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
	testFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")

	parseErr := fmt.Errorf("parse scenario test response: response missing ===TEST_FILE_PATH=== marker")

	writer := &fakeScenarioTestWriter{
		failAttempt:   0,
		failErr:       parseErr,
		returnedPaths: []string{testFile1, testFile2},
		compilableScenarios: map[string]bool{
			"scenario-one": true,
			"scenario-two": true,
		},
	}

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		WorkDir:     currentDir,
	}
	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind == specloop.Blocked {
		failures := ""
		if action.Context != nil {
			failures = strings.Join(action.Context.Failures, "; ")
		}
		t.Fatalf("expected Continue after parse-error retry, got Blocked: %s", failures)
	}
	// writer should have been called at least twice (fail then succeed for scenario-one)
	if writer.calls < 2 {
		t.Fatalf("expected at least 2 writer calls, got %d", writer.calls)
	}
}

// Verify WriteScenarioTestsStage satisfies the Stage interface
var _ specloop.Stage = (*WriteScenarioTestsStage)(nil)
