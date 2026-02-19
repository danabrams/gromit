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

type splitFileDecls struct {
	types   map[string]bool
	funcs   map[string]bool
	methods map[string]map[string]bool
	imports map[string]bool
}

func parseSplitFileDecls(t *testing.T, path string) splitFileDecls {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}

	decls := splitFileDecls{
		types:   make(map[string]bool),
		funcs:   make(map[string]bool),
		methods: make(map[string]map[string]bool),
		imports: make(map[string]bool),
	}

	for _, imp := range file.Imports {
		if imp.Path != nil {
			decls.imports[imp.Path.Value] = true
		}
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if ok {
					decls.types[typeSpec.Name.Name] = true
				}
			}
		case *ast.FuncDecl:
			if d.Recv == nil {
				decls.funcs[d.Name.Name] = true
				continue
			}
			if len(d.Recv.List) == 0 {
				continue
			}
			recvName := receiverTypeName(d.Recv.List[0].Type)
			if recvName == "" {
				continue
			}
			if decls.methods[recvName] == nil {
				decls.methods[recvName] = make(map[string]bool)
			}
			decls.methods[recvName][d.Name.Name] = true
		}
	}

	return decls
}

func receiverTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(e.X)
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

func mustHaveType(t *testing.T, decls splitFileDecls, typeName, fileName string) {
	t.Helper()
	if !decls.types[typeName] {
		t.Fatalf("%s is missing required type %s", fileName, typeName)
	}
}

func mustHaveFunc(t *testing.T, decls splitFileDecls, funcName, fileName string) {
	t.Helper()
	if !decls.funcs[funcName] {
		t.Fatalf("%s is missing required function %s", fileName, funcName)
	}
}

func mustHaveMethod(t *testing.T, decls splitFileDecls, recv, methodName, fileName string) {
	t.Helper()
	if decls.methods[recv] == nil || !decls.methods[recv][methodName] {
		t.Fatalf("%s is missing required method %s.%s", fileName, recv, methodName)
	}
}

func mustHaveImport(t *testing.T, decls splitFileDecls, importPath, fileName string) {
	t.Helper()
	quoted := `"` + importPath + `"`
	if !decls.imports[quoted] {
		t.Fatalf("%s is missing required import %s", fileName, importPath)
	}
}

func verifyRunnerSplitPhase1Layout(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)

	adaptersPath := filepath.Join(runnerDir, "adapters.go")
	callbacksPath := filepath.Join(runnerDir, "callbacks.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	if _, err := os.Stat(adaptersPath); err != nil {
		t.Fatalf("expected adapters.go to exist: %v", err)
	}
	if _, err := os.Stat(callbacksPath); err != nil {
		t.Fatalf("expected callbacks.go to exist: %v", err)
	}

	adaptersDecls := parseSplitFileDecls(t, adaptersPath)
	mustHaveType(t, adaptersDecls, "routerAdapter", "adapters.go")
	mustHaveMethod(t, adaptersDecls, "routerAdapter", "Select", "adapters.go")
	mustHaveMethod(t, adaptersDecls, "routerAdapter", "MarkUnavailable", "adapters.go")
	mustHaveFunc(t, adaptersDecls, "makeStallTimeoutFn", "adapters.go")
	mustHaveType(t, adaptersDecls, "successLearningRouterAdapter", "adapters.go")
	mustHaveMethod(t, adaptersDecls, "successLearningRouterAdapter", "Select", "adapters.go")
	mustHaveType(t, adaptersDecls, "successLearningProviderAdapter", "adapters.go")
	mustHaveMethod(t, adaptersDecls, "successLearningProviderAdapter", "Run", "adapters.go")
	mustHaveType(t, adaptersDecls, "successLearningResultAdapter", "adapters.go")
	mustHaveMethod(t, adaptersDecls, "successLearningResultAdapter", "IsSuccess", "adapters.go")
	mustHaveMethod(t, adaptersDecls, "successLearningResultAdapter", "GetOutput", "adapters.go")
	mustHaveImport(t, adaptersDecls, "context", "adapters.go")
	mustHaveImport(t, adaptersDecls, "time", "adapters.go")
	mustHaveImport(t, adaptersDecls, "github.com/danabrams/gromit/internal/config", "adapters.go")
	mustHaveImport(t, adaptersDecls, "github.com/danabrams/gromit/internal/provider", "adapters.go")
	mustHaveImport(t, adaptersDecls, "github.com/danabrams/gromit/internal/runner/escalation", "adapters.go")
	mustHaveImport(t, adaptersDecls, "github.com/danabrams/gromit/internal/runner/execution", "adapters.go")

	callbacksDecls := parseSplitFileDecls(t, callbacksPath)
	mustHaveMethod(t, callbacksDecls, "Runner", "makeInvokeFn", "callbacks.go")
	mustHaveMethod(t, callbacksDecls, "Runner", "makeValidationExecuteFn", "callbacks.go")
	mustHaveMethod(t, callbacksDecls, "Runner", "makeReviewValidateFn", "callbacks.go")
	mustHaveMethod(t, callbacksDecls, "Runner", "makeMethodologyExec", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "context", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "fmt", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "os/exec", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/bead", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/claude", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/provider", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/prompt", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/runner/escalation", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/runner/methodology", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/runner/reviewpkg", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/runner/runtypes", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/runner/validation", "callbacks.go")
	mustHaveImport(t, callbacksDecls, "github.com/danabrams/gromit/internal/usagelimit", "callbacks.go")

	runnerDecls := parseSplitFileDecls(t, runnerPath)
	if runnerDecls.types["routerAdapter"] {
		t.Fatalf("runner.go still contains type routerAdapter")
	}
	if runnerDecls.types["successLearningRouterAdapter"] {
		t.Fatalf("runner.go still contains type successLearningRouterAdapter")
	}
	if runnerDecls.types["successLearningProviderAdapter"] {
		t.Fatalf("runner.go still contains type successLearningProviderAdapter")
	}
	if runnerDecls.types["successLearningResultAdapter"] {
		t.Fatalf("runner.go still contains type successLearningResultAdapter")
	}
	if runnerDecls.funcs["makeStallTimeoutFn"] {
		t.Fatalf("runner.go still contains function makeStallTimeoutFn")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["makeInvokeFn"] {
		t.Fatalf("runner.go still contains method Runner.makeInvokeFn")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["makeValidationExecuteFn"] {
		t.Fatalf("runner.go still contains method Runner.makeValidationExecuteFn")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["makeReviewValidateFn"] {
		t.Fatalf("runner.go still contains method Runner.makeReviewValidateFn")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["makeMethodologyExec"] {
		t.Fatalf("runner.go still contains method Runner.makeMethodologyExec")
	}

	return runnerDir
}

// TestSplitRunnerPhase1_RunnerGoDoesNotContainMovedDeclarations verifies
// acceptance criterion #3 through the package source surface.
//
// Expected failure: runner.go still contains moved declarations and the source
// audit marker `RunnerGoSplitAuditV1` is not present yet.
func TestSplitRunnerPhase1_RunnerGoDoesNotContainMovedDeclarations(t *testing.T) {
	verifyRunnerSplitPhase1Layout(t)
}
