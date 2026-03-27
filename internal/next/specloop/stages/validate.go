package stages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
	"gopkg.in/yaml.v3"
)

// FinalValidator abstracts final validation for testability.
type FinalValidator interface {
	RunFinal(ctx context.Context, alwaysRun []validator.Check, projectChecks []validator.Check, workDir string) (validator.FinalResult, error)
}

// worktreeCleanupError wraps an error from RemoveWorktree so callers can
// distinguish cleanup failures from recreation failures using errors.As.
type worktreeCleanupError struct{ err error }

func (e *worktreeCleanupError) Error() string {
	return fmt.Sprintf("worktree cleanup failed: %v", e.err)
}
func (e *worktreeCleanupError) Unwrap() error { return e.err }

// ValidateStageConfig configures the ValidateStage.
type ValidateStageConfig struct {
	AlwaysRun        []validator.Check
	AutoFix          []validator.Check // commands to run before validation (e.g. gofmt -w .)
	ProjectChecks    []validator.Check
	WorkDir          string
	EvidenceDir      string
	RepoDir          string
	SearchExtensions []string // file extensions to search for contract correction (e.g. [".go"])
	SpecText         string
}

// ValidateStage runs final validation checks.
type ValidateStage struct {
	validator         FinalValidator
	contractEvaluator contract.ContractEvaluator
	cfg               ValidateStageConfig
	eventLog          *runstore.EventLog
	gitOps            GitOps
}

// NewValidateStage creates a new ValidateStage. An optional ContractEvaluator may be
// provided; if nil, contract checking is skipped.
func NewValidateStage(v FinalValidator, cfg ValidateStageConfig, eventLog *runstore.EventLog, evaluator contract.ContractEvaluator, gitOps GitOps) *ValidateStage {
	return &ValidateStage{validator: v, cfg: cfg, eventLog: eventLog, contractEvaluator: evaluator, gitOps: gitOps}
}

// Name returns the stage name.
func (s *ValidateStage) Name() string { return "validate" }

