//go:build acceptance

package runner

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// TestFacadeRunValidationWithRecoveryDelegatesToValidationRunner verifies that
// the facade's runValidationWithRecovery delegates recovery to the
// validation.Runner's RunWithRecovery method rather than doing inline recovery.
//
// After extraction, the facade should call validationRunner.RunWithRecovery(ctx, bc)
// and the validation.Runner handles the recovery loop (auto-fix, Claude-based fix,
// retry capping). The facade should NOT contain its own recovery for-loop or call
// its own autoFixFn during recovery.
//
// Expected failure: the facade's runValidationWithRecovery currently calls its own
// r.autoFixFn and r.renderer.RenderBuild during inline recovery. After extraction,
// only the validation.Runner's autoFixFn (injected at construction) should run.
func TestFacadeRunValidationWithRecoveryDelegatesToValidationRunner(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 1,
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:        300,
			AnalysisTimeout:    60,
			StallTimeout:       30,
			StallTimeoutActive: 10,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Track which autoFixFn gets called: the facade's or the validation.Runner's.
	// After extraction, only the validation.Runner's autoFixFn should be called
	// during recovery, not the facade's.
	facadeAutoFixCalled := false
	valRunnerAutoFixCalled := false

	valRunnerCmdCalls := 0
	valRunner := validation.NewRunner(cfg,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			valRunnerCmdCalls++
			if valRunnerCmdCalls == 1 {
				return "", "--- FAIL: TestFoo (0.01s)\nFAIL\tpkg/foo", 1, nil
			}
			return "ok", "", 0, nil
		},
		func(startCommit string) error {
			valRunnerAutoFixCalled = true
			return nil
		},
		nil,
	)

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}
	mockAnalyzer := &mockFailureAnalyzer{}

	r := &Runner{
		cfg:              cfg,
		output:           &buf,
		analyzer:         mockAnalyzer,
		renderer:         &mockRenderer{},
		validationRunner: valRunner,
		autoFixFn: func(startCommit string) error {
			facadeAutoFixCalled = true
			return nil
		},
		escalationHandler: escalation.NewHandler(cfg, mockAnalyzer, nil, nil, nil, logFn),
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			// Facade's cmdRunner: first call fails, second passes
			return "", "--- FAIL: TestFoo (0.01s)", 1, nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-1", Title: "Test bead", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: t.TempDir()},
		StartCommit: "abc123",
	}

	_ = r.runValidationWithRecovery(context.Background(), bc)

	// Expected failure: currently the facade calls its own r.autoFixFn during
	// inline recovery. After extraction, the facade should delegate to
	// validationRunner.RunWithRecovery which uses the validation.Runner's
	// own autoFixFn (injected at construction).
	if facadeAutoFixCalled {
		t.Error("facade's autoFixFn was called during recovery — " +
			"after extraction, the facade should delegate recovery entirely " +
			"to validation.Runner.RunWithRecovery, which uses its own autoFixFn")
	}

	// After extraction, the validation.Runner's autoFixFn should be the one
	// called during recovery (not the facade's).
	if !valRunnerAutoFixCalled {
		t.Error("validation.Runner's autoFixFn was NOT called during recovery — " +
			"after extraction, recovery should use the validation.Runner's autoFixFn")
	}
}

