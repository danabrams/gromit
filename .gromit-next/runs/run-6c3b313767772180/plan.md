# Plan (Cycle 1)

## t-001

Add ScenarioTestsWritten and FailureHistory fields to RunState in internal/next/runstore/types.go. ScenarioTestsWritten is a bool, FailureHistory is map[string]int with json omitempty. Ensure NormalizeNilFields initializes FailureHistory to empty map when nil. Do NOT add these fields to the per-cycle reset block in ResetForNewCycle in store.go — add a comment explaining they persist across cycles.

## t-002

Add event types for scenario test writing to internal/next/runstore/events.go: ScenarioTestWrittenEvent (per-scenario, with ScenarioName and TestFile fields), ScenarioTestsCompleteEvent (with ScenarioCount field), and ScenarioTestsBlockedEvent (with Reason field). Follow the existing ContractsWrittenEvent/ContractsBlockedEvent pattern. Event type strings: 'scenario_tests_written', 'scenario_tests_complete', 'scenario_tests_blocked'.

## t-003

Define the ScenarioTestWriter interface in internal/next/contract/scenario_writer.go. Interface has a single method: WriteScenarioTest(ctx context.Context, scenario SpecScenario, implFiles []string, workDir string, compileErrors string) (testFilePath string, err error). Also define ScenarioTestManifest and ScenarioTestEntry types for the manifest JSON in the same file.

## t-004

Implement WriteScenarioTestsStage in internal/next/specloop/stages/write_scenario_tests.go. Follow the WriteContractsStage pattern. Config struct: WriteScenarioTestsStageConfig with EvidenceDir, WorkDir, Store. Stage struct holds: writer (contract.ScenarioTestWriter), cfg, budget (*specloop.Budget), eventLog (*runstore.EventLog). Constructor: NewWriteScenarioTestsStage. Name() returns 'write_scenario_tests'. Run() implements: (1) idempotency check via rs.ScenarioTestsWritten, (2) empty scenarios check returns Continue, (3) load partial manifest if exists for retry skip logic, (4) iterate scenarios one at a time, (5) for each: check budget, skip if already in manifest and compiles, call writer.WriteScenarioTest, check compilation via 'go test -c -o /dev/null ./<pkg>', up to 2 self-repair retries with compile errors, update manifest incrementally after each success, emit scenario_tests_written event, (6) on all success: set rs.ScenarioTestsWritten=true, emit scenario_tests_complete, return Continue, (7) on failure: emit scenario_tests_blocked, return Blocked. Compilation check uses exec.CommandContext. Manifest written to cfg.EvidenceDir/scenario-test-manifest.json.

## t-005

Implement helper function to collect implementation files from RunState tasks: collectImplFiles(rs *runstore.RunState) []string. Returns deduplicated union of FilesChanged from all tasks with Status == 'done'. Place this as a private helper in write_scenario_tests.go. Also implement helper to extract package path from a test file path for the 'go test -c' command.

## t-006

Write unit tests for WriteScenarioTestsStage in internal/next/specloop/stages/write_scenario_tests_test.go. Use mocked ScenarioTestWriter interface. Test cases: (1) idempotency — returns Continue when ScenarioTestsWritten is true, (2) empty scenarios — returns Continue with no scenarios, (3) happy path — 2 scenarios both succeed, manifest written, flag set, events emitted, (4) compile failure with self-repair success on retry, (5) compile failure exhausts retries — returns Blocked, flag NOT set, earlier scenario tests preserved, (6) budget exhausted mid-stage — returns Blocked, (7) partial manifest retry — skips already-compiled scenarios. Follow the Seed/Invoke/Assert pattern from docs/scenario-tests.md.

## t-007

Implement FailureHistory helper functions as exported functions in internal/next/specloop/failure_history.go: (1) ExtractTestFailureKeys(output string) []string — extracts test function names from '--- FAIL: TestName' lines in go test output, (2) ExtractContractFailureKeys(failures []string) []string — extracts 'contract:<name>' prefix by splitting on ' — ' and taking first segment, (3) UpdateFailureHistory(history map[string]int, currentKeys []string) — increments counts for current keys, resets absent keys to zero, (4) PersistentFailureHints(history map[string]int, threshold int) []string — returns diagnostic hint strings for keys at or above threshold.

## t-008

Write unit tests for FailureHistory helpers in internal/next/specloop/failure_history_test.go. Test cases: (1) ExtractTestFailureKeys parses standard go test output with '--- FAIL: TestFoo (0.01s)' lines, (2) ExtractTestFailureKeys returns empty for passing output, (3) ExtractContractFailureKeys splits 'contract:add-works — file_contains failed: ...' correctly, (4) UpdateFailureHistory increments existing keys and resets absent keys, (5) PersistentFailureHints returns hints only for keys >= threshold, (6) PersistentFailureHints returns empty when all below threshold.

## t-009

Integrate FailureHistory into the specloop replan path in internal/next/specloop/specloop.go. After Validate returns ReplanFrom (in the replan handling block), extract failure keys from the failure context using ExtractTestFailureKeys and ExtractContractFailureKeys. Call UpdateFailureHistory on rs.FailureHistory (initializing the map if nil). Append PersistentFailureHints (threshold=2) to the replan context / failure context before passing to the next cycle's planner.

## t-010

Wire WriteScenarioTestsStage into BuildStages in cmd/gromit-next/stage_provider.go. Add noopScenarioTestWriter following the noopContractWriter pattern. Create the real LLM adapter for ScenarioTestWriter (scenarioTestWriterAdapter in cmd/gromit-next/stage_provider.go or a dedicated adapter file) using P1/Sonnet model tier. Wire docs/scenario-tests.md content at adapter construction time (system prompt, not per-call). Insert writeScenarioTestsStage between executeStage and validateStage in the stages slice. Pass scenarios parsed from the spec (reuse the SpecScenario parsing from write_contracts wiring).

## t-011

Update ResetForNewCycle comment in internal/next/runstore/store.go to mention ScenarioTestsWritten and FailureHistory alongside ContractsWritten as fields that persist across cycles. Update the corresponding comment in specloop.go where ResetForNewCycle is called.

## t-012

Write integration tests in internal/next/specloop/stages/write_scenario_tests_integration_test.go. Test the WriteScenarioTests stage integrated with real file I/O (manifest reading/writing, evidence directory). Test cases: (1) full happy path with mock writer producing real files, manifest verification, (2) partial failure + retry correctly skips completed scenarios, (3) idempotency across simulated replan cycles. Follow the write_contracts_integration_test.go pattern.

## t-013

Write specloop integration tests in internal/next/specloop/specloop_failure_history_test.go for the FailureHistory integration in the replan path. Test cases: (1) FailureHistory is populated after first replan with test failure keys, (2) persistent failure hint appears after 2 consecutive cycles with same failure, (3) resolved failures are reset to zero, (4) mixed contract and test failures tracked correctly. Use the existing specloop integration test patterns.

## t-014

Run the full test suite to verify all existing tests pass and all new tests pass. Fix any compilation errors or test failures. Run go vet on the entire project.