// Run executes final validation.
func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	workDir := s.cfg.WorkDir
	if rs.WorktreePath != "" {
		workDir = rs.WorktreePath
	}

	// Health check: if using a worktree, verify it's in a healthy state.
	if rs.WorktreePath != "" {
		healthErr := s.checkWorktreeHealth(workDir)
		if healthErr != nil {
			// If GitOps is configured, attempt recovery
			if s.gitOps != nil {
				recoveryErr := s.recoverWorktree(ctx, rs, healthErr)
				if recoveryErr != nil {
					// Recovery failed; distinguish cleanup failure from recreation failure.
					var blockerMsg string
					var cleanupErr *worktreeCleanupError
					if errors.As(recoveryErr, &cleanupErr) {
						blockerMsg = fmt.Sprintf("infrastructure: %v", recoveryErr)
					} else {
						blockerMsg = fmt.Sprintf("infrastructure: worktree recovery failed: %v, recovery error: %v", healthErr, recoveryErr)
					}
					rs.BlockerSummary = blockerMsg
					return specloop.NextAction{Kind: specloop.Blocked}, nil
				}
				// Recovery succeeded; update workDir to the new worktree path
				workDir = rs.WorktreePath
			} else {
				// GitOps is nil; block with infrastructure diagnostic
				rs.BlockerSummary = fmt.Sprintf("infrastructure: worktree health check failed: %v", healthErr)
				return specloop.NextAction{Kind: specloop.Blocked}, nil
			}
		}
	}

	// Collect contract failures first (if EvidenceDir is configured and file exists).
	var failures []string
	if s.cfg.EvidenceDir != "" && s.contractEvaluator != nil {
		contractPath := filepath.Join(s.cfg.EvidenceDir, "scenario-contracts.yaml")
		data, err := os.ReadFile(contractPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return specloop.NextAction{}, fmt.Errorf("read scenario-contracts.yaml: %w", err)
		}
		if err == nil {
			sc, err := contract.ParseContractYAML(string(data))
			if err != nil {
				return specloop.NextAction{}, fmt.Errorf("parse scenario-contracts.yaml: %w", err)
			}
			contractFailures, err := s.contractEvaluator.Evaluate(ctx, &sc, workDir)
			if err != nil {
				return specloop.NextAction{}, fmt.Errorf("evaluate contracts: %w", err)
			}

			// (1) Deferral pass on raw failures
			deferResult := deferContractFailures(contractFailures, rs.Tasks)
			contractFailures = deferResult.remaining
			emitDeferralEvents(s.eventLog, deferResult.deferred, deferResult.taskIDByFile)

			// (2) Self-correction pass on raw failures
			corrected, _ := s.attemptContractCorrection(&sc, contractFailures, workDir, contractPath)

			// If corrections were made, emit events and re-evaluate
			if len(corrected) > 0 {
				// Emit correction events
				for _, c := range corrected {
					if s.eventLog != nil {
						s.eventLog.Append(runstore.ContractCorrectedEvent{
							BaseEvent:    runstore.BaseEvent{Type: "contract_corrected", Timestamp: time.Now()},
							ScenarioName: c.ScenarioName,
							OldPath:      c.OldPath,
							NewPath:      c.NewPath,
							Pattern:      c.Pattern,
						})
					}
				}

				// Always re-evaluate after corrections, regardless of uncorrectable failures
				reEvaluated, err := s.contractEvaluator.Evaluate(ctx, &sc, workDir)
				if err != nil {
					return specloop.NextAction{}, fmt.Errorf("re-evaluate contracts after correction: %w", err)
				}
				// Re-apply deferral after re-evaluation
				reResult := deferContractFailures(reEvaluated, rs.Tasks)
				contractFailures = reResult.remaining
				emitDeferralEvents(s.eventLog, reResult.deferred, reResult.taskIDByFile)
			}

			// (3) Format to []string AFTER both passes (deferral + self-correction) complete
			contractFailureStrings := []string{}
			for _, f := range contractFailures {
				failureStr := fmt.Sprintf("contract:%s — %s failed: %s", f.ScenarioName, f.AssertionType, f.Details)
				failures = append(failures, failureStr)
				contractFailureStrings = append(contractFailureStrings, failureStr)
			}
			// (4) Contract loop detection (spec 0003g):
			// When the same non-empty set of contract failure strings repeats on
			// two consecutive cycles, escalate to needs_human instead of replanning.
			if len(contractFailureStrings) > 0 && slicesEqual(contractFailureStrings, rs.LastContractFailures) {
				return specloop.NextAction{
					Kind: specloop.NeedsHuman,
					Context: &specloop.FailureContext{
						Failures: append([]string{"repeated contract failures — same failures on consecutive cycles:"}, contractFailureStrings...),
						Cycle:    rs.Cycle,
					},
				}, nil
			}
			// Store non-deferred contract failures for next-cycle comparison
			rs.LastContractFailures = contractFailureStrings
		}
	}

	// Safety net: auto-format all Go files before validation so that
	// TestRepoGofmtCompliance (and the "gofmt -l ." always-run check) won't
	// fail on files created by earlier stages. Best-effort — errors are logged
	// but never block validation.
	if fmtCmd := exec.CommandContext(ctx, "gofmt", "-w", "."); fmtCmd != nil {
		fmtCmd.Dir = workDir
		if out, err := fmtCmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "gromit: gofmt -w . safety net: %v: %s\n", err, out)
		}
	}

	// Run auto-fix commands before validation (e.g. gofmt -w .).
	// Failures are logged but do not block validation.
	if len(s.cfg.AutoFix) > 0 {
		runner := validator.NewRunner()
		for _, fix := range s.cfg.AutoFix {
			runner.RunCheck(ctx, fix, workDir) //nolint:errcheck // auto-fix failures are best-effort
		}
	}

	// Run shell checks regardless of contract results.
	// Scenario test failures are detected via the always-run 'go test ./...' check
	// and reported through the standard go test output format.
	result, err := s.validator.RunFinal(ctx, s.cfg.AlwaysRun, s.cfg.ProjectChecks, workDir)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("final validation: %w", err)
	}

	// Collect shell check failures.
	baselineFailures := rs.BaselineFailures
	if baselineFailures == nil {
		baselineFailures = map[string]string{}
		rs.BaselineFailures = baselineFailures
	}
	baselineExcluded := false
	for _, cr := range result.AlwaysRun.FailedChecks() {
		if output, ok := baselineFailures[cr.Name]; ok {
			baselineExcluded = true
			s.emitBaselineFailureExcludedEvent(rs, cr.Name, output, cr.Output)
			continue
		}
		failures = append(failures, fmt.Sprintf("always-run check %q failed: %s", cr.Name, filterPassingTestLines(cr.Output)))
	}
	for _, cr := range result.ProjectChecks.FailedChecks() {
		failures = append(failures, fmt.Sprintf("project check %q failed: %s", cr.Name, filterPassingTestLines(cr.Output)))
	}

	// Detect I/O leak infrastructure failures before classifying as test logic failures.
	// "Test I/O incomplete" / "WaitDelay expired" means a test subprocess leaked I/O
	// pipes — this is a lifecycle bug, not a test logic failure. Replanning won't help.
	if msg := detectIOLeakFailure(result); msg != "" {
		rs.BlockerSummary = msg
		rs.LastFinalValidation = &result
		return specloop.NextAction{Kind: specloop.Blocked}, nil
	}

	nonBaselineFailures := len(failures)

	// Determine final validation status after collecting ALL failures (contract + shell).
	finalPassed := result.Pass && nonBaselineFailures == 0

	// Patch the result to reflect the actual final status (contract failures may
	// have turned a shell-passing run into an overall failure).
	result.Pass = finalPassed
	rs.LastFinalValidation = &result

	// Store validation result summary reflecting actual final status.
	validationSummary := fmt.Sprintf("pass=%v", finalPassed)
	rs.LastValidationResult = &validationSummary

	// Emit final_validation_result event after all failures are collected.
	if s.eventLog != nil {
		s.eventLog.Append(runstore.FinalValidationResultEvent{
			BaseEvent: runstore.BaseEvent{Type: "final_validation_result", Timestamp: time.Now()},
			Passed:    finalPassed,
		})
	}

	rs.FinalValidationPassed = finalPassed

	if finalPassed {
		// Commit worktree changes to branch so they survive recovery
		// and are visible to the review stage's Claude process.
		if s.gitOps != nil && rs.WorktreePath != "" {
			msg := fmt.Sprintf("gromit: %s cycle %d", rs.SpecID, rs.Cycle)
			if err := s.gitOps.CommitAll(workDir, msg); err != nil {
				fmt.Fprintf(os.Stderr, "gromit: CommitAll warning: %v\n", err)
			}
		}
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}

	if nonBaselineFailures == 0 && baselineExcluded {
		rs.FinalValidationPassed = true
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}

	if len(failures) == 0 {
		failures = []string{"validation failed"}
	}

	return specloop.NextAction{
		Kind: specloop.ReplanFrom, // validation failure replan action
		Context: &specloop.FailureContext{
			Failures: failures,
			Cycle:    rs.Cycle,
		},
	}, nil
}