// TestFacadeRunValidationFullyDelegatesToValidationRunner verifies that after
// extraction, the facade's runValidation delegates the full validation flow
// to the validation.Runner, not just RunDirect for command execution.
//
// Expected failure: the facade's runValidation currently calls
// validationRunner.RunDirect for command execution but handles failure summary
// extraction and result field updates inline. After extraction, these should be
// handled by validation.Runner's internal runValidation method.
func TestFacadeRunValidationFullyDelegatesToValidationRunner(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:     300,
			AnalysisTimeout: 60,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create a validation.Runner whose cmdRunner always fails with recognizable output
	valRunner := validation.NewRunner(cfg,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestBar (0.01s)\nFAIL\tpkg/bar", 1, nil
		},
		nil, nil,
	)

	var buf strings.Builder
	r := &Runner{
		cfg:              cfg,
		output:           &buf,
		analyzer:         &mockFailureAnalyzer{},
		validationRunner: valRunner,
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-2", Title: "Test bead 2", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	_ = r.runValidation(context.Background(), bc)

	// After full delegation, the validation.Runner accumulates failure summaries
	// internally. The facade should read from validationRunner.Failures() rather
	// than calling validation.ExtractValidationSummary inline.
	valFailures := valRunner.Failures()
	if len(valFailures) == 0 {
		t.Error("validation.Runner should accumulate failure summaries via Failures()")
	}

	// Expected failure: currently the facade extracts summaries inline via
	// validation.ExtractValidationSummary and appends to validationFailures.
	// The validation.Runner also accumulates via its own runValidation. After
	// full delegation, the facade should read FROM the validation.Runner's
	// Failures() rather than doing its own extraction.
	if len(r.validationFailures) == 0 {
		t.Error("facade should propagate validation.Runner failures to validationFailures")
	}

	// The facade's failures should be identical to the validation.Runner's failures
	// (read from it, not extracted separately).
	if len(valFailures) > 0 && len(r.validationFailures) > 0 {
		if r.validationFailures[0] != valFailures[0] {
			t.Errorf("facade validationFailures[0] = %q, validation.Runner.Failures()[0] = %q — "+
				"after extraction these should be identical (facade reads from Runner)",
				r.validationFailures[0], valFailures[0])
		}
	}
}

// TestProductionNewRunnerShouldWireValidationRunner verifies that the production
// NewRunner constructor wires a validationRunner, not just NewRunnerWithDeps.
//
// Since the production NewRunner requires external binaries, we test indirectly:
// construct a Runner the way NewRunner does (without validationRunner) and verify
// the fallback path is exercised. After extraction, the fallback should be removed.
//
// Expected failure: runDirectValidationCheck currently falls back to
// runDirectValidationFallback when validationRunner is nil. After extraction,
// all constructors wire validationRunner and the fallback should be removed.
func TestProductionNewRunnerShouldWireValidationRunner(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Claude: config.ClaudeConfig{
			BeadTimeout: 300,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Simulate the production NewRunner's initialization without validationRunner.
	// After extraction, this should not work — runDirectValidationCheck should
	// require a wired validationRunner rather than silently falling back.
	cmdCalled := false
	r := &Runner{
		cfg:    cfg,
		output: &strings.Builder{},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			cmdCalled = true
			return "ok", "", 0, nil
		},
	}

	result, err := r.runDirectValidationCheck(
		context.Background(),
		cfg.Validation.Commands,
		t.TempDir(),
	)

	// Expected failure: currently the fallback path works when validationRunner
	// is nil. After extraction, runDirectValidationCheck should error or panic
	// when validationRunner is nil (because the fallback is removed).
	if err == nil && result != nil && cmdCalled {
		t.Error("runDirectValidationCheck should not silently fall back when " +
			"validationRunner is nil — after extraction, all constructors wire " +
			"validationRunner and the fallback should be removed")
	}
}

