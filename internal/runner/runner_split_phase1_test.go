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
	types map[string]bool
	funcs map[string]bool
}

func parseDecls(t *testing.T, path string) splitDecls {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	decls := splitDecls{
		types: make(map[string]bool),
		funcs: make(map[string]bool),
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
			}
		}
	}

	return decls
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