func (s *ValidateStage) emitBaselineFailureExcludedEvent(rs *runstore.RunState, checkName, baselineOutput, currentOutput string) {
	if s.eventLog == nil {
		return
	}
	s.eventLog.Append(runstore.BaselineFailureExcludedEvent{
		BaseEvent:     runstore.BaseEvent{Type: "baseline_failure_excluded", Timestamp: time.Now()},
		RunID:         rs.RunID,
		SpecID:        rs.SpecID,
		ProjectID:     rs.ProjectID,
		Cycle:         rs.Cycle,
		CheckName:     checkName,
		Output:        baselineOutput,
		CurrentOutput: currentOutput,
	})
}

// correctionResult tracks a single corrected assertion for event emission.
type correctionResult struct {
	ScenarioName string
	OldPath      string
	NewPath      string
	Pattern      string
}

// attemptContractCorrection tries to fix file_contains failures by finding patterns
// in sibling files. Returns corrected assertions and uncorrectable failures.
func (s *ValidateStage) attemptContractCorrection(
	sc *contract.ScenarioContract,
	failures []contract.ContractFailure,
	workDir, contractPath string,
) ([]correctionResult, []contract.ContractFailure) {
	var corrected []correctionResult
	var remaining []contract.ContractFailure

	// Group failures by scenario for easier processing
	failuresByScenario := make(map[string][]contract.ContractFailure)
	for _, f := range failures {
		if f.AssertionType == "file_contains" {
			failuresByScenario[f.ScenarioName] = append(failuresByScenario[f.ScenarioName], f)
		} else {
			remaining = append(remaining, f)
		}
	}

	// Try to correct each scenario's file_contains failures
	for scenarioIdx, scenario := range sc.Scenarios {
		if scenarioFailures, ok := failuresByScenario[scenario.Name]; ok {
			for _, failure := range scenarioFailures {
				// Try to extract path and pattern from failure assertion or details
				oldPath, pat := extractFileAndPattern(failure)
				if oldPath == "" {
					remaining = append(remaining, failure)
					continue
				}

				// Search for the pattern in sibling files
				found := findSiblingFileWithPattern(workDir, oldPath, pat, s.cfg.SearchExtensions)
				if found == "" {
					remaining = append(remaining, failure)
					continue
				}

				// Guard clause: if the old path is mentioned in the spec's Acceptance Criteria,
				// reject the correction (don't accept a sibling-file correction for spec-mentioned paths)
				if specACMentionsPath(s.cfg.SpecText, oldPath) {
					remaining = append(remaining, failure)
					if s.eventLog != nil {
						s.eventLog.Append(runstore.ContractCorrectionRejectedEvent{
							BaseEvent:     runstore.BaseEvent{Type: "contract_correction_rejected", Timestamp: time.Now()},
							ScenarioName:  scenario.Name,
							OldPath:       oldPath,
							CandidatePath: filepath.Base(found),
							Reason:        fmt.Sprintf("oldPath %q mentioned in spec acceptance criteria", oldPath),
						})
					}
					continue
				}
				// Found a sibling file with the pattern, update the contract
				for assertionIdx, assertion := range scenario.Assertions {
					if assertion.FileContains != nil && assertion.FileContains.Path == oldPath && assertion.FileContains.Pattern == pat {
						// Update the assertion
						sc.Scenarios[scenarioIdx].Assertions[assertionIdx].FileContains.Path = found
						corrected = append(corrected, correctionResult{
							ScenarioName: scenario.Name,
							OldPath:      oldPath,
							NewPath:      found,
							Pattern:      pat,
						})
						break
					}
				}
			}
		}
	}

	// Rewrite the corrected contract to disk if any corrections were made
	if len(corrected) > 0 {
		if err := rewriteContractYAML(contractPath, sc); err != nil {
			// Log error but continue (fire-and-forget for contract rewriting)
			fmt.Fprintf(os.Stderr, "gromit: rewrite contract YAML: %v\n", err)
		}
	}

	return corrected, remaining
}

