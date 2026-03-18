package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// WriteScenarioTestsStageConfig configures the WriteScenarioTestsStage.
type WriteScenarioTestsStageConfig struct {
	// SpecPath is the path to the raw spec markdown file.
	SpecPath string
	// EvidenceDir is the directory where scenario-test-manifest.json will be written.
	EvidenceDir string
	// Store provides access to run storage operations.
	Store *runstore.Store
	// WorkDir is the working directory for the project (used for writing test files).
	WorkDir string
	// CompileDir, when non-empty, is the directory used for `go test -c` compilation
	// checks instead of WorkDir. This is useful when new functions exist only in a
	// worktree (CompileDir) and not yet in the main repo (WorkDir).
	CompileDir string
}

// WriteScenarioTestsStage writes test files for each scenario parsed from the spec.
// It is a no-op (idempotent) if ScenarioTestsWritten is already true on the RunState.
type WriteScenarioTestsStage struct {
	writer   contract.ScenarioTestWriter
	cfg      WriteScenarioTestsStageConfig
	budget   *specloop.Budget
	eventLog *runstore.EventLog
}

// NewWriteScenarioTestsStage creates a new WriteScenarioTestsStage.
func NewWriteScenarioTestsStage(writer contract.ScenarioTestWriter, cfg WriteScenarioTestsStageConfig, budget *specloop.Budget, eventLog *runstore.EventLog) *WriteScenarioTestsStage {
	return &WriteScenarioTestsStage{
		writer:   writer,
		cfg:      cfg,
		budget:   budget,
		eventLog: eventLog,
	}
}

// Name returns the stage name.
func (s *WriteScenarioTestsStage) Name() string { return "write_scenario_tests" }