// TestValidationRunnerResetFailuresCalledByRun verifies that the facade calls
// validationRunner.ResetFailures() at the start of each Run() call to prevent
// failure summaries from a previous run leaking into the next run.
//
// Expected failure: currently the facade resets its own validationFailures
// slice but does not call validationRunner.ResetFailures(). After extraction,
// the facade should reset the validation.Runner's accumulated failures too.
func TestValidationRunnerResetFailuresCalledByRun(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 0,
		},
		Claude: config.ClaudeConfig{BeadTimeout: 300},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return nil, nil // no beads, Run exits immediately
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    beads,
		Router:   newMockRouter(),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	if r.validationRunner == nil {
		t.Fatal("validationRunner should be wired by NewRunnerWithDeps")
	}

	// Accumulate a failure in the validation.Runner by running a failing validation
	failRunner := validation.NewRunner(cfg,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestOld (0.01s)\nFAIL\tpkg/old", 1, nil
		},
		nil, nil,
	)
	failBC := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "old-1", Title: "Old bead", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}
	_ = failRunner.RunWithRecovery(context.Background(), failBC)

	// Replace the wired validationRunner with one that has accumulated failures
	r.validationRunner = failRunner

	if len(r.validationRunner.Failures()) == 0 {
		t.Fatal("setup: validationRunner should have accumulated failures")
	}

	// Run with no beads — should reset validationRunner failures
	err = r.Run(context.Background(), 1, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Expected failure: currently Run() resets r.validationFailures but does
	// NOT call r.validationRunner.ResetFailures(). After extraction, it should.
	if len(r.validationRunner.Failures()) != 0 {
		t.Errorf("validationRunner.Failures() after Run() = %v, want empty — "+
			"Run() should call validationRunner.ResetFailures()",
			r.validationRunner.Failures())
	}
}

// TestFacadeRecoveryDoesNotCallRenderBuild verifies that after extraction,
// the facade's runValidationWithRecovery does NOT call r.renderer.RenderBuild
// directly during recovery. Currently the inline recovery loop (process.go ~line 892)
// calls r.renderer.RenderBuild to build the retry prompt. After extraction,
// this responsibility moves into the validation.Runner's ExecuteFn callback,
// so the facade's renderer should NOT be called during validation recovery.
//
// Expected failure: the facade's runValidationWithRecovery currently calls
// r.renderer.RenderBuild(bc.PromptCtx) inline during each recovery attempt.
// After extraction, recovery is handled entirely by validationRunner.RunWithRecovery.
func TestFacadeRecoveryDoesNotCallRenderBuild(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 1,
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:        300,
			AnalysisTimeout:    60,
			StallTimeout:       30,
			StallTimeoutActive: 10,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Track RenderBuild calls from the facade's renderer.
	// After extraction, the facade should NOT call RenderBuild during recovery.
	renderBuildCallCount := 0
	renderer := &mockPromptRenderer{
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			renderBuildCallCount++
			return "mock retry prompt", nil
		},
	}

	// Validation always fails so recovery is attempted
	valRunner := validation.NewRunner(cfg,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestAlways (0.01s)\nFAIL\tpkg/always", 1, nil
		},
		func(startCommit string) error { return nil },
		nil,
	)

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}
	mockAnalyzer := &mockFailureAnalyzer{}
	mockRouter := newMockRouter()

	r := &Runner{
		cfg:               cfg,
		output:            &buf,
		analyzer:          mockAnalyzer,
		renderer:          renderer,
		router:            mockRouter,
		invoker:           newInvokerForTest(mockRouter, &buf, nil),
		validationRunner:  valRunner,
		autoFixFn:         func(startCommit string) error { return nil },
		escalationHandler: escalation.NewHandler(cfg, mockAnalyzer, nil, nil, nil, logFn),
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestAlways (0.01s)", 1, nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-render", Title: "Test bead render", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: t.TempDir()},
		StartCommit: "abc123",
		ParentCtx:   context.Background(),
	}

	_ = r.runValidationWithRecovery(context.Background(), bc)

	// Expected failure: currently the facade calls r.renderer.RenderBuild during
	// inline recovery (process.go ~line 892). After extraction, the facade should
	// delegate recovery entirely to validationRunner.RunWithRecovery, which handles
	// Claude-based fixes via its ExecuteFn callback — the facade's renderer is not
	// used during validation recovery.
	if renderBuildCallCount > 0 {
		t.Errorf("facade renderer.RenderBuild called %d times during recovery — "+
			"after extraction, recovery should be delegated to validation.Runner.RunWithRecovery "+
			"which uses its ExecuteFn callback, not the facade's renderer",
			renderBuildCallCount)
	}
}