// specACMentionsPath extracts the Acceptance Criteria section from the spec markdown
// and checks if the basename of the given file path is mentioned in that section.
func specACMentionsPath(spec string, filePath string) bool {
	// Empty filePath should always return false
	if filePath == "" {
		return false
	}

	// Extract the Acceptance Criteria section from the spec
	acStart := strings.Index(spec, "## Acceptance Criteria")
	if acStart == -1 {
		return false
	}

	// Find where the AC section ends (next heading or EOF)
	acContent := spec[acStart:]
	nextHeading := strings.Index(acContent[len("## Acceptance Criteria"):], "\n##")

	var acSection string
	if nextHeading == -1 {
		acSection = acContent
	} else {
		acSection = acContent[:len("## Acceptance Criteria")+nextHeading]
	}

	// Check if the file's basename is mentioned in the AC section
	baseName := filepath.Base(filePath)
	return strings.Contains(acSection, baseName)
}

// extractFileAndPattern extracts the file path and pattern from a ContractFailure's
// structured Assertion field. Returns (path, pattern) or ("", "") if unavailable.
func extractFileAndPattern(f contract.ContractFailure) (string, string) {
	if f.Assertion.FileContains != nil {
		return f.Assertion.FileContains.Path, f.Assertion.FileContains.Pattern
	}
	return "", ""
}

