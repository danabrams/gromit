package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	phase = flag.String("phase", "pure", "Phase of rollout: pure or tempdir")
	dirs  = flag.String("dirs", "", "Comma-separated list of directories to process")
)

func main() {
	flag.Parse()

	if *dirs == "" {
		fmt.Fprintln(os.Stderr, "--dirs is required")
		os.Exit(1)
	}

	dirList := strings.Split(*dirs, ",")
	includeTempDir := *phase == "tempdir"

	fset := token.NewFileSet()
	for _, dir := range dirList {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			changed, err := processFile(fset, path, includeTempDir)
			if err != nil {
				return err
			}
			if changed {
				fmt.Fprintf(os.Stderr, "updated %s\n", path)
			}
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", dir, err)
			os.Exit(1)
		}
	}
}

func processFile(fset *token.FileSet, filename string, includeTempDir bool) (bool, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}

	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return false, err
	}

	modified := false

	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		if decl.Name == nil || !strings.HasPrefix(decl.Name.Name, "Test") || decl.Body == nil {
			return true
		}
		if decl.Name.Name == "TestMain" {
			return true
		}

		paramName, ok := testingParamName(decl.Type)
		if !ok {
			return true
		}

		if containsParallelBlocker(decl.Body) {
			if newList, removed := removeParallelCall(paramName, decl.Body.List); removed {
				decl.Body.List = newList
				modified = true
			}
			return true
		}

		usesTempDir := containsTempDir(decl.Body)
		if includeTempDir != usesTempDir {
			return true
		}

		if !hasParallel(paramName, decl.Body.List) {
			decl.Body.List = prependParallel(paramName, decl.Body.List)
			modified = true
		}

		ast.Inspect(decl.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "Run" {
				return true
			}
			if len(call.Args) < 2 {
				return true
			}
			funcLit, ok := call.Args[1].(*ast.FuncLit)
			if !ok || funcLit.Body == nil {
				return true
			}

			usesTemp := containsTempDir(funcLit.Body)
			if includeTempDir != usesTemp {
				return true
			}

			innerParam, ok := testingParamName(funcLit.Type)
			if !ok {
				return true
			}

			if containsParallelBlocker(funcLit.Body) {
				if newList, removed := removeParallelCall(innerParam, funcLit.Body.List); removed {
					funcLit.Body.List = newList
					modified = true
				}
				return true
			}

			if !hasParallel(innerParam, funcLit.Body.List) {
				funcLit.Body.List = prependParallel(innerParam, funcLit.Body.List)
				modified = true
			}
			return true
		})

		return true
	})

	if !modified {
		return false, nil
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return false, err
	}

	if err := os.WriteFile(filename, buf.Bytes(), 0o644); err != nil {
		return false, err
	}

	return true, nil
}

func containsTempDir(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel != nil && sel.Sel.Name == "TempDir" {
			found = true
			return false
		}
		return true
	})
	return found
}

func firstParamName(t *ast.FuncType) string {
	if t == nil || t.Params == nil || len(t.Params.List) == 0 {
		return ""
	}
	for _, field := range t.Params.List {
		for _, name := range field.Names {
			return name.Name
		}
	}
	return ""
}

func testingParamName(t *ast.FuncType) (string, bool) {
	if t == nil || t.Params == nil || len(t.Params.List) == 0 {
		return "", false
	}
	for _, field := range t.Params.List {
		if len(field.Names) == 0 {
			continue
		}
		if ft, ok := field.Type.(*ast.StarExpr); ok {
			if se, ok := ft.X.(*ast.SelectorExpr); ok {
				if se.Sel != nil && se.Sel.Name == "T" {
					if x, ok := se.X.(*ast.Ident); ok && x.Name == "testing" {
						return field.Names[0].Name, true
					}
				}
			}
		}
	}
	return "", false
}

func containsParallelBlocker(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	blockerMethods := map[string]struct{}{
		"Setenv":  {},
		"Chdir":   {},
		"Cleanup": {},
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			return true
		}
		if _, ok := blockerMethods[sel.Sel.Name]; !ok {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "t" {
			found = true
			return false
		}
		return true
	})
	return found
}

func hasParallel(param string, stmts []ast.Stmt) bool {
	if len(stmts) == 0 {
		return false
	}
	expr, ok := stmts[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != "Parallel" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == param
}

func prependParallel(param string, stmts []ast.Stmt) []ast.Stmt {
	call := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(param),
				Sel: ast.NewIdent("Parallel"),
			},
		},
	}
	return append([]ast.Stmt{call}, stmts...)
}

func removeParallelCall(param string, stmts []ast.Stmt) ([]ast.Stmt, bool) {
	if len(stmts) == 0 {
		return stmts, false
	}
	expr, ok := stmts[0].(*ast.ExprStmt)
	if !ok {
		return stmts, false
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return stmts, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != "Parallel" {
		return stmts, false
	}
	if ident, ok := sel.X.(*ast.Ident); !ok || ident.Name != param {
		return stmts, false
	}
	copy(stmts[0:], stmts[1:])
	stmts[len(stmts)-1] = nil
	return stmts[:len(stmts)-1], true
}
