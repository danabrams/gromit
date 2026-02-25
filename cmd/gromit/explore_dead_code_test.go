package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestExplorePhaseConfigFlagRemoved(t *testing.T) {
	path := productionFilePath(t, filepath.Join("cmd", "gromit", "explore.go"))
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range valueSpec.Names {
					if name.Name == "exploreAgentOverrideFlag" || name.Name == "explorePhaseConfigFlag" {
						t.Fatalf("found %s declared in %s", name.Name, path)
					}
				}
			}
		}
	}
}