// findSiblingFileWithPattern searches for a file in the same directory as filePath
// that contains the pattern using contract.MatchesPattern. It only considers files
// whose extension is in the provided extensions list and skips the original failing file.
func findSiblingFileWithPattern(workDir, filePath, pattern string, extensions []string) string {
	dir := filepath.Dir(filePath)
	dirPath := filepath.Join(workDir, dir)
	baseName := filepath.Base(filePath)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only search files with an allowed extension.
		ext := filepath.Ext(entry.Name())
		allowed := false
		for _, e := range extensions {
			if ext == e {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}

		// Skip the original failing file.
		if entry.Name() == baseName {
			continue
		}

		candidatePath := filepath.Join(dirPath, entry.Name())
		content, err := os.ReadFile(candidatePath)
		if err != nil {
			continue
		}

		if contract.MatchesPattern(string(content), pattern) {
			relPath, _ := filepath.Rel(workDir, candidatePath)
			return relPath
		}
	}

	return ""
}

// rewriteContractYAML writes the corrected contract back to the YAML file.
func rewriteContractYAML(contractPath string, sc *contract.ScenarioContract) error {
	data, err := yaml.Marshal(sc)
	if err != nil {
		return fmt.Errorf("marshal contract: %w", err)
	}
	return os.WriteFile(contractPath, data, 0o644)
}

// deferralResult holds the result of deferring contract failures, including
// a map from file path to the task ID that covers it.
type deferralResult struct {
	remaining    []contract.ContractFailure
	deferred     []contract.ContractFailure
	taskIDByFile map[string]string // file path → covering task ID
}

// deferContractFailures defers file_contains and file_exists failures that are
// covered by pending tasks' ExpectedTouchedArea. Uses exact string matching only.
// First task in rs.Tasks slice order wins when multiple pending tasks cover the same file.
// Only pending tasks trigger deferral; tasks with other statuses are ignored.
func deferContractFailures(failures []contract.ContractFailure, tasks []runstore.Task) deferralResult {
	// Build a map from file path to first task ID that covers it (pending tasks only)
	coverageMap := make(map[string]string)
	for _, task := range tasks {
		// Only process pending tasks
		if task.Status != "pending" {
			continue
		}
		for _, path := range task.ExpectedTouchedArea {
			// Only map if we haven't seen this path before (first task wins)
			if _, exists := coverageMap[path]; !exists {
				coverageMap[path] = task.TaskID
			}
		}
	}

	var result deferralResult
	result.taskIDByFile = coverageMap

	// Process each failure
	for _, f := range failures {
		filePath := extractFilePath(f)
		if filePath == "" || (f.AssertionType != "file_contains" && f.AssertionType != "file_exists") {
			// Not a deferrable failure type or couldn't extract path
			result.remaining = append(result.remaining, f)
			continue
		}

		// Check if the file is covered by any task
		if _, covered := coverageMap[filePath]; covered {
			result.deferred = append(result.deferred, f)
		} else {
			result.remaining = append(result.remaining, f)
		}
	}

	return result
}

// emitDeferralEvents emits a ContractDeferredEvent for each deferred failure,
// using the provided taskIDByFile map to look up covering task IDs.
func emitDeferralEvents(eventLog *runstore.EventLog, deferred []contract.ContractFailure, taskIDByFile map[string]string) {
	if eventLog == nil {
		return
	}
	for _, d := range deferred {
		filePath := extractFilePath(d)
		eventLog.Append(runstore.ContractDeferredEvent{
			BaseEvent:    runstore.BaseEvent{Type: "contract_deferred", Timestamp: time.Now()},
			ScenarioName: d.ScenarioName,
			FilePath:     filePath,
			Pattern:      extractPattern(d),
			TaskID:       taskIDByFile[filePath],
		})
	}
}

// extractFilePath extracts the file path from a ContractAssertion.
func extractFilePath(f contract.ContractFailure) string {
	switch f.AssertionType {
	case "file_exists":
		return f.Assertion.FileExists
	case "file_contains":
		if f.Assertion.FileContains != nil {
			return f.Assertion.FileContains.Path
		}
	}
	return ""
}

// extractPattern extracts the pattern from a ContractAssertion.
// Returns empty string for file_exists assertions.
func extractPattern(f contract.ContractFailure) string {
	if f.AssertionType == "file_contains" && f.Assertion.FileContains != nil {
		return f.Assertion.FileContains.Pattern
	}
	return ""
}

