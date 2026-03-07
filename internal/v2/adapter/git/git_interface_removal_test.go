package git

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestGitInterfaceRemoved(t *testing.T) {
	t.Helper()
	path := filepath.Join("git.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse git.go: %v", err)
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "Git" {
				continue
			}
			if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				t.Fatalf("found Git interface declared in %s", path)
			}
		}
	}
}
