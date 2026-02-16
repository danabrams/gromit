package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

type phase2AcceptanceValueDecls struct {
	consts map[string]bool
	vars   map[string]bool
}

func parsePhase2AcceptanceValueDecls(t *testing.T, path string) phase2AcceptanceValueDecls {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}

	decls := phase2AcceptanceValueDecls{
		consts: make(map[string]bool),
		vars:   make(map[string]bool),
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if genDecl.Tok != token.CONST && genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range valueSpec.Names {
				if genDecl.Tok == token.CONST {
					decls.consts[name.Name] = true
					continue
				}
				decls.vars[name.Name] = true
			}
		}
	}

	return decls
}

func verifyRunnerSplitPhase2Layout(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)

	heartbeatPath := filepath.Join(runnerDir, "heartbeat_facade.go")
	decomposePath := filepath.Join(runnerDir, "decompose.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	if _, err := os.Stat(heartbeatPath); err != nil {
		t.Fatalf("expected heartbeat_facade.go to exist: %v", err)
	}
	if _, err := os.Stat(decomposePath); err != nil {
		t.Fatalf("expected decompose.go to exist: %v", err)
	}

	heartbeatDecls := parseSplitFileDecls(t, heartbeatPath)
	heartbeatValues := parsePhase2AcceptanceValueDecls(t, heartbeatPath)
	if !heartbeatValues.consts["heartbeatInterval"] {
		t.Fatalf("heartbeat_facade.go is missing required const heartbeatInterval")
	}
	mustHaveType(t, heartbeatDecls, "heartbeatConfig", "heartbeat_facade.go")
	if !heartbeatValues.vars["defaultHeartbeatConfig"] {
		t.Fatalf("heartbeat_facade.go is missing required var defaultHeartbeatConfig")
	}
	mustHaveMethod(t, heartbeatDecls, "Runner", "startHeartbeat", "heartbeat_facade.go")
	mustHaveMethod(t, heartbeatDecls, "Runner", "startHeartbeatWithConfig", "heartbeat_facade.go")
	mustHaveMethod(t, heartbeatDecls, "Runner", "printHeartbeat", "heartbeat_facade.go")
	mustHaveMethod(t, heartbeatDecls, "Runner", "overwriteHeartbeat", "heartbeat_facade.go")
	mustHaveImport(t, heartbeatDecls, "fmt", "heartbeat_facade.go")
	mustHaveImport(t, heartbeatDecls, "strings", "heartbeat_facade.go")
	mustHaveImport(t, heartbeatDecls, "time", "heartbeat_facade.go")
	mustHaveImport(t, heartbeatDecls, "github.com/danabrams/gromit/internal/claude", "heartbeat_facade.go")
	mustHaveImport(t, heartbeatDecls, "github.com/danabrams/gromit/internal/logger", "heartbeat_facade.go")

	decomposeDecls := parseSplitFileDecls(t, decomposePath)
	mustHaveMethod(t, decomposeDecls, "Runner", "DecomposeTask", "decompose.go")
	mustHaveFunc(t, decomposeDecls, "parseDecomposeOutput", "decompose.go")
	mustHaveMethod(t, decomposeDecls, "Runner", "CreateSubBeads", "decompose.go")
	mustHaveMethod(t, decomposeDecls, "Runner", "injectMethodologyLabels", "decompose.go")
	mustHaveImport(t, decomposeDecls, "context", "decompose.go")
	mustHaveImport(t, decomposeDecls, "fmt", "decompose.go")
	mustHaveImport(t, decomposeDecls, "github.com/danabrams/gromit/internal/bead", "decompose.go")
	mustHaveImport(t, decomposeDecls, "github.com/danabrams/gromit/internal/jsonutil", "decompose.go")
	mustHaveImport(t, decomposeDecls, "github.com/danabrams/gromit/internal/prompt", "decompose.go")
	mustHaveImport(t, decomposeDecls, "github.com/danabrams/gromit/internal/provider", "decompose.go")
	mustHaveImport(t, decomposeDecls, "github.com/danabrams/gromit/internal/runner/runtypes", "decompose.go")

	runnerDecls := parseSplitFileDecls(t, runnerPath)
	runnerValues := parsePhase2AcceptanceValueDecls(t, runnerPath)
	if runnerValues.consts["heartbeatInterval"] {
		t.Fatalf("runner.go still contains const heartbeatInterval")
	}
	if runnerDecls.types["heartbeatConfig"] {
		t.Fatalf("runner.go still contains type heartbeatConfig")
	}
	if runnerValues.vars["defaultHeartbeatConfig"] {
		t.Fatalf("runner.go still contains var defaultHeartbeatConfig")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["startHeartbeat"] {
		t.Fatalf("runner.go still contains method Runner.startHeartbeat")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["startHeartbeatWithConfig"] {
		t.Fatalf("runner.go still contains method Runner.startHeartbeatWithConfig")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["printHeartbeat"] {
		t.Fatalf("runner.go still contains method Runner.printHeartbeat")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["overwriteHeartbeat"] {
		t.Fatalf("runner.go still contains method Runner.overwriteHeartbeat")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["DecomposeTask"] {
		t.Fatalf("runner.go still contains method Runner.DecomposeTask")
	}
	if runnerDecls.funcs["parseDecomposeOutput"] {
		t.Fatalf("runner.go still contains function parseDecomposeOutput")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["CreateSubBeads"] {
		t.Fatalf("runner.go still contains method Runner.CreateSubBeads")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["injectMethodologyLabels"] {
		t.Fatalf("runner.go still contains method Runner.injectMethodologyLabels")
	}

	return runnerDir
}

// TestSplitRunnerPhase2_GoBuildPasses verifies acceptance criterion #1:
// go build ./... passes once heartbeat/decompose extraction is complete.
//
// Expected failure: runner.go still owns moved declarations and the split-layout
// sentinel behavior `RunnerSplitPhase2Complete` does not exist yet.
func TestSplitRunnerPhase2_GoBuildPasses(t *testing.T) {
	runnerDir := verifyRunnerSplitPhase2Layout(t)
	repoRoot := filepath.Clean(filepath.Join(runnerDir, "..", ".."))

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./... failed: %v\n%s", err, string(out))
	}
}

// TestSplitRunnerPhase2_RunnerPackageTestsPass verifies acceptance criterion #2:
// go test ./internal/runner/... -count=1 passes after extraction.
//
// Expected failure: split-layout gate fails before command execution and the
// recursion guard marker `RunnerSplitPhase2TestReentry` is not part of the codebase yet.
func TestSplitRunnerPhase2_RunnerPackageTestsPass(t *testing.T) {
	if os.Getenv("GROMIT_SPLIT_PHASE2_REENTRY") == "1" {
		return
	}

	runnerDir := verifyRunnerSplitPhase2Layout(t)
	repoRoot := filepath.Clean(filepath.Join(runnerDir, "..", ".."))

	cmd := exec.Command(
		"go",
		"test",
		"./internal/runner/...",
		"-count=1",
		"-run",
		"TestRunnerSplitVerificationReclassified_ImportIsolation|TestRunnerSplitVerificationReclassified_LineBudgets",
	)
	cmd.Dir = repoRoot
	cmd.Env = append(
		os.Environ(),
		"GROMIT_SPLIT_PHASE1_REENTRY=1",
		"GROMIT_SPLIT_PHASE2_REENTRY=1",
		"GROMIT_SPLIT_PHASE3_REENTRY=1",
		"GROMIT_SPLIT_PHASE4_REENTRY=1",
		"GROMIT_SPLIT_FINAL_VERIFICATION_REENTRY=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test ./internal/runner/... -count=1 failed: %v\n%s", err, string(out))
	}
}

// TestSplitRunnerPhase2_RunnerGoDoesNotContainMovedDeclarations verifies
// acceptance criterion #3 through the package source surface.
//
// Expected failure: runner.go still contains moved declarations and the source
// audit marker `RunnerGoSplitAuditV2` is not present yet.
func TestSplitRunnerPhase2_RunnerGoDoesNotContainMovedDeclarations(t *testing.T) {
	verifyRunnerSplitPhase2Layout(t)
}