// checkWorktreeHealth verifies that the worktree directory is healthy.
// Returns nil if all checks pass, or an error describing what's wrong.
func (s *ValidateStage) checkWorktreeHealth(workDir string) error {
	// Check if directory exists
	if _, err := os.Stat(workDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("directory does not exist: %s", workDir)
		}
		return err
	}

	// Check if .git file exists
	gitPath := filepath.Join(workDir, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(".git file missing in %s", workDir)
		}
		return err
	}

	// Check if go.mod exists
	goModPath := filepath.Join(workDir, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("go.mod not found in %s", workDir)
		}
		return err
	}

	return nil
}

// ioLeakSignatures are substrings in go test output that indicate a test binary
// leaked subprocess I/O pipes. These are infrastructure/lifecycle bugs, not test
// logic failures — replanning or retrying the fix task won't resolve them.
var ioLeakSignatures = []string{
	"Test I/O incomplete",
	"WaitDelay expired",
}

// detectIOLeakFailure scans all failed check results for I/O leak signatures.
// Returns an infrastructure blocker message if detected, or "" if no leak found.
func detectIOLeakFailure(result validator.FinalResult) string {
	allFailed := append(result.AlwaysRun.FailedChecks(), result.ProjectChecks.FailedChecks()...)
	for _, cr := range allFailed {
		for _, sig := range ioLeakSignatures {
			if strings.Contains(cr.Output, sig) {
				return fmt.Sprintf(
					"infrastructure_io_leak: test binary has leaked subprocess I/O — likely a fire-and-forget goroutine spawning a subprocess (check %q)",
					cr.Name,
				)
			}
		}
	}
	return ""
}

// filterPassingTestLines removes passing-package and no-test-files lines from
// go test output, keeping only lines relevant to failures. This reduces noise
// for the replanner without truncating any failure details.
//
// Lines removed:
//   - "ok  \t..." — passing packages
//   - "?   \t..." — packages with no test files
//
// If filtering produces an empty string (shouldn't happen for a failed check),
// the original string is returned unchanged as a fallback.
func filterPassingTestLines(s string) string {
	lines := strings.Split(s, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(line, "ok  \t") || strings.HasPrefix(line, "?   \t") {
			continue
		}
		filtered = append(filtered, line)
	}
	result := strings.Join(filtered, "\n")
	if strings.TrimSpace(result) == "" {
		return s
	}
	return result
}

// slicesEqual returns true if a and b have the same length and identical
// elements in the same order. Used for contract loop detection (spec 0003g).
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// recoverWorktree attempts to recover a failed worktree by removing the existing
// one and creating a fresh worktree. It emits WorktreeRecoveryEvent and updates
// rs.WorktreePath on success.
func (s *ValidateStage) recoverWorktree(_ context.Context, rs *runstore.RunState, healthErr error) error {
	// Step 1: Remove the old worktree
	if err := s.gitOps.RemoveWorktree(rs.WorktreePath); err != nil {
		if s.eventLog != nil {
			s.eventLog.Append(runstore.WorktreeRecoveryEvent{
				BaseEvent:          runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
				HealthCheckFailure: healthErr.Error(),
				RecoverySucceeded:  false,
			})
		}
		return &worktreeCleanupError{err: err}
	}

	// Step 2: Derive the branch name
	branch := fmt.Sprintf("gromit/spec-%s-%s", rs.SpecID, rs.RunID)

	// Step 3: Recover (recreate) the worktree
	newWorktreePath, err := s.gitOps.RecoverWorktree(s.cfg.RepoDir, branch)
	if err != nil {
		if s.eventLog != nil {
			s.eventLog.Append(runstore.WorktreeRecoveryEvent{
				BaseEvent:          runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
				HealthCheckFailure: healthErr.Error(),
				RecoverySucceeded:  false,
			})
		}
		return err
	}

	// Step 4: On success, update rs.WorktreePath and emit success event
	rs.WorktreePath = newWorktreePath
	if s.eventLog != nil {
		s.eventLog.Append(runstore.WorktreeRecoveryEvent{
			BaseEvent:          runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
			HealthCheckFailure: healthErr.Error(),
			RecoverySucceeded:  true,
			NewWorktreePath:    newWorktreePath,
		})
	}
	return nil
}
