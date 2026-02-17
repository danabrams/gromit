package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func verifyRunnerSplitPhase3Layout(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)

	gatesPath := filepath.Join(runnerDir, "gates.go")
	loggingPath := filepath.Join(runnerDir, "logging.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	if _, err := os.Stat(gatesPath); err != nil {
		t.Fatalf("expected gates.go to exist: %v", err)
	}
	if _, err := os.Stat(loggingPath); err != nil {
		t.Fatalf("expected logging.go to exist: %v", err)
	}

	gatesDecls := parseSplitFileDecls(t, gatesPath)
	mustHaveMethod(t, gatesDecls, "Runner", "runPrecheck", "gates.go")
	mustHaveMethod(t, gatesDecls, "Runner", "checkScope", "gates.go")
	mustHaveImport(t, gatesDecls, "context", "gates.go")
	mustHaveImport(t, gatesDecls, "strings", "gates.go")
	mustHaveImport(t, gatesDecls, "time", "gates.go")
	mustHaveImport(t, gatesDecls, "github.com/danabrams/gromit/internal/bead", "gates.go")
	mustHaveImport(t, gatesDecls, "github.com/danabrams/gromit/internal/prompt", "gates.go")
	mustHaveImport(t, gatesDecls, "github.com/danabrams/gromit/internal/provider", "gates.go")

	loggingDecls := parseSplitFileDecls(t, loggingPath)
	mustHaveMethod(t, loggingDecls, "Runner", "writeIterationLog", "logging.go")
	mustHaveMethod(t, loggingDecls, "Runner", "logResult", "logging.go")
	mustHaveMethod(t, loggingDecls, "Runner", "log", "logging.go")
	mustHaveImport(t, loggingDecls, "fmt", "logging.go")
	mustHaveImport(t, loggingDecls, "strings", "logging.go")
	mustHaveImport(t, loggingDecls, "time", "logging.go")
	mustHaveImport(t, loggingDecls, "github.com/danabrams/gromit/internal/logger", "logging.go")

	runnerDecls := parseSplitFileDecls(t, runnerPath)
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["runPrecheck"] {
		t.Fatalf("runner.go still contains method Runner.runPrecheck")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["checkScope"] {
		t.Fatalf("runner.go still contains method Runner.checkScope")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["writeIterationLog"] {
		t.Fatalf("runner.go still contains method Runner.writeIterationLog")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["logResult"] {
		t.Fatalf("runner.go still contains method Runner.logResult")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["log"] {
		t.Fatalf("runner.go still contains method Runner.log")
	}

	return runnerDir
}

// TestSplitRunnerPhase3_RunnerGoDoesNotContainMovedMethods verifies acceptance
// criterion #3 through package source auditing.
//
// Expected failure: runner.go still contains moved methods and marker
// `RunnerGoSplitAuditV3` is not in the codebase.
func TestSplitRunnerPhase3_RunnerGoDoesNotContainMovedMethods(t *testing.T) {
	verifyRunnerSplitPhase3Layout(t)
}