// TestFacadeRecoveryUsesValidationRunnerExecuteFn verifies that after extraction,
// the facade's runValidationWithRecovery delegates Claude-based fixes to the
// validation.Runner's ExecuteFn callback rather than calling
// escalationHandler.ExecuteWithRetry directly.
//
// Expected failure: the facade currently calls r.escalationHandler.ExecuteWithRetry
// directly during each recovery attempt (process.go ~line 904). After extraction,
// the facade delegates to validationRunner.RunWithRecovery which uses the
// ExecuteFn callback injected at construction time.
func TestFacadeRecoveryUsesValidationRunnerExecuteFn(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 1,
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:        300,
			AnalysisTimeout:    60,
			StallTimeout:       30,
			StallTimeoutActive: 10,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	executeFnCalled := false

	// Create a validation.Runner with an ExecuteFn that tracks its usage.
	// After extraction, THIS callback should be the one doing Claude-based fixes.
	valRunner := validation.NewRunner(cfg,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestAlways (0.01s)\nFAIL\tpkg/always", 1, nil
		},
		func(startCommit string) error { return nil },
		func(ctx context.Context, bc *runtypes.BeadContext) bool {
			executeFnCalled = true
			return true
		},
	)

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}
	mockAnalyzer := &mockFailureAnalyzer{}
	mockRouter := newMockRouter()

	r := &Runner{
		cfg:               cfg,
		output:            &buf,
		analyzer:          mockAnalyzer,
		renderer:          &mockRenderer{},
		router:            mockRouter,
		invoker:           newInvokerForTest(mockRouter, &buf, nil),
		validationRunner:  valRunner,
		autoFixFn:         func(startCommit string) error { return nil },
		escalationHandler: escalation.NewHandler(cfg, mockAnalyzer, nil, nil, nil, logFn),
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestAlways (0.01s)", 1, nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-execfn", Title: "Test ExecuteFn", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: t.TempDir()},
		StartCommit: "abc123",
		ParentCtx:   context.Background(),
	}

	_ = r.runValidationWithRecovery(context.Background(), bc)

	// Expected failure: currently the facade calls r.escalationHandler.ExecuteWithRetry
	// directly during inline recovery — the validation.Runner's ExecuteFn is NOT called.
	// After extraction, the facade delegates to validationRunner.RunWithRecovery
	// and the ExecuteFn callback handles Claude fixes.
	if !executeFnCalled {
		t.Error("validation.Runner's ExecuteFn was NOT called during recovery — " +
			"after extraction, recovery should use the validation.Runner's ExecuteFn callback " +
			"rather than the facade calling escalationHandler.ExecuteWithRetry directly")
	}
}

