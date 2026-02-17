package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func phase4MustHaveFunc(t *testing.T, decls splitDecls, funcName, fileName string) {
	t.Helper()
	if !decls.funcs[funcName] {
		t.Fatalf("%s is missing required function %s", fileName, funcName)
	}
}

func phase4MustHaveMethod(t *testing.T, decls splitDecls, recv, methodName, fileName string) {
	t.Helper()
	if decls.methods[recv] == nil || !decls.methods[recv][methodName] {
		t.Fatalf("%s is missing required method %s.%s", fileName, recv, methodName)
	}
}

func phase4MustHaveImport(t *testing.T, imports map[string]bool, importPath, fileName string) {
	t.Helper()
	if !imports[importPath] {
		t.Fatalf("%s is missing required import %s", fileName, importPath)
	}
}

func verifyRunnerSplitPhase4Layout(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)

	helpersPath := filepath.Join(runnerDir, "helpers.go")
	lifecyclePath := filepath.Join(runnerDir, "lifecycle.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	if _, err := os.Stat(helpersPath); err != nil {
		t.Fatalf("expected helpers.go to exist: %v", err)
	}
	if _, err := os.Stat(lifecyclePath); err != nil {
		t.Fatalf("expected lifecycle.go to exist: %v", err)
	}

	helpersDecls := parseDecls(t, helpersPath)
	helpersImports := parseImports(t, helpersPath)
	phase4MustHaveFunc(t, helpersDecls, "getGitHead", "helpers.go")
	phase4MustHaveFunc(t, helpersDecls, "getGitDiffStat", "helpers.go")
	phase4MustHaveFunc(t, helpersDecls, "getGitDiff", "helpers.go")
	phase4MustHaveMethod(t, helpersDecls, "Runner", "getDiff", "helpers.go")
	phase4MustHaveMethod(t, helpersDecls, "Runner", "hasNewPackages", "helpers.go")
	phase4MustHaveMethod(t, helpersDecls, "Runner", "updateTouchedPackages", "helpers.go")
	phase4MustHaveFunc(t, helpersDecls, "defaultCmdRunner", "helpers.go")
	phase4MustHaveMethod(t, helpersDecls, "Runner", "runCmd", "helpers.go")
	phase4MustHaveFunc(t, helpersDecls, "checkExpectedOutputs", "helpers.go")
	phase4MustHaveMethod(t, helpersDecls, "Runner", "showPartialProgress", "helpers.go")
	phase4MustHaveImport(t, helpersImports, "bytes", "helpers.go")
	phase4MustHaveImport(t, helpersImports, "context", "helpers.go")
	phase4MustHaveImport(t, helpersImports, "fmt", "helpers.go")
	phase4MustHaveImport(t, helpersImports, "os", "helpers.go")
	phase4MustHaveImport(t, helpersImports, "os/exec", "helpers.go")
	phase4MustHaveImport(t, helpersImports, "strings", "helpers.go")
	phase4MustHaveImport(t, helpersImports, "github.com/danabrams/gromit/internal/bead", "helpers.go")

	lifecycleDecls := parseDecls(t, lifecyclePath)
	lifecycleImports := parseImports(t, lifecyclePath)
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "checkRetroSuggestion", "lifecycle.go")
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "isStuckBeadWithStats", "lifecycle.go")
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "Status", "lifecycle.go")
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "runSessionCompletion", "lifecycle.go")
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "mergeInteractiveBranches", "lifecycle.go")
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "handleMergeFailure", "lifecycle.go")
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "runBetweenIterationsCommand", "lifecycle.go")
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "SetLabelFilters", "lifecycle.go")
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "getNextBead", "lifecycle.go")
	phase4MustHaveMethod(t, lifecycleDecls, "Runner", "updateGlobalStats", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "context", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "fmt", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "os", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "os/exec", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "path/filepath", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "strings", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "time", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "github.com/danabrams/gromit/internal/bead", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "github.com/danabrams/gromit/internal/learnings", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "github.com/danabrams/gromit/internal/logger", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "github.com/danabrams/gromit/internal/pipeline", "lifecycle.go")
	phase4MustHaveImport(t, lifecycleImports, "github.com/danabrams/gromit/internal/state", "lifecycle.go")

	runnerDecls := parseDecls(t, runnerPath)
	if runnerDecls.funcs["getGitHead"] {
		t.Fatalf("runner.go still contains function getGitHead")
	}
	if runnerDecls.funcs["getGitDiffStat"] {
		t.Fatalf("runner.go still contains function getGitDiffStat")
	}
	if runnerDecls.funcs["getGitDiff"] {
		t.Fatalf("runner.go still contains function getGitDiff")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["getDiff"] {
		t.Fatalf("runner.go still contains method Runner.getDiff")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["hasNewPackages"] {
		t.Fatalf("runner.go still contains method Runner.hasNewPackages")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["updateTouchedPackages"] {
		t.Fatalf("runner.go still contains method Runner.updateTouchedPackages")
	}
	if runnerDecls.funcs["defaultCmdRunner"] {
		t.Fatalf("runner.go still contains function defaultCmdRunner")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["runCmd"] {
		t.Fatalf("runner.go still contains method Runner.runCmd")
	}
	if runnerDecls.funcs["checkExpectedOutputs"] {
		t.Fatalf("runner.go still contains function checkExpectedOutputs")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["showPartialProgress"] {
		t.Fatalf("runner.go still contains method Runner.showPartialProgress")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["checkRetroSuggestion"] {
		t.Fatalf("runner.go still contains method Runner.checkRetroSuggestion")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["isStuckBeadWithStats"] {
		t.Fatalf("runner.go still contains method Runner.isStuckBeadWithStats")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["Status"] {
		t.Fatalf("runner.go still contains method Runner.Status")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["runSessionCompletion"] {
		t.Fatalf("runner.go still contains method Runner.runSessionCompletion")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["mergeInteractiveBranches"] {
		t.Fatalf("runner.go still contains method Runner.mergeInteractiveBranches")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["handleMergeFailure"] {
		t.Fatalf("runner.go still contains method Runner.handleMergeFailure")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["runBetweenIterationsCommand"] {
		t.Fatalf("runner.go still contains method Runner.runBetweenIterationsCommand")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["SetLabelFilters"] {
		t.Fatalf("runner.go still contains method Runner.SetLabelFilters")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["getNextBead"] {
		t.Fatalf("runner.go still contains method Runner.getNextBead")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["updateGlobalStats"] {
		t.Fatalf("runner.go still contains method Runner.updateGlobalStats")
	}

	return runnerDir
}

// TestSplitRunnerPhase4_RunnerUnder1000Lines verifies acceptance criterion #2:
// runner.go is under 1,000 lines, verified with wc -l.
//
// Expected failure: moved declarations still live in runner.go and marker
// `RunnerSplitPhase4LineBudget` is not in the codebase.
func TestSplitRunnerPhase4_RunnerUnder1000Lines(t *testing.T) {
	runnerDir := verifyRunnerSplitPhase4Layout(t)
	runnerPath := filepath.Join(runnerDir, "runner.go")

	cmd := exec.Command("wc", "-l", runnerPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wc -l %s failed: %v\n%s", runnerPath, err, string(out))
	}

	fields := strings.Fields(string(out))
	if len(fields) < 1 {
		t.Fatalf("unexpected wc output: %q", string(out))
	}
	lineCount, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("failed to parse wc output %q: %v", string(out), err)
	}
	if lineCount >= 1000 {
		t.Fatalf("runner.go line count = %d, expected < 1000", lineCount)
	}
}
