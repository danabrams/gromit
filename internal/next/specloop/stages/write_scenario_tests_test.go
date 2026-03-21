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
	receivedWorkDirs    []string        // workDir values passed to WriteScenarioTest
}

func (m *fakeScenarioTestWriter) WriteScenarioTest(
	ctx context.Context,
	scenario contract.SpecScenario,
	implFiles []string,
	workDir string,
	compileErrors string,
) (testFilePath string, err error) {
	defer func() { m.calls++ }()
	m.receivedWorkDirs = append(m.receivedWorkDirs, workDir)

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

// TestWriteScenarioTests_CompileDir_UsedForCompilation verifies that when CompileDir
// is set, compilation runs in CompileDir rather than WorkDir.
//
// The test creates two temp dirs:
//   - workDir: where the test file is written (simulates the main repo, which lacks the new function)
//   - compileDir: where compilation runs (simulates the worktree, which has the new function)
//
// The fake writer writes a valid compilable test into workDir. The stage must use
// compileDir for `go test -c`, so the package path derived from the test file location
// (relative to workDir) is applied inside compileDir. Since both dirs share the same
// sub-directory layout, deriving the relative path from workDir is valid for compileDir.
//
// Because both dirs are temporary and not real Go modules, we piggyback on the actual
// project tree: workDir = a "stale" subdirectory that has no new function, and
// compileDir = the real project root (which already compiles). The test file is written
// into the real project tree under a unique subdir, and we verify that:
//  1. When CompileDir == "" (fallback), compilation uses WorkDir.
//  2. When CompileDir is set, compilation uses CompileDir.
func TestWriteScenarioTests_CompileDir_UsedWhenSet(t *testing.T) {
	currentDir := os.Getenv("PWD")
	if currentDir == "" {
		currentDir = "."
	}

	const singleScenario = `# Test Spec

## Scenarios

### Scenario: scenario-one
**Given:** precondition
**When:** action
**Then:** outcome
`

	t.Run("succeeds when CompileDir points to real module", func(t *testing.T) {
		// compileDir is the real project root — known to compile.
		compileDir := currentDir

		// workDir is a subdirectory that does NOT form a valid Go module on its own,
		// so using it as the compile dir would fail. We set it to a temp dir so that
		// any accidental `go test -c` run in workDir would produce an error.
		workDir := t.TempDir()

		// Write the test file into a sub-package of compileDir so `go test -c` works.
		testDataDir := filepath.Join(compileDir, "internal/next/specloop/stages/testdata_compiledir")
		if err := os.MkdirAll(testDataDir, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(testDataDir) })

		testFilePath := filepath.Join(testDataDir, "scenario_compiledir_test.go")
		testCode := `package stages_test

import "testing"

func TestScenario_compiledir(t *testing.T) {
	t.Log("compiledir scenario")
}
`
		if err := os.WriteFile(testFilePath, []byte(testCode), 0o644); err != nil {
			t.Fatal(err)
		}

		// fakeWriter that returns the pre-written test file path.
		writer := &fakeScenarioTestWriter{
			failAttempt:   -1,
			returnedPaths: []string{testFilePath},
			compilableScenarios: map[string]bool{
				"scenario-one": true,
			},
		}

		tmp := t.TempDir()
		specPath := filepath.Join(tmp, "spec.md")
		if err := os.WriteFile(specPath, []byte(singleScenario), 0o644); err != nil {
			t.Fatal(err)
		}

		evidenceDir := filepath.Join(tmp, "evidence")
		if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
			t.Fatal(err)
		}

		rs := makeWriteScenarioTestsRunState(t)

		// The test file path is absolute and lives under compileDir. WorkDir is a plain
		// tmp dir (not a Go module). With CompileDir set to currentDir, derivePackagePath
		// will compute the relative path from the test file to CompileDir and run
		// `go test -c` there — which should succeed.
		cfg := WriteScenarioTestsStageConfig{
			SpecPath:    specPath,
			EvidenceDir: evidenceDir,
			WorkDir:     workDir,
			CompileDir:  compileDir,
		}
		stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

		action, err := stage.Run(context.Background(), rs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action.Kind != specloop.Continue {
			msg := "unknown"
			if action.Context != nil && len(action.Context.Failures) > 0 {
				msg = action.Context.Failures[0]
			}
			t.Fatalf("expected Continue when CompileDir is set, got %v: %s", action.Kind, msg)
		}
		if !rs.ScenarioTestsWritten {
			t.Fatal("expected ScenarioTestsWritten=true after CompileDir-based success")
		}
	})

	t.Run("blocked when CompileDir empty and WorkDir has no module", func(t *testing.T) {
		// WorkDir is a plain temp dir — not a Go module. CompileDir is empty, so
		// compilesSuccessfully() falls back to WorkDir. Since WorkDir has no go.mod,
		// `go test -c` will fail, and the stage must exhaust all retries and return Blocked.

		// Write the test file into a subdirectory of WorkDir (not the real project tree)
		// so that the package path derivation works relative to WorkDir.
		workDir := t.TempDir()

		testDataDir := filepath.Join(workDir, "internal/next/specloop/stages/testdata_compiledir_empty")
		if err := os.MkdirAll(testDataDir, 0o755); err != nil {
			t.Fatal(err)
		}

		testFilePath := filepath.Join(testDataDir, "scenario_compiledir_empty_test.go")
		testCode := `package stages_test

import "testing"

func TestScenario_compiledir_empty(t *testing.T) {
	t.Log("compiledir empty scenario")
}
`
		if err := os.WriteFile(testFilePath, []byte(testCode), 0o644); err != nil {
			t.Fatal(err)
		}

		// fakeWriter returns the same path on every call so all 3 attempts use the
		// same (syntactically valid but non-compilable-in-workDir) file.
		writer := &fakeScenarioTestWriter{
			failAttempt:   -1,
			returnedPaths: []string{testFilePath, testFilePath, testFilePath},
			compilableScenarios: map[string]bool{
				"scenario-one": true,
			},
		}

		tmp := t.TempDir()
		specPath := filepath.Join(tmp, "spec.md")
		if err := os.WriteFile(specPath, []byte(singleScenario), 0o644); err != nil {
			t.Fatal(err)
		}

		evidenceDir := filepath.Join(tmp, "evidence")
		if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
			t.Fatal(err)
		}

		rs := makeWriteScenarioTestsRunState(t)

		// CompileDir is intentionally empty — should fall back to WorkDir (a plain
		// temp dir with no go.mod), causing compilation to fail every attempt.
		cfg := WriteScenarioTestsStageConfig{
			SpecPath:    specPath,
			EvidenceDir: evidenceDir,
			WorkDir:     workDir,
			CompileDir:  "",
		}
		stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

		action, err := stage.Run(context.Background(), rs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action.Kind != specloop.Blocked {
			t.Fatalf("expected Blocked when CompileDir is empty and WorkDir has no module, got %v", action.Kind)
		}
		if rs.ScenarioTestsWritten {
			t.Fatal("expected ScenarioTestsWritten=false after compilation failure")
		}
	})
}

