package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

type splitDecls struct {
	types   map[string]bool
	funcs   map[string]bool
	methods map[string]map[string]bool
}

func parseDecls(t *testing.T, path string) splitDecls {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	decls := splitDecls{
		types:   make(map[string]bool),
		funcs:   make(map[string]bool),
		methods: make(map[string]map[string]bool),
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
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
			recv := receiverName(d.Recv.List[0].Type)
			if recv == "" {
				continue
			}
			if decls.methods[recv] == nil {
				decls.methods[recv] = make(map[string]bool)
			}
			decls.methods[recv][d.Name.Name] = true
		}
	}

	return decls
}

func receiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

func TestRunnerSplitPhase1_AdaptersExtracted(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)
	adaptersPath := filepath.Join(runnerDir, "adapters.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	adaptersDecls := parseDecls(t, adaptersPath)
	if !adaptersDecls.types["routerAdapter"] {
		t.Fatal("adapters.go missing type routerAdapter")
	}
	if !adaptersDecls.funcs["makeStallTimeoutFn"] {
		t.Fatal("adapters.go missing function makeStallTimeoutFn")
	}
	if !adaptersDecls.types["successLearningRouterAdapter"] {
		t.Fatal("adapters.go missing type successLearningRouterAdapter")
	}
	if !adaptersDecls.types["successLearningProviderAdapter"] {
		t.Fatal("adapters.go missing type successLearningProviderAdapter")
	}
	if !adaptersDecls.types["successLearningResultAdapter"] {
		t.Fatal("adapters.go missing type successLearningResultAdapter")
	}

	runnerDecls := parseDecls(t, runnerPath)
	if runnerDecls.types["routerAdapter"] {
		t.Fatal("runner.go still contains type routerAdapter")
	}
	if runnerDecls.funcs["makeStallTimeoutFn"] {
		t.Fatal("runner.go still contains function makeStallTimeoutFn")
	}
	if runnerDecls.types["successLearningRouterAdapter"] {
		t.Fatal("runner.go still contains type successLearningRouterAdapter")
	}
	if runnerDecls.types["successLearningProviderAdapter"] {
		t.Fatal("runner.go still contains type successLearningProviderAdapter")
	}
	if runnerDecls.types["successLearningResultAdapter"] {
		t.Fatal("runner.go still contains type successLearningResultAdapter")
	}
}

func TestRunnerSplitPhase1_CallbacksExtracted(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)
	callbacksPath := filepath.Join(runnerDir, "callbacks.go")
	callbacksValidationPath := filepath.Join(runnerDir, "callbacks_validation.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	callbacksDecls := parseDecls(t, callbacksPath)
	if callbacksDecls.methods["Runner"] == nil || !callbacksDecls.methods["Runner"]["makeInvokeFn"] {
		t.Fatal("callbacks.go missing method Runner.makeInvokeFn")
	}
	if callbacksDecls.methods["Runner"] == nil || !callbacksDecls.methods["Runner"]["makeMethodologyExec"] {
		t.Fatal("callbacks.go missing method Runner.makeMethodologyExec")
	}
	callbacksValidationDecls := parseDecls(t, callbacksValidationPath)
	if callbacksValidationDecls.methods["Runner"] == nil || !callbacksValidationDecls.methods["Runner"]["makeValidationExecuteFn"] {
		t.Fatal("callbacks_validation.go missing method Runner.makeValidationExecuteFn")
	}
	if callbacksValidationDecls.methods["Runner"] == nil || !callbacksValidationDecls.methods["Runner"]["makeReviewValidateFn"] {
		t.Fatal("callbacks_validation.go missing method Runner.makeReviewValidateFn")
	}

	runnerDecls := parseDecls(t, runnerPath)
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["makeInvokeFn"] {
		t.Fatal("runner.go still contains method Runner.makeInvokeFn")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["makeValidationExecuteFn"] {
		t.Fatal("runner.go still contains method Runner.makeValidationExecuteFn")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["makeReviewValidateFn"] {
		t.Fatal("runner.go still contains method Runner.makeReviewValidateFn")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["makeMethodologyExec"] {
		t.Fatal("runner.go still contains method Runner.makeMethodologyExec")
	}
}
