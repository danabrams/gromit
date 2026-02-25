package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
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
