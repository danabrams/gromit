package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCallbacksUseInjectedGitDependencies(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	callbacksPath := filepath.Join(filepath.Dir(thisFile), "callbacks.go")
	tddPath := filepath.Join(filepath.Dir(thisFile), "callbacks_tdd.go")

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, callbacksPath, nil, 0)
	if err != nil {
		t.Fatalf("parse callbacks.go: %v", err)
	}
	tddFile, err := parser.ParseFile(fset, tddPath, nil, 0)
	if err != nil {
		t.Fatalf("parse callbacks_tdd.go: %v", err)
	}

	makeMethodologyExec := findRunnerMethod(file, "makeMethodologyExec")
	if makeMethodologyExec == nil {
		t.Fatal("Runner.makeMethodologyExec not found")
	}
	assertNoDirectExecCommand(t, makeMethodologyExec)
	assertHasReceiverCall(t, makeMethodologyExec, "runArgv")
	assertHasReceiverSelector(t, makeMethodologyExec, "getHead")
	assertNoIdentifierUse(t, makeMethodologyExec, "getGitHead")

	makeTDDOrchestrator := findRunnerMethod(tddFile, "makeTDDOrchestrator")
	if makeTDDOrchestrator == nil {
		t.Fatal("Runner.makeTDDOrchestrator not found")
	}
	assertNoDirectExecCommand(t, makeTDDOrchestrator)
	assertHasReceiverCall(t, makeTDDOrchestrator, "runArgv")
	assertHasReceiverSelector(t, makeTDDOrchestrator, "getHead")
}

func findRunnerMethod(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != name || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
			if ident, ok := star.X.(*ast.Ident); ok && ident.Name == "Runner" {
				return fn
			}
		}
	}
	return nil
}

func assertNoDirectExecCommand(t *testing.T, fn *ast.FuncDecl) {
	t.Helper()
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if ok && pkg.Name == "exec" && strings.HasPrefix(sel.Sel.Name, "Command") {
			t.Fatalf("%s should not call exec.%s directly", fn.Name.Name, sel.Sel.Name)
		}
		return true
	})
}

func assertHasReceiverCall(t *testing.T, fn *ast.FuncDecl, methodName string) {
	t.Helper()
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if ok && recv.Name == "r" && sel.Sel.Name == methodName {
			found = true
		}
		return true
	})
	if !found {
		t.Fatalf("%s should call r.%s", fn.Name.Name, methodName)
	}
}

func assertNoIdentifierUse(t *testing.T, fn *ast.FuncDecl, identName string) {
	t.Helper()
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && ident.Name == identName {
			t.Fatalf("%s should not reference %s directly", fn.Name.Name, identName)
		}
		return true
	})
}

func assertHasReceiverSelector(t *testing.T, fn *ast.FuncDecl, methodName string) {
	t.Helper()
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if ok && recv.Name == "r" && sel.Sel.Name == methodName {
			found = true
		}
		return true
	})
	if !found {
		t.Fatalf("%s should reference r.%s", fn.Name.Name, methodName)
	}
}