// TestValidationSummaryWrapperRemovedAfterExtraction verifies that after
// full extraction, the backward-compatibility wrapper extractValidationSummary
// in validation_summary.go is removed. The facade should read failure summaries
// from validationRunner.Failures() rather than calling extractValidationSummary.
//
// Expected failure: the file validation_summary.go currently exists and contains
// the extractValidationSummary wrapper function. After full extraction, this file
// should be deleted because the facade reads from validationRunner.Failures().
func TestValidationSummaryWrapperRemovedAfterExtraction(t *testing.T) {
	// After extraction, validation_summary.go should be deleted.
	// The facade should use validationRunner.Failures() instead of calling
	// the extractValidationSummary wrapper.
	//
	// We detect this by checking if the file still declares the wrapper function.
	// Using AST parsing to verify that no function named extractValidationSummary
	// exists in the runner package's non-test files.

	// Locate the runner directory relative to this test file's module root.
	// Tests run from the module root, so use a well-known marker file.
	runnerDir := "."

	// Fallback: find the directory by looking for go.mod and navigating
	goModDirs := []string{
		filepath.Join("..", "..", ".."),     // from internal/runner/
		filepath.Join("..", ".."),           // from internal/
		".",                                 // from module root
		filepath.Join("internal", "runner"), // already at module root
	}
	for _, dir := range goModDirs {
		candidate := filepath.Join(dir, "internal", "runner")
		if _, err := os.Stat(candidate); err == nil {
			runnerDir = candidate
			break
		}
		// Also check if we're already in the runner dir
		if _, err := os.Stat(filepath.Join(dir, "process.go")); err == nil {
			runnerDir = dir
			break
		}
	}

	entries, err := os.ReadDir(runnerDir)
	if err != nil {
		t.Fatalf("could not read runner directory %q: %v", runnerDir, err)
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(runnerDir, name)
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			continue
		}

		for _, decl := range f.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			// Expected failure: extractValidationSummary still exists in
			// validation_summary.go as a backward-compat wrapper. After extraction,
			// this function should be deleted — the facade reads from
			// validationRunner.Failures() instead.
			if funcDecl.Name.Name == "extractValidationSummary" && funcDecl.Recv == nil {
				t.Errorf("found extractValidationSummary function in %s — "+
					"after full extraction, this wrapper should be removed; "+
					"the facade should read from validationRunner.Failures() instead",
					name)
			}
		}
	}
}

// TestFacadeRunValidationDelegatesFailureAccumulation verifies that after
// extraction, the facade's runValidation delegates failure summary accumulation
// to the validation.Runner rather than calling validation.ExtractValidationSummary
// inline and appending to r.validationFailures independently.
//
// After extraction, the facade should call the validation.Runner's full validation
// flow (not just RunDirect for command execution), and the validation.Runner
// accumulates failures internally via its own runValidation method. The facade
// then reads from validationRunner.Failures() to populate r.validationFailures.
//
// Expected failure: the facade's runValidation currently calls
// validationRunner.RunDirect for command execution and then calls
// validation.ExtractValidationSummary inline (process.go ~line 787). The
// validation.Runner's Failures() accumulator is NOT populated because RunDirect
// does NOT accumulate failures — only the Runner's internal runValidation does.
// After extraction, the facade calls the Runner's full flow, so Failures() IS
// populated.
func TestFacadeRunValidationDelegatesFailureAccumulation(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:     300,
			AnalysisTimeout: 60,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create a validation.Runner that fails with recognizable output
	valRunner := validation.NewRunner(cfg,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestInline (0.01s)\nFAIL\tpkg/inline", 1, nil
		},
		nil, nil,
	)

	var buf strings.Builder
	r := &Runner{
		cfg:              cfg,
		output:           &buf,
		analyzer:         &mockFailureAnalyzer{},
		validationRunner: valRunner,
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-inline", Title: "Test inline extraction", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	_ = r.runValidation(context.Background(), bc)

	// Expected failure: currently the facade calls validationRunner.RunDirect
	// for command execution. RunDirect does NOT accumulate failures in the
	// validation.Runner — only the Runner's internal runValidation method does.
	// After extraction, the facade delegates the full validation flow to the
	// validation.Runner, so Failures() IS populated.
	valFailures := valRunner.Failures()
	if len(valFailures) == 0 {
		t.Error("validation.Runner.Failures() should be populated after facade's runValidation — " +
			"after extraction, the facade delegates the full validation flow " +
			"(not just RunDirect) to the validation.Runner, which accumulates failures internally")
	}
}