// TestWriteScenarioTests_CompileDir_FallsBackToWorkDir verifies that when CompileDir
// is empty, the stage uses WorkDir for compilation (backward-compatible behaviour).
func TestWriteScenarioTests_CompileDir_FallsBackToWorkDir(t *testing.T) {
	// compileDir() should return WorkDir when CompileDir is empty.
	tmp := t.TempDir()
	cfg := WriteScenarioTestsStageConfig{
		WorkDir:    tmp,
		CompileDir: "",
	}
	stage := &WriteScenarioTestsStage{cfg: cfg}
	if got := stage.compileDir(); got != tmp {
		t.Fatalf("expected compileDir() == WorkDir %q, got %q", tmp, got)
	}
}

// TestWriteScenarioTests_CompileDir_ReturnedWhenSet verifies that compileDir()
// returns CompileDir when it is non-empty.
func TestWriteScenarioTests_CompileDir_ReturnedWhenSet(t *testing.T) {
	tmp := t.TempDir()
	compileDir := "/some/other/path"
	cfg := WriteScenarioTestsStageConfig{
		WorkDir:    tmp,
		CompileDir: compileDir,
	}
	stage := &WriteScenarioTestsStage{cfg: cfg}
	if got := stage.compileDir(); got != compileDir {
		t.Fatalf("expected compileDir() == %q, got %q", compileDir, got)
	}
}

func TestWriteScenarioTests_UsesWorktreePathWhenSet(t *testing.T) {
	// When rs.WorktreePath is set, the writer should receive the worktree path
	// as workDir, not the config WorkDir. This prevents files from being written
	// to the main repo when a worktree is active.
	mainDir := t.TempDir()
	worktreeDir := t.TempDir()

	specPath := filepath.Join(mainDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(mainDir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(worktreeDir, "cmd/gromit-next/scenario_test.go")
	writer := &fakeScenarioTestWriter{
		failAttempt:         -1,
		returnedPaths:       []string{testFile, testFile},
		compilableScenarios: map[string]bool{"scenario-one": true, "scenario-two": true},
	}

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		WorkDir:     mainDir, // should NOT be used when worktree is active
	}
	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	rs := makeWriteScenarioTestsRunState(t)
	rs.WorktreePath = worktreeDir

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The writer should have received worktreeDir, not mainDir
	for i, wd := range writer.receivedWorkDirs {
		if wd != worktreeDir {
			t.Fatalf("writer call %d received workDir=%q, want %q (worktree path)", i, wd, worktreeDir)
		}
	}
}

// Verify WriteScenarioTestsStage satisfies the Stage interface
var _ specloop.Stage = (*WriteScenarioTestsStage)(nil)
