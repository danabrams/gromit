package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRefineHelpersContainsSpecAbsent(t *testing.T) {
	path := productionFilePath(t, filepath.Join("cmd", "gromit", "refine_helpers.go"))
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "containsSpec" {
			t.Fatalf("found containsSpec declared in %s", path)
		}
	}
}

func productionFilePath(t *testing.T, rel string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine caller file path")
	}

	root := filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
	return filepath.Join(root, rel)
}