// TestExtractValidationSummaryTestsUsesValidationPackageDirectly verifies that
// after extraction, the tests in extract_validation_summary_test.go call
// validation.ExtractValidationSummary from the validation package directly,
// not the local extractValidationSummary backward-compat wrapper.
//
// The bead task says "Update any tests that reference extractValidationSummary
// to use the validation package directly." This test enforces that requirement
// by scanning the test file's AST for calls to the local function.
//
// Expected failure: extract_validation_summary_test.go currently calls the
// local extractValidationSummary() wrapper (e.g., line 71, 132, 155, 168, 182).
// After extraction, these calls should be replaced with
// validation.ExtractValidationSummary() and the wrapper should be deleted.
func TestExtractValidationSummaryTestsUsesValidationPackageDirectly(t *testing.T) {
	// Locate the runner directory
	runnerDir := "."
	goModDirs := []string{
		filepath.Join("..", "..", ".."),
		filepath.Join("..", ".."),
		".",
		filepath.Join("internal", "runner"),
	}
	for _, dir := range goModDirs {
		candidate := filepath.Join(dir, "internal", "runner")
		if _, err := os.Stat(candidate); err == nil {
			runnerDir = candidate
			break
		}
		if _, err := os.Stat(filepath.Join(dir, "process.go")); err == nil {
			runnerDir = dir
			break
		}
	}

	testFile := filepath.Join(runnerDir, "extract_validation_summary_test.go")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		// If the file doesn't exist, it might have been deleted or renamed.
		// That's acceptable — the wrapper and its tests are removed together.
		return
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, testFile, nil, 0)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", testFile, err)
	}

	// Walk the AST looking for calls to the local extractValidationSummary function.
	// After extraction, all calls should be to validation.ExtractValidationSummary
	// (which shows up as a SelectorExpr, not a plain Ident).
	localCalls := 0
	ast.Inspect(f, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// Check for direct call to extractValidationSummary (a plain Ident, not pkg.Func)
		ident, ok := callExpr.Fun.(*ast.Ident)
		if ok && ident.Name == "extractValidationSummary" {
			localCalls++
		}
		return true
	})

	// Expected failure: currently extract_validation_summary_test.go calls the
	// local extractValidationSummary() wrapper directly (5 calls at lines
	// 71, 132, 155, 168, 182). After extraction, these should be replaced with
	// validation.ExtractValidationSummary() or the test file should be deleted.
	if localCalls > 0 {
		t.Errorf("extract_validation_summary_test.go contains %d calls to local "+
			"extractValidationSummary() — after extraction, these should use "+
			"validation.ExtractValidationSummary() directly or the file should be "+
			"deleted (since validation/validation_test.go already tests the function)",
			localCalls)
	}
}

// TestFacadeRunValidationWithRecoveryDoesNotSetIsRetry verifies that after
// extraction, the facade's runValidationWithRecovery does NOT set
// bc.PromptCtx.IsRetry or bc.PromptCtx.FailureContext during recovery.
// Currently the inline recovery loop sets these (process.go lines 887-889)
// before invoking Claude. After extraction, the validation.Runner's
// RunWithRecovery handles recovery via its ExecuteFn callback, and the
// facade never touches the prompt context during recovery.
//
// Expected failure: the facade currently sets bc.PromptCtx.IsRetry = true
// and bc.PromptCtx.FailureContext during each inline recovery attempt.
// After extraction, the facade delegates to validationRunner.RunWithRecovery
// which uses its ExecuteFn callback for Claude-based fixes.
func TestFacadeRunValidationWithRecoveryDoesNotSetIsRetry(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 1,
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:        300,
			AnalysisTimeout:    60,
			StallTimeout:       30,
			StallTimeoutActive: 10,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Validation always fails so recovery is attempted
	valRunner := validation.NewRunner(cfg,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestAlways (0.01s)\nFAIL\tpkg/always", 1, nil
		},
		func(startCommit string) error { return nil },
		func(ctx context.Context, bc *runtypes.BeadContext) bool { return false },
	)

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}
	mockAnalyzer := &mockFailureAnalyzer{}

	r := &Runner{
		cfg:               cfg,
		output:            &buf,
		analyzer:          mockAnalyzer,
		renderer:          &mockRenderer{},
		validationRunner:  valRunner,
		autoFixFn:         func(startCommit string) error { return nil },
		escalationHandler: escalation.NewHandler(cfg, mockAnalyzer, nil, nil, nil, logFn),
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestAlways (0.01s)", 1, nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-isretry", Title: "Test IsRetry", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: t.TempDir()},
		StartCommit: "abc123",
		ParentCtx:   context.Background(),
	}

	_ = r.runValidationWithRecovery(context.Background(), bc)

	// Expected failure: currently the facade sets bc.PromptCtx.IsRetry = true
	// during inline recovery (process.go line 887). After extraction, the
	// facade delegates recovery to validationRunner.RunWithRecovery, which
	// handles Claude-based fixes via its ExecuteFn callback without modifying
	// the facade's prompt context.
	if bc.PromptCtx.IsRetry {
		t.Error("bc.PromptCtx.IsRetry should not be set by facade during validation recovery — " +
			"after extraction, recovery is delegated to validation.Runner.RunWithRecovery " +
			"which uses its ExecuteFn callback, not the facade's prompt context")
	}
	if bc.PromptCtx.FailureContext != "" {
		t.Errorf("bc.PromptCtx.FailureContext = %q, want empty — "+
			"after extraction, the facade should not set FailureContext during "+
			"validation recovery; the validation.Runner handles this via ExecuteFn",
			bc.PromptCtx.FailureContext)
	}
}

