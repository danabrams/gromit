package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type phase2ValueDecls struct {
	consts map[string]bool
	vars   map[string]bool
}

func parsePhase2ValueDecls(t *testing.T, path string) phase2ValueDecls {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	decls := phase2ValueDecls{
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

func TestRunnerSplitPhase2_HeartbeatExtracted(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)

	heartbeatPath := filepath.Join(runnerDir, "heartbeat_facade.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	if _, err := os.Stat(heartbeatPath); err != nil {
		t.Fatalf("expected heartbeat_facade.go to exist: %v", err)
	}

	heartbeatDecls := parseDecls(t, heartbeatPath)
	heartbeatValues := parsePhase2ValueDecls(t, heartbeatPath)
	if !heartbeatValues.consts["heartbeatInterval"] {
		t.Fatal("heartbeat_facade.go missing const heartbeatInterval")
	}
	if !heartbeatDecls.types["heartbeatConfig"] {
		t.Fatal("heartbeat_facade.go missing type heartbeatConfig")
	}
	if !heartbeatValues.vars["defaultHeartbeatConfig"] {
		t.Fatal("heartbeat_facade.go missing var defaultHeartbeatConfig")
	}
	if heartbeatDecls.methods["Runner"] == nil || !heartbeatDecls.methods["Runner"]["startHeartbeat"] {
		t.Fatal("heartbeat_facade.go missing method Runner.startHeartbeat")
	}
	if heartbeatDecls.methods["Runner"] == nil || !heartbeatDecls.methods["Runner"]["startHeartbeatWithConfig"] {
		t.Fatal("heartbeat_facade.go missing method Runner.startHeartbeatWithConfig")
	}
	if heartbeatDecls.methods["Runner"] == nil || !heartbeatDecls.methods["Runner"]["printHeartbeat"] {
		t.Fatal("heartbeat_facade.go missing method Runner.printHeartbeat")
	}
	if heartbeatDecls.methods["Runner"] == nil || !heartbeatDecls.methods["Runner"]["overwriteHeartbeat"] {
		t.Fatal("heartbeat_facade.go missing method Runner.overwriteHeartbeat")
	}

	runnerDecls := parseDecls(t, runnerPath)
	runnerValues := parsePhase2ValueDecls(t, runnerPath)
	if runnerValues.consts["heartbeatInterval"] {
		t.Fatal("runner.go still contains const heartbeatInterval")
	}
	if runnerDecls.types["heartbeatConfig"] {
		t.Fatal("runner.go still contains type heartbeatConfig")
	}
	if runnerValues.vars["defaultHeartbeatConfig"] {
		t.Fatal("runner.go still contains var defaultHeartbeatConfig")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["startHeartbeat"] {
		t.Fatal("runner.go still contains method Runner.startHeartbeat")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["startHeartbeatWithConfig"] {
		t.Fatal("runner.go still contains method Runner.startHeartbeatWithConfig")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["printHeartbeat"] {
		t.Fatal("runner.go still contains method Runner.printHeartbeat")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["overwriteHeartbeat"] {
		t.Fatal("runner.go still contains method Runner.overwriteHeartbeat")
	}
}

func TestRunnerSplitPhase2_DecomposeExtracted(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)

	decomposePath := filepath.Join(runnerDir, "decompose.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	if _, err := os.Stat(decomposePath); err != nil {
		t.Fatalf("expected decompose.go to exist: %v", err)
	}

	decomposeDecls := parseDecls(t, decomposePath)
	if decomposeDecls.methods["Runner"] == nil || !decomposeDecls.methods["Runner"]["DecomposeTask"] {
		t.Fatal("decompose.go missing method Runner.DecomposeTask")
	}
	if !decomposeDecls.funcs["parseDecomposeOutput"] {
		t.Fatal("decompose.go missing function parseDecomposeOutput")
	}
	if decomposeDecls.methods["Runner"] == nil || !decomposeDecls.methods["Runner"]["CreateSubBeads"] {
		t.Fatal("decompose.go missing method Runner.CreateSubBeads")
	}
	if decomposeDecls.methods["Runner"] == nil || !decomposeDecls.methods["Runner"]["injectMethodologyLabels"] {
		t.Fatal("decompose.go missing method Runner.injectMethodologyLabels")
	}

	runnerDecls := parseDecls(t, runnerPath)
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["DecomposeTask"] {
		t.Fatal("runner.go still contains method Runner.DecomposeTask")
	}
	if runnerDecls.funcs["parseDecomposeOutput"] {
		t.Fatal("runner.go still contains function parseDecomposeOutput")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["CreateSubBeads"] {
		t.Fatal("runner.go still contains method Runner.CreateSubBeads")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["injectMethodologyLabels"] {
		t.Fatal("runner.go still contains method Runner.injectMethodologyLabels")
	}
}