// Run executes the write-scenario-tests stage.
func (s *WriteScenarioTestsStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	// Early guard: EvidenceDir is required to write the manifest file.
	if s.cfg.EvidenceDir == "" {
		return specloop.NextAction{}, fmt.Errorf("write_scenario_tests: EvidenceDir is required but empty")
	}

	// Idempotency: if scenario tests are already written, skip.
	if rs.ScenarioTestsWritten {
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}

	// Read raw spec markdown to parse scenarios.
	specBytes, err := os.ReadFile(s.cfg.SpecPath)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("read spec file: %w", err)
	}

	scenarios, skipped, err := contract.ParseScenarios(string(specBytes))
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("parse scenarios: %w", err)
	}
	for _, reason := range skipped {
		if s.eventLog != nil {
			s.eventLog.Append(runstore.ContractScenarioSkippedEvent{
				BaseEvent: runstore.BaseEvent{Type: "contract_scenario_skipped", Timestamp: time.Now()},
				Reason:    reason,
			})
		}
	}

	// No scenarios — no-op.
	if len(scenarios) == 0 {
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}

	// Budget check before writing scenario tests.
	if s.budget != nil && s.budget.Exceeded() {
		reason := "budget exhausted: " + s.budget.Reason()
		if s.eventLog != nil {
			s.eventLog.Append(runstore.ScenarioTestsBlockedEvent{
				BaseEvent: runstore.BaseEvent{Type: "scenario_tests_blocked", Timestamp: time.Now()},
				Reason:    reason,
			})
		}
		return specloop.NextAction{
			Kind: specloop.Blocked,
			Context: &specloop.FailureContext{
				Failures: []string{reason},
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	// Collect implementation files from done tasks (deduplicated union).
	implFiles := collectImplementationFiles(rs)

	// Load existing manifest for partial recovery.
	manifestPath := filepath.Join(s.cfg.EvidenceDir, "scenario-test-manifest.json")
	manifest := loadManifest(manifestPath)

	// Track which scenarios have already been successfully written.
	alreadyWritten := make(map[string]bool)
	for _, entry := range manifest.Scenarios {
		alreadyWritten[entry.Name] = true
	}

	// Iterate scenarios one at a time.
	var blockedReason string
	for _, scenario := range scenarios {
		// Check budget before each scenario.
		if s.budget != nil && s.budget.Exceeded() {
			blockedReason = "budget exhausted during scenario test writing: " + s.budget.Reason()
			break
		}

		// Skip if already successfully written and compiled.
		if alreadyWritten[scenario.Name] {
			testFile := findTestFileInManifest(manifest, scenario.Name)
			if testFile != "" && s.compilesSuccessfully(testFile) {
				continue
			}
			// Stale non-compiling test file: delete it and remove from manifest.
			if testFile != "" {
				absPath := testFile
				if !filepath.IsAbs(absPath) {
					absPath = filepath.Join(s.cfg.WorkDir, absPath)
				}
				os.Remove(absPath)
				manifest.Scenarios = removeManifestEntry(manifest.Scenarios, scenario.Name)
				_ = saveManifest(manifestPath, manifest)
			}
		}

		// Write scenario test with up to 2 self-repair retries.
		const maxRetries = 2
		var testFilePath string
		var writeErr error
		compileErrors := ""

		for attempt := 0; attempt <= maxRetries; attempt++ {
			testFilePath, writeErr = s.writer.WriteScenarioTest(ctx, scenario, implFiles, s.cfg.WorkDir, compileErrors)
			if writeErr != nil {
				errMsg := writeErr.Error()
				// Parse errors are retryable — treat like compile errors so the LLM
				// gets feedback about the format and can correct itself.
				if strings.Contains(errMsg, "parse scenario test response:") && attempt < maxRetries {
					compileErrors = "Prior format error (fix your output format):\n" + errMsg + "\n\n" + compileErrors
					continue
				}
				blockedReason = fmt.Sprintf("scenario test writer error for %q: %v", scenario.Name, writeErr)
				break
			}

			if testFilePath == "" {
				// nil path with nil error means deliberate no-op
				break
			}

			// Verify compilation.
			if s.compilesSuccessfully(testFilePath) {
				// Success — update manifest and emit event.
				manifest.Scenarios = append(manifest.Scenarios, contract.ScenarioTestEntry{
					Name:     scenario.Name,
					TestFile: testFilePath,
				})
				if err := saveManifest(manifestPath, manifest); err != nil {
					return specloop.NextAction{}, fmt.Errorf("save manifest: %w", err)
				}
				if s.eventLog != nil {
					s.eventLog.Append(runstore.ScenarioTestWrittenEvent{
						BaseEvent:    runstore.BaseEvent{Type: "scenario_tests_written", Timestamp: time.Now()},
						ScenarioName: scenario.Name,
						TestFile:     testFilePath,
					})
				}
				writeErr = nil
				break
			}

			// Compilation failed — collect error and retry if not last attempt.
			compileErr := s.getCompileError(testFilePath)
			if attempt < maxRetries {
				compileErrors = "Prior compilation error:\n" + compileErr + "\n\n" + compileErrors
			} else {
				blockedReason = fmt.Sprintf("scenario test %q failed compilation after %d retries: %s", scenario.Name, maxRetries, compileErr)
				writeErr = fmt.Errorf("compilation failed: %s", compileErr)
			}
		}

		// Check if this scenario failed all retries.
		if writeErr != nil || blockedReason != "" {
			if blockedReason == "" {
				blockedReason = fmt.Sprintf("scenario test writer error for %q: %v", scenario.Name, writeErr)
			}
			if s.eventLog != nil {
				s.eventLog.Append(runstore.ScenarioTestsBlockedEvent{
					BaseEvent: runstore.BaseEvent{Type: "scenario_tests_blocked", Timestamp: time.Now()},
					Reason:    blockedReason,
				})
			}
			return specloop.NextAction{
				Kind: specloop.Blocked,
				Context: &specloop.FailureContext{
					Failures: []string{blockedReason},
					Cycle:    rs.Cycle,
				},
			}, nil
		}
	}

	// Check if we hit a budget limit.
	if blockedReason != "" {
		if s.eventLog != nil {
			s.eventLog.Append(runstore.ScenarioTestsBlockedEvent{
				BaseEvent: runstore.BaseEvent{Type: "scenario_tests_blocked", Timestamp: time.Now()},
				Reason:    blockedReason,
			})
		}
		return specloop.NextAction{
			Kind: specloop.Blocked,
			Context: &specloop.FailureContext{
				Failures: []string{blockedReason},
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	// Set flag and emit success event.
	rs.ScenarioTestsWritten = true

	if s.eventLog != nil {
		s.eventLog.Append(runstore.ScenarioTestsCompleteEvent{
			BaseEvent:     runstore.BaseEvent{Type: "scenario_tests_complete", Timestamp: time.Now()},
			ScenarioCount: len(scenarios),
		})
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}

// collectImplementationFiles collects the deduplicated union of FilesChanged from all
// tasks with Status=='done'.
func collectImplementationFiles(rs *runstore.RunState) []string {
	seen := make(map[string]bool)
	var files []string
	for _, task := range rs.Tasks {
		if task.Status == "done" {
			for _, f := range task.FilesChanged {
				if !seen[f] {
					seen[f] = true
					files = append(files, f)
				}
			}
		}
	}
	return files
}

// loadManifest loads the scenario-test-manifest.json file, returning an empty manifest if not found.
func loadManifest(path string) *contract.ScenarioTestManifest {
	data, err := os.ReadFile(path)
	if err != nil {
		return &contract.ScenarioTestManifest{Scenarios: []contract.ScenarioTestEntry{}}
	}
	var manifest contract.ScenarioTestManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return &contract.ScenarioTestManifest{Scenarios: []contract.ScenarioTestEntry{}}
	}
	if manifest.Scenarios == nil {
		manifest.Scenarios = []contract.ScenarioTestEntry{}
	}
	return &manifest
}

// saveManifest saves the manifest to the scenario-test-manifest.json file.
func saveManifest(path string, manifest *contract.ScenarioTestManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create evidence dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest file: %w", err)
	}
	return nil
}

// findTestFileInManifest finds the test file path for a scenario in the manifest.
func findTestFileInManifest(manifest *contract.ScenarioTestManifest, scenarioName string) string {
	for _, entry := range manifest.Scenarios {
		if entry.Name == scenarioName {
			return entry.TestFile
		}
	}
	return ""
}

// removeManifestEntry removes all entries with the given scenario name from the manifest.
func removeManifestEntry(entries []contract.ScenarioTestEntry, name string) []contract.ScenarioTestEntry {
	filtered := make([]contract.ScenarioTestEntry, 0, len(entries))
	for _, e := range entries {
		if e.Name != name {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// compileDir returns the directory to use for `go test -c` compilation checks.
// When CompileDir is non-empty it is used; otherwise WorkDir is the fallback.
func (s *WriteScenarioTestsStage) compileDir() string {
	if s.cfg.CompileDir != "" {
		return s.cfg.CompileDir
	}
	return s.cfg.WorkDir
}

// compilesSuccessfully checks if a test file compiles by running 'go test -c -o /dev/null ./package-path'.
func (s *WriteScenarioTestsStage) compilesSuccessfully(testFilePath string) bool {
	pkgPath := s.derivePackagePath(testFilePath)
	if pkgPath == "" {
		return false
	}

	cmd := exec.Command("go", "test", "-c", "-o", "/dev/null", pkgPath)
	cmd.Dir = s.compileDir()
	err := cmd.Run()
	return err == nil
}

// getCompileError returns a string describing the compilation error for a test file.
func (s *WriteScenarioTestsStage) getCompileError(testFilePath string) string {
	pkgPath := s.derivePackagePath(testFilePath)
	if pkgPath == "" {
		return "could not derive package path from test file"
	}

	cmd := exec.Command("go", "test", "-c", "-o", "/dev/null", pkgPath)
	cmd.Dir = s.compileDir()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%v: %s", err, string(output))
	}
	return "unknown compilation error"
}

// derivePackagePath derives the Go package path suitable for use with `go test -c`
// run inside compileDir(). The test file is written under WorkDir, but if CompileDir
// is set both trees share the same sub-directory layout, so the relative path is
// identical in both. When testFilePath is a relative path it is resolved against
// WorkDir first to obtain an absolute path, then the relative offset is re-expressed
// from compileDir() (which may differ from WorkDir).
//
// Precondition: CompileDir and WorkDir must share the same subdirectory layout
// (as is the case for git worktrees). Violating this invariant causes this function
// to return "" which makes compilesSuccessfully return false.
//
// Example: testFile = "/worktree/internal/next/contract/foo_test.go",
//
//	compileDir = "/worktree"  → "./internal/next/contract"
func (s *WriteScenarioTestsStage) derivePackagePath(testFilePath string) string {
	// Ensure testFilePath is absolute, resolving against WorkDir if relative.
	if !filepath.IsAbs(testFilePath) {
		testFilePath = filepath.Join(s.cfg.WorkDir, testFilePath)
	}

	// Get the directory containing the test file.
	dir := filepath.Dir(testFilePath)

	// Compute the relative path from compileDir() to the test file's directory.
	// When CompileDir matches WorkDir (or is empty), this is identical to the old
	// behaviour. When they differ but share the same sub-directory structure the
	// resulting relative path is still correct for compilation in compileDir().
	base := s.compileDir()
	relPath, err := filepath.Rel(base, dir)
	if err != nil {
		return ""
	}

	// Return as a Go package path: "./<relPath>"
	return "./" + relPath
}