// TestFacadeRunValidationDoesNotCallExtractValidationSummary verifies that
// after extraction, the facade's runValidation does NOT call
// validation.ExtractValidationSummary inline. Currently the facade calls
// validation.ExtractValidationSummary(failureOutput) on line 787 and appends
// the result to r.validationFailures. After extraction, the validation.Runner's
// internal runValidation method calls ExtractValidationSummary and accumulates
// in its own failures slice. The facade then reads from validationRunner.Failures()
// to sync with r.validationFailures.
//
// Expected failure: the facade currently calls validation.ExtractValidationSummary
// inline and appends to r.validationFailures. After extraction, the facade reads
// from validationRunner.Failures() instead, so r.validationFailures should be
// populated from the validation.Runner's accumulation (not inline extraction).
func TestFacadeRunValidationDoesNotCallExtractValidationSummary(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:     300,
			AnalysisTimeout: 60,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create a validation.Runner that fails with recognizable output.
	// The Runner's internal runValidation accumulates failures via Failures().
	valRunner := validation.NewRunner(cfg,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "--- FAIL: TestInlineExtract (0.01s)\nFAIL\tpkg/extract", 1, nil
		},
		nil, nil,
	)

	var buf strings.Builder
	r := &Runner{
		cfg:              cfg,
		output:           &buf,
		analyzer:         &mockFailureAnalyzer{},
		validationRunner: valRunner,
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-extract", Title: "Test extract", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	_ = r.runValidation(context.Background(), bc)

	// After extraction, the facade should sync r.validationFailures from
	// validationRunner.Failures(). If the facade is still calling
	// validation.ExtractValidationSummary inline (current behavior), then
	// valRunner.Failures() will be EMPTY (because RunDirect doesn't accumulate).
	//
	// Expected failure: currently the facade calls RunDirect (which doesn't
	// accumulate) and then calls validation.ExtractValidationSummary inline.
	// After extraction, the facade calls the Runner's higher-level method
	// (which accumulates), and reads from Failures().
	valFailures := valRunner.Failures()
	if len(valFailures) == 0 {
		t.Error("validation.Runner.Failures() should be populated — " +
			"after extraction, the facade calls the Runner's full validation flow " +
			"(not RunDirect) which accumulates failures internally; " +
			"the facade then reads from Failures() instead of calling " +
			"validation.ExtractValidationSummary inline")
	}

	// Verify the facade's validationFailures are sourced from the Runner's Failures(),
	// not from an independent inline extraction
	if len(r.validationFailures) > 0 && len(valFailures) == 0 {
		t.Error("facade has validationFailures but validation.Runner.Failures() is empty — " +
			"the facade is still extracting summaries inline instead of reading from " +
			"the validation.Runner's accumulation")
	}
}
